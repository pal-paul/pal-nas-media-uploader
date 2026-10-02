package store

import (
	"context"
	"time"

	"pal-next-gallery-server/app/auth"
)

func (store *Postgres) ReplaceRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, `DELETE FROM totp_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, hash := range hashes {
		if _, err := transaction.Exec(ctx, `INSERT INTO totp_recovery_codes (user_id, code_hash) VALUES ($1, $2)`, userID, hash); err != nil {
			return err
		}
	}
	return transaction.Commit(ctx)
}

func (store *Postgres) ConsumeRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	result, err := store.pool.Exec(ctx, `UPDATE totp_recovery_codes SET used_at = now()
		WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`, userID, hash)
	return err == nil && result.RowsAffected() == 1, err
}

func (store *Postgres) AuthUserCount(ctx context.Context) (int, error) {
	var count int
	err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (store *Postgres) CreateAuthUser(ctx context.Context, user auth.User) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO users (id, username, password_hash, role, upload_folder, totp_required)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)`,
		user.ID, user.Username, user.PasswordHash, user.Role, user.UploadFolder, user.TOTPRequired)
	return err
}

func (store *Postgres) GetAuthUserByUsername(ctx context.Context, username string) (auth.User, error) {
	var user auth.User
	err := store.pool.QueryRow(ctx, `SELECT id, username, password_hash, COALESCE(totp_secret, ''), role,
		COALESCE(upload_folder, ''), totp_required FROM users WHERE username = $1`, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.TOTPSecret, &user.Role, &user.UploadFolder, &user.TOTPRequired)
	return user, err
}

func (store *Postgres) CreateAuthChallenge(ctx context.Context, tokenHash, userID, pendingSecret string, expiresAt time.Time) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO auth_challenges (token_hash, user_id, pending_totp_secret, expires_at)
		VALUES ($1, $2, NULLIF($3, ''), $4)`, tokenHash, userID, pendingSecret, expiresAt)
	return err
}

func (store *Postgres) GetAuthChallenge(ctx context.Context, tokenHash string) (auth.Challenge, error) {
	var challenge auth.Challenge
	err := store.pool.QueryRow(ctx, `SELECT c.user_id, u.username, COALESCE(u.totp_secret, ''),
		COALESCE(c.pending_totp_secret, ''), c.expires_at FROM auth_challenges c
		JOIN users u ON u.id = c.user_id WHERE c.token_hash = $1`, tokenHash).
		Scan(&challenge.UserID, &challenge.Username, &challenge.TOTPSecret, &challenge.PendingSecret, &challenge.ExpiresAt)
	return challenge, err
}

func (store *Postgres) DeleteAuthChallenge(ctx context.Context, tokenHash string) (bool, error) {
	result, err := store.pool.Exec(ctx, `DELETE FROM auth_challenges WHERE token_hash = $1`, tokenHash)
	return err == nil && result.RowsAffected() == 1, err
}

func (store *Postgres) EnableTOTP(ctx context.Context, userID, secret string) (bool, error) {
	result, err := store.pool.Exec(ctx, `UPDATE users SET totp_secret = $2 WHERE id = $1 AND totp_secret IS NULL`, userID, secret)
	return err == nil && result.RowsAffected() == 1, err
}

func (store *Postgres) CreateAuthSession(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO auth_sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, tokenHash, userID, expiresAt)
	return err
}

func (store *Postgres) GetAuthSessionUser(ctx context.Context, tokenHash string, now time.Time) (auth.User, error) {
	var user auth.User
	err := store.pool.QueryRow(ctx, `SELECT u.id, u.username, u.password_hash, COALESCE(u.totp_secret, ''), u.role,
		COALESCE(u.upload_folder, ''), u.totp_required FROM auth_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > $2`, tokenHash, now).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.TOTPSecret, &user.Role, &user.UploadFolder, &user.TOTPRequired)
	return user, err
}

func (store *Postgres) DeleteAuthSession(ctx context.Context, tokenHash string) error {
	_, err := store.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (store *Postgres) CleanupExpiredAuth(ctx context.Context, now time.Time) (int64, error) {
	var deleted int64
	err := store.pool.QueryRow(ctx, `WITH deleted_challenges AS (
		DELETE FROM auth_challenges WHERE expires_at <= $1 RETURNING 1
	), deleted_sessions AS (
		DELETE FROM auth_sessions WHERE expires_at <= $1 RETURNING 1
	) SELECT (SELECT COUNT(*) FROM deleted_challenges) + (SELECT COUNT(*) FROM deleted_sessions)`, now).Scan(&deleted)
	return deleted, err
}
