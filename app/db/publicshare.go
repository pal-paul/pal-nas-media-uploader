package store

import (
	"context"
	"errors"
	"time"

	"pal-nas-media-uploader/app/publicshare"
)

func (store *Postgres) CreatePublicLink(ctx context.Context, link publicshare.Link, mediaID, albumID string) error {
	result, err := store.pool.Exec(ctx, `INSERT INTO public_shares
		(id, owner_id, media_id, album_id, password_hash, expires_at)
		SELECT $1, $2, NULLIF($3, ''), NULLIF($4, '')::uuid, NULLIF($5, ''), $6
		WHERE ($3 <> '' AND EXISTS (SELECT 1 FROM media_uploads WHERE upload_id = $3 AND owner_id = $2 AND deleted_at IS NULL))
		OR ($4 <> '' AND EXISTS (SELECT 1 FROM albums WHERE id = $4 AND owner_id = $2))`,
		link.ID, link.OwnerID, mediaID, albumID, link.PasswordHash, link.ExpiresAt)
	if err == nil && result.RowsAffected() == 0 {
		return errors.New("owned share target not found")
	}
	return err
}

func (store *Postgres) ResolvePublicLink(ctx context.Context, id string, now time.Time) (publicshare.Link, error) {
	var link publicshare.Link
	var mediaID, albumID string
	err := store.pool.QueryRow(ctx, `UPDATE public_shares SET access_count = access_count + 1
		WHERE id = $1 AND expires_at > $2
		RETURNING id, owner_id, COALESCE(media_id, ''), COALESCE(album_id::text, ''),
		COALESCE(password_hash, ''), expires_at`, id, now).Scan(
		&link.ID, &link.OwnerID, &mediaID, &albumID, &link.PasswordHash, &link.ExpiresAt)
	if err != nil {
		return link, err
	}
	if mediaID != "" {
		var file publicshare.File
		err = store.pool.QueryRow(ctx, `SELECT filename, mime_type, media_path, size FROM media_uploads
			WHERE upload_id = $1 AND deleted_at IS NULL`, mediaID).Scan(&file.Filename, &file.MimeType, &file.MediaPath, &file.Size)
		link.Title = file.Filename
		link.Files = []publicshare.File{file}
		return link, err
	}
	err = store.pool.QueryRow(ctx, `SELECT title FROM albums WHERE id = $1`, albumID).Scan(&link.Title)
	if err != nil {
		return link, err
	}
	rows, err := store.pool.Query(ctx, `SELECT media.filename, media.mime_type, media.media_path, media.size
		FROM album_media item JOIN media_uploads media ON media.upload_id = item.upload_id
		WHERE item.album_id = $1 AND media.deleted_at IS NULL ORDER BY item.added_at`, albumID)
	if err != nil {
		return link, err
	}
	defer rows.Close()
	for rows.Next() {
		var file publicshare.File
		if err := rows.Scan(&file.Filename, &file.MimeType, &file.MediaPath, &file.Size); err != nil {
			return link, err
		}
		link.Files = append(link.Files, file)
	}
	return link, rows.Err()
}

func (store *Postgres) DeletePublicLink(ctx context.Context, id, ownerID string) error {
	result, err := store.pool.Exec(ctx, `DELETE FROM public_shares WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err == nil && result.RowsAffected() == 0 {
		return errors.New("public link not found")
	}
	return err
}
