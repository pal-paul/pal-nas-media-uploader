package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (store *Postgres) Close() { store.pool.Close() }

func (store *Postgres) Migrate(ctx context.Context) error {
	_, err := store.pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id UUID PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	totp_secret TEXT,
	role TEXT NOT NULL DEFAULT 'admin' CHECK (role IN ('admin', 'user')),
	upload_folder TEXT,
	totp_required BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'admin';
ALTER TABLE users ADD COLUMN IF NOT EXISTS upload_folder TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_required BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users SET totp_required = TRUE WHERE role = 'admin' AND totp_required = FALSE;
CREATE TABLE IF NOT EXISTS auth_challenges (
	token_hash TEXT PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	pending_totp_secret TEXT,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS auth_sessions (
	token_hash TEXT PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS auth_sessions_expires_at_idx ON auth_sessions(expires_at);
CREATE TABLE IF NOT EXISTS media_uploads (
	upload_id TEXT PRIMARY KEY,
	owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
	filename TEXT NOT NULL,
	mime_type TEXT NOT NULL,
	size BIGINT NOT NULL CHECK (size > 0),
	sha256 TEXT NOT NULL,
	media_path TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL
);
ALTER TABLE media_uploads ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE media_uploads ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS media_uploads_owner_id_idx ON media_uploads(owner_id);
CREATE TABLE IF NOT EXISTS media_shares (
	upload_id TEXT NOT NULL REFERENCES media_uploads(upload_id) ON DELETE CASCADE,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (upload_id, user_id)
);
CREATE TABLE IF NOT EXISTS user_media_shares (
	owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (owner_id, user_id),
	CHECK (owner_id <> user_id)
);
INSERT INTO user_media_shares (owner_id, user_id)
	SELECT DISTINCT media.owner_id, share.user_id
	FROM media_shares share
	JOIN media_uploads media ON media.upload_id = share.upload_id
	WHERE media.owner_id IS NOT NULL AND media.owner_id <> share.user_id
	ON CONFLICT DO NOTHING;
DELETE FROM media_shares;
CREATE TABLE IF NOT EXISTS albums (
	id UUID PRIMARY KEY,
	owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
	description TEXT NOT NULL DEFAULT '',
	automatic BOOLEAN NOT NULL DEFAULT FALSE,
	auto_key TEXT,
	position INTEGER,
	cover_media_id TEXT REFERENCES media_uploads(upload_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE albums ADD COLUMN IF NOT EXISTS automatic BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE albums ADD COLUMN IF NOT EXISTS auto_key TEXT;
CREATE INDEX IF NOT EXISTS albums_owner_position_idx ON albums(owner_id, position, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS albums_owner_auto_key_idx ON albums(owner_id, auto_key) WHERE auto_key IS NOT NULL;
CREATE TABLE IF NOT EXISTS album_media (
	album_id UUID NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	upload_id TEXT NOT NULL REFERENCES media_uploads(upload_id) ON DELETE CASCADE,
	added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (album_id, upload_id)
);
CREATE TABLE IF NOT EXISTS media_preferences (
	upload_id TEXT NOT NULL REFERENCES media_uploads(upload_id) ON DELETE CASCADE,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	favorite BOOLEAN NOT NULL DEFAULT FALSE,
	PRIMARY KEY (upload_id, user_id)
);
`
