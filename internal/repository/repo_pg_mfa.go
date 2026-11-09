package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// PgMFARepository stores encrypted TOTP seeds and hashed recovery codes.
type PgMFARepository struct {
	pool *pgxpool.Pool
}

func NewMFARepository(pool *pgxpool.Pool) *PgMFARepository {
	return &PgMFARepository{pool: pool}
}

func (r *PgMFARepository) GetTOTP(ctx context.Context, userID domain.UserID) (*domain.MFATOTP, error) {
	var totp domain.MFATOTP
	err := r.pool.QueryRow(ctx, `
		SELECT user_id::text, secret_encrypted, last_used_step, confirmed_at,
			created_at, updated_at
		FROM mfa_totp WHERE user_id = $1::text::uuid`, string(userID),
	).Scan(&totp.UserID, &totp.SecretEncrypted, &totp.LastUsedStep,
		&totp.ConfirmedAt, &totp.CreatedAt, &totp.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &totp, nil
}

func (r *PgMFARepository) SaveTOTP(ctx context.Context, totp *domain.MFATOTP) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO mfa_totp (user_id, secret_encrypted, last_used_step, confirmed_at)
		VALUES ($1::text::uuid, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			secret_encrypted = EXCLUDED.secret_encrypted,
			last_used_step = EXCLUDED.last_used_step,
			confirmed_at = EXCLUDED.confirmed_at,
			updated_at = now()
		RETURNING created_at, updated_at`,
		string(totp.UserID), totp.SecretEncrypted, totp.LastUsedStep,
		totp.ConfirmedAt,
	).Scan(&totp.CreatedAt, &totp.UpdatedAt)
	return mapError(err)
}

// ClaimTOTPStep atomically accepts a step only when it is newer than the last
// accepted one. The SQL guard is what makes replay rejection concurrency safe.
func (r *PgMFARepository) ClaimTOTPStep(ctx context.Context, userID domain.UserID, step int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE mfa_totp
		SET last_used_step = $2, updated_at = now()
		WHERE user_id = $1::text::uuid
			AND confirmed_at IS NOT NULL
			AND last_used_step < $2`,
		string(userID), step)
	if err != nil {
		return false, mapError(err)
	}
	return tag.RowsAffected() > 0, nil
}

// ConfirmTOTP activates the seed, replaces recovery codes, upgrades the
// current session, and appends the audit event in one transaction.
func (r *PgMFARepository) ConfirmTOTP(ctx context.Context, userID domain.UserID, sessionID domain.SessionID, lastUsedStep int64, codes []domain.MFARecoveryCode, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE mfa_totp
			SET confirmed_at = now(), last_used_step = $2, updated_at = now()
			WHERE user_id = $1::text::uuid AND confirmed_at IS NULL`,
			string(userID), lastUsedStep)
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		if err := replaceRecoveryCodesTx(ctx, tx, userID, codes); err != nil {
			return err
		}
		if sessionID != "" {
			if _, err := tx.Exec(ctx, `
				UPDATE session
				SET acr = $3,
					amr = CASE WHEN $4 = ANY(amr) THEN amr ELSE array_append(amr, $4) END
				WHERE id = $1::text::uuid AND user_id = $2::text::uuid
					AND revoked_at IS NULL`,
				string(sessionID), string(userID), domain.ACRMFA, domain.AMROTP); err != nil {
				return mapError(err)
			}
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgMFARepository) ReplaceRecoveryCodes(ctx context.Context, userID domain.UserID, codes []domain.MFARecoveryCode, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := replaceRecoveryCodesTx(ctx, tx, userID, codes); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgMFARepository) UseRecoveryCode(ctx context.Context, userID domain.UserID, codeHash string, audit *domain.AuditEvent) (bool, error) {
	used := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE mfa_recovery_code
			SET used_at = now()
			WHERE user_id = $1::text::uuid AND code_hash = $2 AND used_at IS NULL`,
			string(userID), codeHash)
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		used = true
		return insertAuditTx(ctx, tx, audit)
	})
	return used, err
}

func (r *PgMFARepository) DeleteTOTP(ctx context.Context, userID domain.UserID, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM mfa_recovery_code WHERE user_id = $1::text::uuid`,
			string(userID)); err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM mfa_totp WHERE user_id = $1::text::uuid`,
			string(userID)); err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func replaceRecoveryCodesTx(ctx context.Context, tx pgx.Tx, userID domain.UserID, codes []domain.MFARecoveryCode) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM mfa_recovery_code WHERE user_id = $1::text::uuid`,
		string(userID)); err != nil {
		return mapError(err)
	}
	for i := range codes {
		id := codes[i].ID
		if id == "" {
			id = uuid.NewString()
		}
		createdAt := codes[i].CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO mfa_recovery_code (id, user_id, code_hash, created_at)
			VALUES ($1::text::uuid, $2::text::uuid, $3, $4)`,
			id, string(userID), codes[i].CodeHash, createdAt); err != nil {
			return mapError(err)
		}
	}
	return nil
}
