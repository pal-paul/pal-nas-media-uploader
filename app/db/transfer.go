package store

import (
	"context"
	"errors"

	"pal-nas-media-uploader/app/transfer"

	"github.com/google/uuid"
)

func (store *Postgres) ExportManifest(ctx context.Context, ownerID string) (transfer.Manifest, error) {
	manifest := transfer.Manifest{Media: make([]transfer.Media, 0), Albums: make([]transfer.Album, 0)}
	mediaRows, err := store.pool.Query(ctx, `SELECT upload_id, filename, mime_type, size, sha256, created_at
		FROM media_uploads WHERE owner_id = $1 AND deleted_at IS NULL ORDER BY created_at`, ownerID)
	if err != nil {
		return manifest, err
	}
	for mediaRows.Next() {
		var item transfer.Media
		if err := mediaRows.Scan(&item.ID, &item.Filename, &item.MimeType, &item.Size, &item.SHA256, &item.CreatedAt); err != nil {
			mediaRows.Close()
			return manifest, err
		}
		manifest.Media = append(manifest.Media, item)
	}
	if err := mediaRows.Err(); err != nil {
		mediaRows.Close()
		return manifest, err
	}
	mediaRows.Close()
	albumRows, err := store.pool.Query(ctx, `SELECT album.title, album.description,
		COALESCE(array_agg(item.upload_id ORDER BY item.added_at) FILTER (WHERE item.upload_id IS NOT NULL), '{}')
		FROM albums album LEFT JOIN album_media item ON item.album_id = album.id
		WHERE album.owner_id = $1 AND album.automatic = FALSE GROUP BY album.id ORDER BY album.created_at`, ownerID)
	if err != nil {
		return manifest, err
	}
	defer albumRows.Close()
	for albumRows.Next() {
		var album transfer.Album
		if err := albumRows.Scan(&album.Title, &album.Description, &album.MediaIDs); err != nil {
			return manifest, err
		}
		manifest.Albums = append(manifest.Albums, album)
	}
	return manifest, albumRows.Err()
}

func (store *Postgres) ImportManifest(ctx context.Context, ownerID, sessionID string, manifest transfer.Manifest) (int, error) {
	if _, err := store.pool.Exec(ctx, `INSERT INTO import_sessions (id, owner_id, status) VALUES ($1, $2, 'processing')`, sessionID, ownerID); err != nil {
		return 0, err
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	for _, album := range manifest.Albums {
		if album.Title == "" {
			return 0, errors.New("album title is required")
		}
		albumID := uuid.NewString()
		if _, err := transaction.Exec(ctx, `INSERT INTO albums (id, owner_id, title, description)
			VALUES ($1, $2, $3, $4)`, albumID, ownerID, album.Title, album.Description); err != nil {
			return 0, err
		}
		if len(album.MediaIDs) > 0 {
			if _, err := transaction.Exec(ctx, `INSERT INTO album_media (album_id, upload_id)
				SELECT $1, upload_id FROM media_uploads WHERE owner_id = $2 AND deleted_at IS NULL AND upload_id = ANY($3)
				ON CONFLICT DO NOTHING`, albumID, ownerID, album.MediaIDs); err != nil {
				return 0, err
			}
		}
	}
	if _, err := transaction.Exec(ctx, `UPDATE import_sessions SET status = 'completed', imported_items = $2,
		updated_at = now() WHERE id = $1`, sessionID, len(manifest.Albums)); err != nil {
		return 0, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return 0, err
	}
	return len(manifest.Albums), nil
}
