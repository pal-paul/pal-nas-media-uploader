package store

import (
	"context"

	"pal-nas-media-uploader/app/uploader"
)

func (store *Postgres) OwnerStorageBytes(ctx context.Context, ownerID string) (int64, error) {
	var size int64
	err := store.pool.QueryRow(ctx, `SELECT COALESCE(SUM(size), 0) FROM media_uploads WHERE owner_id = $1`, ownerID).Scan(&size)
	return size, err
}

func (store *Postgres) SaveCompletedMedia(ctx context.Context, media uploader.CompletedMedia) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO media_uploads
		(upload_id, owner_id, filename, mime_type, size, sha256, media_path, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (upload_id) DO UPDATE SET
			owner_id = EXCLUDED.owner_id,
			filename = EXCLUDED.filename,
			mime_type = EXCLUDED.mime_type,
			size = EXCLUDED.size,
			sha256 = EXCLUDED.sha256,
			media_path = EXCLUDED.media_path,
			created_at = EXCLUDED.created_at`,
		media.UploadID, media.OwnerID, media.Filename, media.MimeType, media.Size, media.SHA256, media.MediaPath, media.CreatedAt)
	return err
}
