package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgIdempotencyRepository stores replayable responses in Postgres.
type PgIdempotencyRepository struct {
	pool *pgxpool.Pool
}

func NewIdempotencyRepository(pool *pgxpool.Pool) *PgIdempotencyRepository {
	return &PgIdempotencyRepository{pool: pool}
}

func (r *PgIdempotencyRepository) Get(ctx context.Context, realmID domain.RealmID, endpoint, key string) (*domain.IdempotencyKey, error) {
	var record domain.IdempotencyKey
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, realm_id::text, key, endpoint, request_hash,
			COALESCE(response, '{}'::jsonb), created_at, expires_at
		FROM idempotency_key
		WHERE realm_id = $1::text::uuid AND endpoint = $2 AND key = $3
			AND expires_at > now()`,
		string(realmID), endpoint, key,
	).Scan(&record.ID, &record.RealmID, &record.Key, &record.Endpoint,
		&record.RequestHash, &record.Response, &record.CreatedAt, &record.ExpiresAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &record, nil
}

func (r *PgIdempotencyRepository) Save(ctx context.Context, record *domain.IdempotencyKey) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO idempotency_key (id, realm_id, key, endpoint, request_hash,
			response, created_at, expires_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6::jsonb, $7, $8)`,
		record.ID, string(record.RealmID), record.Key, record.Endpoint,
		record.RequestHash, jsonValue(record.Response), record.CreatedAt,
		record.ExpiresAt)
	return mapError(err)
}
