package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"pal-nas-media-uploader/app/processing"

	"github.com/jackc/pgx/v5"
)

func (store *Postgres) ClaimProcessingJob(ctx context.Context) (processing.Job, bool, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return processing.Job{}, false, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var job processing.Job
	err = transaction.QueryRow(ctx, `SELECT job.id, job.upload_id, media.owner_id, media.media_path,
		job.status, job.attempts, COALESCE(job.last_error, ''), job.scheduled_for, job.updated_at
		FROM media_processing_jobs job JOIN media_uploads media ON media.upload_id = job.upload_id
		WHERE ((job.status = 'queued' AND job.scheduled_for <= now()) OR
			(job.status = 'processing' AND job.updated_at <= now() - interval '15 minutes'))
			AND media.deleted_at IS NULL
		ORDER BY job.scheduled_for, job.created_at FOR UPDATE OF job SKIP LOCKED LIMIT 1`).Scan(
		&job.ID, &job.UploadID, &job.OwnerID, &job.MediaPath, &job.Status, &job.Attempts,
		&job.LastError, &job.Scheduled, &job.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return processing.Job{}, false, nil
	}
	if err != nil {
		return processing.Job{}, false, err
	}
	job.Attempts++
	job.Status = "processing"
	if _, err := transaction.Exec(ctx, `UPDATE media_processing_jobs SET status = 'processing', attempts = $2,
		updated_at = now() WHERE id = $1`, job.ID, job.Attempts); err != nil {
		return processing.Job{}, false, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return processing.Job{}, false, err
	}
	return job, true, nil
}

func (store *Postgres) CompleteProcessingJob(ctx context.Context, job processing.Job, metadata processing.Metadata) error {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, `UPDATE media_uploads SET thumbnail_path = $2, width = NULLIF($3, 0),
		height = NULLIF($4, 0), video_duration_seconds = NULLIF($5, 0) WHERE upload_id = $1`,
		job.UploadID, metadata.ThumbnailPath, metadata.Width, metadata.Height, metadata.Duration); err != nil {
		return err
	}
	rawEXIF := metadata.RawEXIF
	if len(rawEXIF) == 0 {
		rawEXIF = json.RawMessage(`{}`)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO media_exif (upload_id, captured_at, latitude, longitude, raw_exif)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT (upload_id) DO UPDATE SET captured_at = EXCLUDED.captured_at,
		latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude, raw_exif = EXCLUDED.raw_exif`,
		job.UploadID, metadata.CapturedAt, metadata.Latitude, metadata.Longitude, rawEXIF); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `UPDATE media_processing_jobs SET status = 'completed', last_error = NULL,
		updated_at = now() WHERE id = $1`, job.ID); err != nil {
		return err
	}
	return transaction.Commit(ctx)
}

func (store *Postgres) FailProcessingJob(ctx context.Context, job processing.Job, message string) error {
	status := "queued"
	scheduled := time.Now().Add(time.Duration(1<<min(job.Attempts, 6)) * 30 * time.Second)
	if job.Attempts >= 3 {
		status = "failed"
	}
	_, err := store.pool.Exec(ctx, `UPDATE media_processing_jobs SET status = $2, last_error = $3,
		scheduled_for = $4, updated_at = now() WHERE id = $1`, job.ID, status, message, scheduled)
	return err
}

func (store *Postgres) GetProcessingStatus(ctx context.Context, uploadID, userID string) (processing.Job, error) {
	var job processing.Job
	err := store.pool.QueryRow(ctx, `SELECT job.id, job.upload_id, job.status, job.attempts,
		COALESCE(job.last_error, ''), job.scheduled_for, job.updated_at
		FROM media_processing_jobs job JOIN media_uploads media ON media.upload_id = job.upload_id
		WHERE job.upload_id = $1 AND (media.owner_id = $2 OR EXISTS (SELECT 1 FROM user_media_shares share
		WHERE share.owner_id = media.owner_id AND share.user_id = $2))`, uploadID, userID).Scan(
		&job.ID, &job.UploadID, &job.Status, &job.Attempts, &job.LastError, &job.Scheduled, &job.UpdatedAt)
	return job, err
}

func (store *Postgres) ListProcessingJobs(ctx context.Context) ([]processing.Job, error) {
	rows, err := store.pool.Query(ctx, `SELECT id, upload_id, status, attempts, COALESCE(last_error, ''),
		scheduled_for, updated_at FROM media_processing_jobs ORDER BY updated_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []processing.Job
	for rows.Next() {
		var job processing.Job
		if err := rows.Scan(&job.ID, &job.UploadID, &job.Status, &job.Attempts, &job.LastError, &job.Scheduled, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (store *Postgres) RetryProcessingJob(ctx context.Context, jobID string) error {
	result, err := store.pool.Exec(ctx, `UPDATE media_processing_jobs SET status = 'queued', attempts = 0,
		last_error = NULL, scheduled_for = now(), updated_at = now() WHERE id = $1 AND status = 'failed'`, jobID)
	if err == nil && result.RowsAffected() == 0 {
		return errors.New("failed processing job not found")
	}
	return err
}
