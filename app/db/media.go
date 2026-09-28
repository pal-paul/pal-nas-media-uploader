package store

import (
	"context"

	"pal-nas-media-uploader/app/uploader"
)

func (store *Postgres) SaveCompletedMedia(ctx context.Context, media uploader.CompletedMedia) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO media_uploads
		(upload_id, filename, mime_type, size, sha256, media_path, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (upload_id) DO UPDATE SET
			filename = EXCLUDED.filename,
			mime_type = EXCLUDED.mime_type,
			size = EXCLUDED.size,
			sha256 = EXCLUDED.sha256,
			media_path = EXCLUDED.media_path,
			created_at = EXCLUDED.created_at`,
		media.UploadID, media.Filename, media.MimeType, media.Size, media.SHA256, media.MediaPath, media.CreatedAt)
	return err
}
