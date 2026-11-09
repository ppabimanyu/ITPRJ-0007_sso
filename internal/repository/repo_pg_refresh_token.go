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

const refreshTokenColumns = `id::text, session_id::text, coalesce(client_id::text, ''),
	token_hash, family_id::text, parent_id::text, scope, expires_at,
	absolute_expires_at, used_at, replaced_by::text, revoked_at, created_at`

// PgRefreshTokenRepository owns opaque refresh tokens. Every mutation that
// touches more than one row runs in one transaction.
type PgRefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *PgRefreshTokenRepository {
	return &PgRefreshTokenRepository{pool: pool}
}

func (r *PgRefreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO refresh_token (id, session_id, client_id, token_hash, family_id,
			parent_id, scope, expires_at, absolute_expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, $5::text::uuid,
			NULLIF($6, '')::uuid, $7, $8, $9)
		RETURNING created_at`,
		token.ID, string(token.SessionID), string(token.ClientID), token.TokenHash,
		token.FamilyID, deref(token.ParentID), token.Scope, token.ExpiresAt,
		token.AbsoluteExpiresAt,
	).Scan(&token.CreatedAt)
}

func (r *PgRefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	return scanRefreshToken(r.pool.QueryRow(ctx, `
		SELECT `+refreshTokenColumns+`
		FROM refresh_token WHERE token_hash = $1`, hash))
}

// Rotate marks the old token used and inserts its replacement in one
// transaction. A presented token that was already used is reuse: the whole
// family is revoked before the transaction commits.
func (r *PgRefreshTokenRepository) Rotate(ctx context.Context, oldHash, clientID, newHash, newID string, now time.Time, idleTTL time.Duration, audit *domain.AuditEvent) (*domain.RefreshRotation, error) {
	result := &domain.RefreshRotation{}
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		old, err := scanRefreshToken(tx.QueryRow(ctx, `
			SELECT `+refreshTokenColumns+`
			FROM refresh_token WHERE token_hash = $1
			FOR UPDATE`, oldHash))
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if string(old.ClientID) != clientID {
			return domain.ErrNotFound
		}
		switch classifyRefreshToken(old, now) {
		case refreshTokenRevoked:
			return domain.ErrNotFound
		case refreshTokenReused:
			if _, err := tx.Exec(ctx, `
				UPDATE refresh_token SET revoked_at = $2
				WHERE family_id = $1::text::uuid AND revoked_at IS NULL`,
				old.FamilyID, now); err != nil {
				return mapError(err)
			}
			if err := insertAuditTx(ctx, tx, audit); err != nil {
				return err
			}
			result.Reused = true
			result.Token = old
			return nil
		case refreshTokenExpired:
			return domain.ErrRefreshExpired
		}

		expiresAt := now.Add(idleTTL)
		if expiresAt.After(old.AbsoluteExpiresAt) {
			expiresAt = old.AbsoluteExpiresAt
		}
		next := &domain.RefreshToken{
			ID:                newID,
			SessionID:         old.SessionID,
			ClientID:          old.ClientID,
			TokenHash:         newHash,
			FamilyID:          old.FamilyID,
			ParentID:          &old.ID,
			Scope:             old.Scope,
			ExpiresAt:         expiresAt,
			AbsoluteExpiresAt: old.AbsoluteExpiresAt,
			CreatedAt:         now,
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO refresh_token (id, session_id, client_id, token_hash, family_id,
				parent_id, scope, expires_at, absolute_expires_at, created_at)
			VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, $5::text::uuid,
				$6::text::uuid, $7, $8, $9, $10)`,
			next.ID, string(next.SessionID), string(next.ClientID), next.TokenHash,
			next.FamilyID, old.ID, next.Scope, next.ExpiresAt,
			next.AbsoluteExpiresAt, next.CreatedAt); err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE refresh_token
			SET used_at = $2, replaced_by = $3::text::uuid
			WHERE id = $1::text::uuid`,
			old.ID, now, next.ID); err != nil {
			return mapError(err)
		}
		result.Token = next
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *PgRefreshTokenRepository) RevokeFamily(ctx context.Context, familyID string, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE refresh_token SET revoked_at = now()
			WHERE family_id = $1::text::uuid AND revoked_at IS NULL`,
			familyID); err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgRefreshTokenRepository) Revoke(ctx context.Context, hash, clientID string, audit *domain.AuditEvent) (*domain.RefreshToken, error) {
	var token *domain.RefreshToken
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		row, err := scanRefreshToken(tx.QueryRow(ctx, `
			SELECT `+refreshTokenColumns+`
			FROM refresh_token WHERE token_hash = $1
			FOR UPDATE`, hash))
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if string(row.ClientID) != clientID {
			return domain.ErrNotFound
		}
		now := time.Now().UTC()
		if row.RevokedAt == nil {
			if _, err := tx.Exec(ctx, `
				UPDATE refresh_token SET revoked_at = $2 WHERE id = $1::text::uuid`,
				row.ID, now); err != nil {
				return mapError(err)
			}
			row.RevokedAt = &now
		}
		if _, err := tx.Exec(ctx, `
			UPDATE refresh_token SET revoked_at = $2
			WHERE family_id = $1::text::uuid AND revoked_at IS NULL`,
			row.FamilyID, now); err != nil {
			return mapError(err)
		}
		if err := insertAuditTx(ctx, tx, audit); err != nil {
			return err
		}
		token = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

func scanRefreshToken(row rowScanner) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	err := row.Scan(&token.ID, &token.SessionID, &token.ClientID, &token.TokenHash,
		&token.FamilyID, &token.ParentID, &token.Scope, &token.ExpiresAt,
		&token.AbsoluteExpiresAt, &token.UsedAt, &token.ReplacedBy,
		&token.RevokedAt, &token.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &token, nil
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type refreshTokenState int

const (
	refreshTokenValid refreshTokenState = iota
	refreshTokenReused
	refreshTokenExpired
	refreshTokenRevoked
)

// classifyRefreshToken is the reuse rule applied under the row lock: a used
// token means the family was stolen, a revoked token is simply gone, and an
// expired token fails without touching the family.
func classifyRefreshToken(token *domain.RefreshToken, now time.Time) refreshTokenState {
	switch {
	case token.RevokedAt != nil:
		return refreshTokenRevoked
	case token.UsedAt != nil:
		return refreshTokenReused
	case !token.ExpiresAt.After(now) || !token.AbsoluteExpiresAt.After(now):
		return refreshTokenExpired
	default:
		return refreshTokenValid
	}
}
