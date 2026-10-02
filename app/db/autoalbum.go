package store

import (
	"context"
	"errors"

	"pal-next-gallery-server/app/autoalbum"

	"github.com/jackc/pgx/v5"
)

func (store *Postgres) ListUnassignedAutomaticAlbumMedia(ctx context.Context) ([]autoalbum.Media, error) {
	rows, err := store.pool.Query(ctx, `WITH accessible_media AS (
		SELECT m.upload_id, m.owner_id AS user_id, m.created_at
		FROM media_uploads m WHERE m.deleted_at IS NULL
		UNION ALL
		SELECT m.upload_id, share.user_id, m.created_at
		FROM media_uploads m
		JOIN user_media_shares share ON share.owner_id = m.owner_id
		WHERE m.deleted_at IS NULL
	)
		SELECT accessible.upload_id, accessible.user_id, accessible.created_at
		FROM accessible_media accessible
		WHERE NOT EXISTS (
			SELECT 1 FROM album_media am JOIN albums a ON a.id = am.album_id
			WHERE am.upload_id = accessible.upload_id AND a.automatic = TRUE
			AND a.owner_id = accessible.user_id)
		ORDER BY accessible.user_id, accessible.created_at, accessible.upload_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	media := make([]autoalbum.Media, 0)
	for rows.Next() {
		var item autoalbum.Media
		if err := rows.Scan(&item.ID, &item.AlbumOwnerID, &item.CreatedAt); err != nil {
			return nil, err
		}
		media = append(media, item)
	}
	return media, rows.Err()
}

func (store *Postgres) AssignAutomaticAlbum(ctx context.Context, assignment autoalbum.Assignment) error {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	lockKey := assignment.OwnerID + ":" + assignment.DailyKey
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return err
	}

	var dailyAlbumID string
	err = transaction.QueryRow(ctx, `SELECT id FROM albums
		WHERE owner_id = $1 AND auto_key = $2`, assignment.OwnerID, assignment.DailyKey).Scan(&dailyAlbumID)
	if err == nil {
		if _, err := transaction.Exec(ctx, `INSERT INTO album_media (album_id, upload_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, dailyAlbumID, assignment.MediaID); err != nil {
			return err
		}
		return transaction.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var weeklyAlbumID string
	if err := transaction.QueryRow(ctx, `INSERT INTO albums
		(id, owner_id, title, description, automatic, auto_key)
		VALUES ($1, $2, $3, '', TRUE, $4)
		ON CONFLICT (owner_id, auto_key) WHERE auto_key IS NOT NULL
		DO UPDATE SET title = EXCLUDED.title
		RETURNING id`, assignment.WeeklyAlbumID, assignment.OwnerID,
		assignment.WeeklyTitle, assignment.WeeklyKey).Scan(&weeklyAlbumID); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO album_media (album_id, upload_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, weeklyAlbumID, assignment.MediaID); err != nil {
		return err
	}

	var dailyCount int
	if err := transaction.QueryRow(ctx, `SELECT COUNT(*) FROM media_uploads m
		WHERE m.deleted_at IS NULL AND (m.owner_id = $1 OR EXISTS (
			SELECT 1 FROM user_media_shares share
			WHERE share.owner_id = m.owner_id AND share.user_id = $1))
		AND m.created_at >= $2 AND m.created_at < $2 + INTERVAL '1 day'`,
		assignment.OwnerID, assignment.DayStart).Scan(&dailyCount); err != nil {
		return err
	}
	if dailyCount <= assignment.DailyThreshold {
		return transaction.Commit(ctx)
	}

	if err := transaction.QueryRow(ctx, `INSERT INTO albums
		(id, owner_id, title, description, automatic, auto_key)
		VALUES ($1, $2, $3, '', TRUE, $4)
		ON CONFLICT (owner_id, auto_key) WHERE auto_key IS NOT NULL
		DO UPDATE SET title = EXCLUDED.title
		RETURNING id`, assignment.DailyAlbumID, assignment.OwnerID,
		assignment.DailyTitle, assignment.DailyKey).Scan(&dailyAlbumID); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO album_media (album_id, upload_id)
		SELECT $1, m.upload_id FROM media_uploads m
		WHERE m.deleted_at IS NULL AND (m.owner_id = $2 OR EXISTS (
			SELECT 1 FROM user_media_shares share
			WHERE share.owner_id = m.owner_id AND share.user_id = $2))
		AND m.created_at >= $3 AND m.created_at < $3 + INTERVAL '1 day'
		ON CONFLICT DO NOTHING`, dailyAlbumID, assignment.OwnerID, assignment.DayStart); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM album_media
		WHERE album_id = $1 AND upload_id IN (
			SELECT m.upload_id FROM media_uploads m
			WHERE m.deleted_at IS NULL AND (m.owner_id = $2 OR EXISTS (
				SELECT 1 FROM user_media_shares share
				WHERE share.owner_id = m.owner_id AND share.user_id = $2))
			AND m.created_at >= $3 AND m.created_at < $3 + INTERVAL '1 day')`,
		weeklyAlbumID, assignment.OwnerID, assignment.DayStart); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM albums a
		WHERE a.id = $1 AND a.automatic = TRUE
		AND NOT EXISTS (SELECT 1 FROM album_media am WHERE am.album_id = a.id)`, weeklyAlbumID); err != nil {
		return err
	}
	return transaction.Commit(ctx)
}
