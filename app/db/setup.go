package store

import (
	"context"

	setupapp "pal-next-gallery-server/app/setup"
)

func (store *Postgres) SetupComplete(ctx context.Context) (bool, error) {
	var complete bool
	err := store.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&complete)
	return complete, err
}

func (store *Postgres) CreateInitialAdmin(ctx context.Context, account setupapp.Account) error {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('pal-initial-admin'))`); err != nil {
		return err
	}
	var configured bool
	if err := transaction.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&configured); err != nil {
		return err
	}
	if configured {
		return setupapp.ErrAlreadyConfigured
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO users
		(id, username, password_hash, role, totp_required)
		VALUES ($1, $2, $3, 'admin', TRUE)`, account.ID, account.Username, account.PasswordHash); err != nil {
		return err
	}
	return transaction.Commit(ctx)
}
