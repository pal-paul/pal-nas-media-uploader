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
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
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
	filename TEXT NOT NULL,
	mime_type TEXT NOT NULL,
	size BIGINT NOT NULL CHECK (size > 0),
	sha256 TEXT NOT NULL,
	media_path TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL
);
`
