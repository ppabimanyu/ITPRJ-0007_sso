package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgSigningKeyRepository stores encrypted RSA private keys.
type PgSigningKeyRepository struct {
	pool *pgxpool.Pool
}

func NewSigningKeyRepository(pool *pgxpool.Pool) *PgSigningKeyRepository {
	return &PgSigningKeyRepository{pool: pool}
}

func (r *PgSigningKeyRepository) Create(ctx context.Context, key *domain.RealmSigningKey) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO realm_signing_key (id, realm_id, kid, algorithm, private_key_encrypted, active, retire_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6, $7)
		RETURNING created_at`,
		string(key.ID), string(key.RealmID), key.KID, key.Algorithm,
		key.PrivateKeyEncrypted, key.Active, key.RetireAt,
	).Scan(&key.CreatedAt)
	return mapError(err)
}

func (r *PgSigningKeyRepository) ListActive(ctx context.Context, realmID domain.RealmID) ([]domain.RealmSigningKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, realm_id::text, kid, algorithm, private_key_encrypted, active, created_at, retire_at
		FROM realm_signing_key
		WHERE realm_id = $1::text::uuid AND active
		ORDER BY created_at DESC`, string(realmID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var keys []domain.RealmSigningKey
	for rows.Next() {
		var key domain.RealmSigningKey
		if err := rows.Scan(&key.ID, &key.RealmID, &key.KID, &key.Algorithm,
			&key.PrivateKeyEncrypted, &key.Active, &key.CreatedAt, &key.RetireAt); err != nil {
			return nil, mapError(err)
		}
		keys = append(keys, key)
	}
	return keys, mapError(rows.Err())
}
