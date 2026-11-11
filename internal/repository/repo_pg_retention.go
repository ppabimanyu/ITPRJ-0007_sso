package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgRetentionRepository deletes expired sessions, tokens, and failed jobs.
type PgRetentionRepository struct {
	pool *pgxpool.Pool
}

func NewRetentionRepository(pool *pgxpool.Pool) *PgRetentionRepository {
	return &PgRetentionRepository{pool: pool}
}

func (r *PgRetentionRepository) DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM session
		WHERE absolute_expires_at < $1
		   OR (revoked_at IS NOT NULL AND revoked_at < $1)`, before)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

func (r *PgRetentionRepository) DeleteExpiredRefreshTokens(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM refresh_token WHERE expires_at < $1`, before)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

func (r *PgRetentionRepository) DeleteExpiredEmailTokens(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM email_token
		WHERE expires_at < $1
		   OR (used_at IS NOT NULL AND used_at < $1)`, before)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

func (r *PgRetentionRepository) DeleteFailedOutbox(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM outbox
		WHERE status = 'failed' AND COALESCE(processed_at, created_at) < $1`, before)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

func (r *PgRetentionRepository) DeleteExpiredIdempotencyKeys(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM idempotency_key WHERE expires_at < $1`, before)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

var _ domain.RetentionRepository = (*PgRetentionRepository)(nil)
