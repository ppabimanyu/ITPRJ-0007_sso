package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgRealmRepository stores realms.
type PgRealmRepository struct {
	pool *pgxpool.Pool
}

func NewRealmRepository(pool *pgxpool.Pool) *PgRealmRepository {
	return &PgRealmRepository{pool: pool}
}

func (r *PgRealmRepository) Create(ctx context.Context, realm *domain.Realm) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO realm (id, name, issuer, settings)
		VALUES ($1::text::uuid, $2, $3, $4::jsonb)
		RETURNING created_at, updated_at`,
		string(realm.ID), realm.Name, realm.Issuer, jsonValue(realm.Settings),
	).Scan(&realm.CreatedAt, &realm.UpdatedAt)
	return mapError(err)
}

func (r *PgRealmRepository) GetByID(ctx context.Context, id domain.RealmID) (*domain.Realm, error) {
	return r.getOne(ctx, `
		SELECT id::text, name, issuer, settings, created_at, updated_at
		FROM realm WHERE id = $1::text::uuid`, string(id))
}

func (r *PgRealmRepository) GetByName(ctx context.Context, name string) (*domain.Realm, error) {
	return r.getOne(ctx, `
		SELECT id::text, name, issuer, settings, created_at, updated_at
		FROM realm WHERE name = $1`, name)
}

func (r *PgRealmRepository) getOne(ctx context.Context, query string, args ...any) (*domain.Realm, error) {
	var realm domain.Realm
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&realm.ID, &realm.Name, &realm.Issuer, &realm.Settings,
		&realm.CreatedAt, &realm.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &realm, nil
}
