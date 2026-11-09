package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgCredentialRepository stores password hashes.
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
		SELECT user_id::text, password_hash, algorithm, params, updated_at
		FROM user_credential WHERE user_id = $1::text::uuid`, string(userID),
	).Scan(&credential.UserID, &credential.PasswordHash, &credential.Algorithm,
		&credential.Params, &credential.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &credential, nil
}
