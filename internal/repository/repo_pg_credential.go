package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// PgCredentialRepository stores password hashes and Postgres lockout state.
type PgCredentialRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialRepository(pool *pgxpool.Pool) *PgCredentialRepository {
	return &PgCredentialRepository{pool: pool}
}

func (r *PgCredentialRepository) Upsert(ctx context.Context, credential *domain.Credential) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO user_credential (user_id, password_hash, algorithm, params, updated_at)
		VALUES ($1::text::uuid, $2, $3, $4::jsonb, now())
		ON CONFLICT (user_id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			algorithm = EXCLUDED.algorithm,
			params = EXCLUDED.params,
			failed_attempts = 0,
			locked_until = NULL,
			updated_at = now()
		RETURNING updated_at`,
		string(credential.UserID), credential.PasswordHash, credential.Algorithm,
		jsonValue(credential.Params),
	).Scan(&credential.UpdatedAt)
	return mapError(err)
}

func (r *PgCredentialRepository) GetByUserID(ctx context.Context, userID domain.UserID) (*domain.Credential, error) {
	var credential domain.Credential
	err := r.pool.QueryRow(ctx, `
		SELECT user_id::text, password_hash, algorithm, params,
			failed_attempts, locked_until, updated_at
		FROM user_credential WHERE user_id = $1::text::uuid`, string(userID),
	).Scan(&credential.UserID, &credential.PasswordHash, &credential.Algorithm,
		&credential.Params, &credential.FailedAttempts, &credential.LockedUntil,
		&credential.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &credential, nil
}

// RecordFailedLogin increments the failure counter and locks the account when
// the threshold is reached. An expired lock starts a fresh window.
func (r *PgCredentialRepository) RecordFailedLogin(ctx context.Context, userID domain.UserID, now time.Time, threshold int, lockWindow time.Duration) (int, *time.Time, error) {
	if threshold <= 0 {
		threshold = 5
	}
	var failed int
	var lockedUntil *time.Time
	err := r.pool.QueryRow(ctx, `
		UPDATE user_credential
		SET failed_attempts = CASE
				WHEN locked_until IS NOT NULL AND locked_until <= $2 THEN 1
				ELSE failed_attempts + 1
			END,
			locked_until = CASE
				WHEN (CASE
					WHEN locked_until IS NOT NULL AND locked_until <= $2 THEN 1
					ELSE failed_attempts + 1
				END) >= $3 THEN $2 + make_interval(secs => $4)
				ELSE NULL
			END,
			updated_at = now()
		WHERE user_id = $1::text::uuid
		RETURNING failed_attempts, locked_until`,
		string(userID), now, threshold, int(lockWindow.Seconds()),
	).Scan(&failed, &lockedUntil)
	return failed, lockedUntil, mapError(err)
}

// ChangePassword updates the hash and revokes every other session and refresh
// token for the user in one transaction.
func (r *PgCredentialRepository) ChangePassword(ctx context.Context, userID domain.UserID, keepSessionID domain.SessionID, passwordHash string, params []byte, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE user_credential
			SET password_hash = $2, algorithm = 'argon2id', params = $3::jsonb,
				failed_attempts = 0, locked_until = NULL, updated_at = now()
			WHERE user_id = $1::text::uuid`,
			string(userID), passwordHash, jsonValue(params)); err != nil {
			return mapError(err)
		}
		if err := revokeSessionsTx(ctx, tx, userID, keepSessionID, time.Now().UTC()); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}
