package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgRoleRepository stores realm roles and user assignments.
type PgRoleRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) *PgRoleRepository {
	return &PgRoleRepository{pool: pool}
}

func (r *PgRoleRepository) Create(ctx context.Context, role *domain.Role) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO role (id, realm_id, name, description)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4)
		RETURNING created_at`,
		string(role.ID), string(role.RealmID), role.Name, role.Description,
	).Scan(&role.CreatedAt)
	return mapError(err)
}

func (r *PgRoleRepository) GetByName(ctx context.Context, realmID domain.RealmID, name string) (*domain.Role, error) {
	var role domain.Role
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, realm_id::text, name, description, created_at
		FROM role WHERE realm_id = $1::text::uuid AND lower(name) = lower($2)`,
		string(realmID), name,
	).Scan(&role.ID, &role.RealmID, &role.Name, &role.Description, &role.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &role, nil
}

func (r *PgRoleRepository) ListByRealm(ctx context.Context, realmID domain.RealmID) ([]domain.Role, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, realm_id::text, name, description, created_at
		FROM role WHERE realm_id = $1::text::uuid ORDER BY name`, string(realmID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.RealmID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		roles = append(roles, role)
	}
	return roles, mapError(rows.Err())
}

func (r *PgRoleRepository) AssignToUser(ctx context.Context, realmID domain.RealmID, userID domain.UserID, roleID domain.RoleID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_role (realm_id, user_id, role_id)
		VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid)
		ON CONFLICT (user_id, role_id) DO NOTHING`,
		string(realmID), string(userID), string(roleID))
	return mapError(err)
}

func (r *PgRoleRepository) ListByUser(ctx context.Context, realmID domain.RealmID, userID domain.UserID) ([]domain.Role, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id::text, r.realm_id::text, r.name, r.description, r.created_at
		FROM role r
		JOIN user_role ur ON ur.role_id = r.id
		WHERE ur.realm_id = $1::text::uuid AND ur.user_id = $2::text::uuid
		ORDER BY r.name`, string(realmID), string(userID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.RealmID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		roles = append(roles, role)
	}
	return roles, mapError(rows.Err())
}

func (r *PgRoleRepository) ListGroupsByUser(ctx context.Context, realmID domain.RealmID, userID domain.UserID) ([]domain.Group, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT g.id::text, g.realm_id::text, g.name, g.description, g.created_at
		FROM "group" g
		JOIN user_group ug ON ug.group_id = g.id
		WHERE ug.realm_id = $1::text::uuid AND ug.user_id = $2::text::uuid
		ORDER BY g.name`, string(realmID), string(userID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var groups []domain.Group
	for rows.Next() {
		var group domain.Group
		if err := rows.Scan(&group.ID, &group.RealmID, &group.Name, &group.Description, &group.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		groups = append(groups, group)
	}
	return groups, mapError(rows.Err())
}
