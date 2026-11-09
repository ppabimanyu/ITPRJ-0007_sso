package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// PgEmailTokenRepository stores one-time email tokens and consumes them in the
// same transaction as the state change they authorize.
type PgEmailTokenRepository struct {
	pool *pgxpool.Pool
}

func NewEmailTokenRepository(pool *pgxpool.Pool) *PgEmailTokenRepository {
	return &PgEmailTokenRepository{pool: pool}
}

func (r *PgEmailTokenRepository) Create(ctx context.Context, token *domain.EmailToken, outbox *domain.OutboxEvent, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := insertEmailTokenTx(ctx, tx, token); err != nil {
			return err
		}
		if err := insertOutboxTx(ctx, tx, outbox); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

// GetValidUser reads the user behind a live token. The handler needs the email
// and name for password policy before it consumes the token.
func (r *PgEmailTokenRepository) GetValidUser(ctx context.Context, realmID domain.RealmID, tokenHash string, purpose domain.EmailTokenPurpose) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `
		SELECT `+userColumnsQualified+`
		FROM email_token t
		JOIN "user" u ON u.id = t.user_id
		WHERE u.realm_id = $1::text::uuid
			AND t.token_hash = $2
			AND t.purpose = $3
			AND t.used_at IS NULL
			AND t.expires_at > now()`,
		string(realmID), tokenHash, string(purpose)))
}

// CreatePasswordReset invalidates earlier reset tokens, inserts the new one,
// and queues the email in one transaction.
func (r *PgEmailTokenRepository) CreatePasswordReset(ctx context.Context, token *domain.EmailToken, outbox *domain.OutboxEvent, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE email_token SET used_at = now()
			WHERE user_id = $1::text::uuid AND purpose = 'reset' AND used_at IS NULL`,
			string(token.UserID)); err != nil {
			return mapError(err)
		}
		if err := insertEmailTokenTx(ctx, tx, token); err != nil {
			return err
		}
		if err := insertOutboxTx(ctx, tx, outbox); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgEmailTokenRepository) CompletePasswordReset(ctx context.Context, realmID domain.RealmID, tokenHash, passwordHash string, params []byte, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		userID, err := consumeTokenTx(ctx, tx, realmID, tokenHash, domain.EmailTokenReset)
		if err != nil {
			return err
		}
		if err := upsertCredentialTx(ctx, tx, userID, passwordHash, params); err != nil {
			return err
		}
		if err := revokeSessionsTx(ctx, tx, userID, "", time.Now().UTC()); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgEmailTokenRepository) AcceptInvite(ctx context.Context, realmID domain.RealmID, tokenHash, passwordHash string, params []byte, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		userID, err := consumeTokenTx(ctx, tx, realmID, tokenHash, domain.EmailTokenInvite)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE "user"
			SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now()
			WHERE id = $1::text::uuid`, string(userID)); err != nil {
			return mapError(err)
		}
		if err := upsertCredentialTx(ctx, tx, userID, passwordHash, params); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgEmailTokenRepository) VerifyEmail(ctx context.Context, realmID domain.RealmID, tokenHash string, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		userID, err := consumeTokenTx(ctx, tx, realmID, tokenHash, domain.EmailTokenVerify)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE "user"
			SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now()
			WHERE id = $1::text::uuid`, string(userID)); err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func consumeTokenTx(ctx context.Context, tx pgx.Tx, realmID domain.RealmID, tokenHash string, purpose domain.EmailTokenPurpose) (domain.UserID, error) {
	var userID domain.UserID
	err := tx.QueryRow(ctx, `
		UPDATE email_token t SET used_at = now()
		FROM "user" u
		WHERE t.user_id = u.id
			AND u.realm_id = $1::text::uuid
			AND t.token_hash = $2
			AND t.purpose = $3
			AND t.used_at IS NULL
			AND t.expires_at > now()
		RETURNING u.id::text`,
		string(realmID), tokenHash, string(purpose)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return userID, mapError(err)
}

func insertEmailTokenTx(ctx context.Context, tx pgx.Tx, token *domain.EmailToken) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO email_token (id, user_id, purpose, token_hash, expires_at, created_by)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, NULLIF($6, '')::uuid)`,
		token.ID, string(token.UserID), string(token.Purpose), token.TokenHash,
		token.ExpiresAt, optionalID(token.CreatedBy))
	return mapError(err)
}

func upsertCredentialTx(ctx context.Context, tx pgx.Tx, userID domain.UserID, passwordHash string, params []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_credential (user_id, password_hash, algorithm, params, updated_at)
		VALUES ($1::text::uuid, $2, 'argon2id', $3::jsonb, now())
		ON CONFLICT (user_id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			algorithm = EXCLUDED.algorithm,
			params = EXCLUDED.params,
			failed_attempts = 0,
			locked_until = NULL,
			updated_at = now()`,
		string(userID), passwordHash, jsonValue(params))
	return mapError(err)
}

func optionalID(id *domain.UserID) string {
	if id == nil {
		return ""
	}
	return string(*id)
}
