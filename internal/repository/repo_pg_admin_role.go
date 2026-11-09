package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// PgAdminRoleRepository writes roles with their audit event.
type PgAdminRoleRepository struct {
	pool *pgxpool.Pool
}

func NewAdminRoleRepository(pool *pgxpool.Pool) *PgAdminRoleRepository {
	return &PgAdminRoleRepository{pool: pool}
}

func (r *PgAdminRoleRepository) CreateWithAudit(ctx context.Context, role *domain.Role, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO role (id, realm_id, name, description)
			VALUES ($1::text::uuid, $2::text::uuid, $3, $4)
			RETURNING created_at`,
			string(role.ID), string(role.RealmID), role.Name, role.Description,
		).Scan(&role.CreatedAt)
		if err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

var _ domain.AdminRoleRepository = (*PgAdminRoleRepository)(nil)
