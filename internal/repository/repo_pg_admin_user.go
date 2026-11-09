package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

const adminUserColumns = `u.id::text, u.realm_id::text, u.email, u.username,
	u.name, u.given_name, u.family_name, u.locale, u.email_verified_at,
	u.disabled_at, u.created_at, u.updated_at,
	EXISTS (
		SELECT 1 FROM mfa_totp m
		WHERE m.user_id = u.id AND m.confirmed_at IS NOT NULL
	),
	COALESCE((
		SELECT jsonb_agg(jsonb_build_object('id', r.id::text, 'name', r.name) ORDER BY r.name)
		FROM user_role ur
		JOIN role r ON r.id = ur.role_id
		WHERE ur.realm_id = u.realm_id AND ur.user_id = u.id
	), '[]'::jsonb),
	COALESCE((
		SELECT jsonb_agg(jsonb_build_object('id', g.id::text, 'name', g.name) ORDER BY g.name)
		FROM user_group ug
		JOIN "group" g ON g.id = ug.group_id
		WHERE ug.realm_id = u.realm_id AND ug.user_id = u.id
	), '[]'::jsonb),
	(
		SELECT count(*) FROM session s
		WHERE s.user_id = u.id
			AND s.revoked_at IS NULL
			AND s.idle_expires_at > now()
			AND s.absolute_expires_at > now()
	)`

const adminUserFrom = ` FROM "user" u`

// PgAdminUserRepository reads the console view and owns admin user writes.
type PgAdminUserRepository struct {
	pool *pgxpool.Pool
}

func NewAdminUserRepository(pool *pgxpool.Pool) *PgAdminUserRepository {
	return &PgAdminUserRepository{pool: pool}
}

func (r *PgAdminUserRepository) List(ctx context.Context, realmID domain.RealmID, filter domain.AdminUserFilter) ([]domain.AdminUserView, string, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{string(realmID)}
	clauses := []string{"u.realm_id = $1::text::uuid"}

	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		index := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(u.email ILIKE $%d OR u.name ILIKE $%d)", index, index))
	}
	switch filter.Status {
	case "":
	case "active":
		clauses = append(clauses, "u.disabled_at IS NULL AND u.email_verified_at IS NOT NULL")
	case "pending":
		clauses = append(clauses, "u.disabled_at IS NULL AND u.email_verified_at IS NULL")
	case "disabled":
		clauses = append(clauses, "u.disabled_at IS NOT NULL")
	default:
		return nil, "", domain.ErrNotFound
	}
	if filter.After != "" {
		createdAt, id, err := decodeAdminCursor(filter.After)
		if err != nil {
			return nil, "", domain.ErrNotFound
		}
		args = append(args, createdAt, id)
		index := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(u.created_at, u.id) > ($%d::timestamptz, $%d::text::uuid)", index-1, index))
	}
	args = append(args, limit+1)
	query := fmt.Sprintf(`SELECT `+adminUserColumns+adminUserFrom+
		` WHERE `+strings.Join(clauses, " AND ")+
		` ORDER BY u.created_at, u.id LIMIT $%d`, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", mapError(err)
	}
	defer rows.Close()

	views := make([]domain.AdminUserView, 0, limit)
	for rows.Next() {
		view, err := scanAdminUserView(rows)
		if err != nil {
			return nil, "", err
		}
		views = append(views, *view)
	}
	if err := rows.Err(); err != nil {
		return nil, "", mapError(err)
	}
	next := ""
	if len(views) > limit {
		last := views[limit-1]
		next = encodeAdminCursor(last.CreatedAt, last.ID)
		views = views[:limit]
	}
	return views, next, nil
}

func (r *PgAdminUserRepository) Get(ctx context.Context, realmID domain.RealmID, userID domain.UserID) (*domain.AdminUserView, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+adminUserColumns+adminUserFrom+`
		WHERE u.realm_id = $1::text::uuid AND u.id = $2::text::uuid`,
		string(realmID), string(userID))
	return scanAdminUserView(row)
}

func (r *PgAdminUserRepository) CreateWithInvite(ctx context.Context, user *domain.User, token *domain.EmailToken, outbox *domain.OutboxEvent, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO "user" (id, realm_id, email, username, name, given_name,
				family_name, locale, email_verified_at, disabled_at)
			VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at`,
			string(user.ID), string(user.RealmID), user.Email, nullString(user.Username),
			user.Name, user.GivenName, user.FamilyName, nullString(user.Locale),
			user.EmailVerifiedAt, user.DisabledAt,
		).Scan(&user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return mapError(err)
		}
		if token != nil {
			if err := insertEmailTokenTx(ctx, tx, token); err != nil {
				return err
			}
		}
		if err := insertOutboxTx(ctx, tx, outbox); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgAdminUserRepository) Update(ctx context.Context, realmID domain.RealmID, userID domain.UserID, update domain.AdminUserUpdate, audit *domain.AuditEvent) (*domain.User, error) {
	var updated *domain.User
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockRealmTx(ctx, tx, realmID); err != nil {
			return err
		}
		current, err := getUserForUpdateTx(ctx, tx, realmID, userID)
		if err != nil {
			return err
		}
		if update.Name != nil {
			current.Name = *update.Name
		}
		if update.GivenName != nil {
			current.GivenName = *update.GivenName
		}
		if update.FamilyName != nil {
			current.FamilyName = *update.FamilyName
		}
		if update.Locale != nil {
			current.Locale = update.Locale
		}
		disabledNow := false
		if update.Disabled != nil {
			if *update.Disabled {
				if current.DisabledAt == nil {
					admin, err := userHasRoleTx(ctx, tx, realmID, userID, domain.PlatformAdminRole)
					if err != nil {
						return err
					}
					if admin {
						others, err := otherActiveAdminsTx(ctx, tx, realmID, userID)
						if err != nil {
							return err
						}
						if others == 0 {
							return domain.ErrConflict
						}
					}
					now := time.Now().UTC()
					current.DisabledAt = &now
					disabledNow = true
				}
			} else {
				current.DisabledAt = nil
			}
		}
		err = tx.QueryRow(ctx, `
			UPDATE "user"
			SET name = $2, given_name = $3, family_name = $4, locale = $5,
				disabled_at = $6, updated_at = now()
			WHERE id = $1::text::uuid
			RETURNING updated_at`,
			string(userID), current.Name, current.GivenName, current.FamilyName,
			nullString(current.Locale), current.DisabledAt,
		).Scan(&current.UpdatedAt)
		if err != nil {
			return mapError(err)
		}
		if disabledNow {
			if err := revokeSessionsTx(ctx, tx, userID, "", time.Now().UTC()); err != nil {
				return err
			}
		}
		if err := insertAuditTx(ctx, tx, audit); err != nil {
			return err
		}
		updated = current
		return nil
	})
	return updated, err
}

// ReplaceRoles replaces the whole role set. The realm row lock serializes
// concurrent admin changes so the last-admin count cannot race.
func (r *PgAdminUserRepository) ReplaceRoles(ctx context.Context, realmID domain.RealmID, userID domain.UserID, roleIDs []domain.RoleID, audit *domain.AuditEvent) ([]domain.Role, error) {
	var assigned []domain.Role
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockRealmTx(ctx, tx, realmID); err != nil {
			return err
		}
		if _, err := getUserForUpdateTx(ctx, tx, realmID, userID); err != nil {
			return err
		}
		roles, err := selectRolesTx(ctx, tx, realmID, roleIDs)
		if err != nil {
			return err
		}
		currentAdmin, err := userHasRoleTx(ctx, tx, realmID, userID, domain.PlatformAdminRole)
		if err != nil {
			return err
		}
		nextAdmin := false
		for _, role := range roles {
			if strings.EqualFold(role.Name, domain.PlatformAdminRole) {
				nextAdmin = true
			}
		}
		if currentAdmin && !nextAdmin {
			others, err := otherActiveAdminsTx(ctx, tx, realmID, userID)
			if err != nil {
				return err
			}
			if others == 0 {
				return domain.ErrConflict
			}
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM user_role WHERE realm_id = $1::text::uuid AND user_id = $2::text::uuid`,
			string(realmID), string(userID)); err != nil {
			return mapError(err)
		}
		for _, role := range roles {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_role (realm_id, user_id, role_id)
				VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid)`,
				string(realmID), string(userID), string(role.ID)); err != nil {
				return mapError(err)
			}
		}
		if err := insertAuditTx(ctx, tx, audit); err != nil {
			return err
		}
		assigned = roles
		return nil
	})
	return assigned, err
}

func (r *PgAdminUserRepository) EnqueueInvite(ctx context.Context, token *domain.EmailToken, outbox *domain.OutboxEvent, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE email_token SET used_at = now()
			WHERE user_id = $1::text::uuid AND purpose = 'invite' AND used_at IS NULL`,
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

func scanAdminUserView(row rowScanner) (*domain.AdminUserView, error) {
	var user domain.User
	var mfaEnrolled bool
	var roleJSON []byte
	var groupJSON []byte
	var sessionCount int64
	err := row.Scan(
		&user.ID, &user.RealmID, &user.Email, &user.Username, &user.Name,
		&user.GivenName, &user.FamilyName, &user.Locale, &user.EmailVerifiedAt,
		&user.DisabledAt, &user.CreatedAt, &user.UpdatedAt, &mfaEnrolled,
		&roleJSON, &groupJSON, &sessionCount,
	)
	if err != nil {
		return nil, mapError(err)
	}
	var roles []domain.RoleRef
	if err := json.Unmarshal(roleJSON, &roles); err != nil {
		return nil, err
	}
	var groups []domain.GroupRef
	if err := json.Unmarshal(groupJSON, &groups); err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []domain.RoleRef{}
	}
	if groups == nil {
		groups = []domain.GroupRef{}
	}
	return &domain.AdminUserView{
		UserDTO: domain.UserDTO{
			ID:              string(user.ID),
			Email:           user.Email,
			Username:        user.Username,
			Name:            optionalStringValue(user.Name),
			GivenName:       optionalStringValue(user.GivenName),
			FamilyName:      optionalStringValue(user.FamilyName),
			Locale:          user.Locale,
			EmailVerifiedAt: user.EmailVerifiedAt,
			DisabledAt:      user.DisabledAt,
			MFAEnrolled:     mfaEnrolled,
			Roles:           roles,
			Groups:          groups,
			CreatedAt:       user.CreatedAt,
			UpdatedAt:       user.UpdatedAt,
		},
		SessionCount: int(sessionCount),
	}, nil
}

func optionalStringValue(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func decodeAdminCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return createdAt, parts[1], nil
}

func encodeAdminCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func lockRealmTx(ctx context.Context, tx pgx.Tx, realmID domain.RealmID) error {
	var found string
	err := tx.QueryRow(ctx, `SELECT id::text FROM realm WHERE id = $1::text::uuid FOR UPDATE`,
		string(realmID)).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return mapError(err)
}

func getUserForUpdateTx(ctx context.Context, tx pgx.Tx, realmID domain.RealmID, userID domain.UserID) (*domain.User, error) {
	return scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+`
		FROM "user" WHERE realm_id = $1::text::uuid AND id = $2::text::uuid FOR UPDATE`,
		string(realmID), string(userID)))
}

func userHasRoleTx(ctx context.Context, tx pgx.Tx, realmID domain.RealmID, userID domain.UserID, name string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_role ur
			JOIN role r ON r.id = ur.role_id
			WHERE ur.realm_id = $1::text::uuid
				AND ur.user_id = $2::text::uuid
				AND lower(r.name) = lower($3)
		)`, string(realmID), string(userID), name).Scan(&exists)
	return exists, mapError(err)
}

func otherActiveAdminsTx(ctx context.Context, tx pgx.Tx, realmID domain.RealmID, exclude domain.UserID) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM "user" u
		JOIN user_role ur ON ur.user_id = u.id AND ur.realm_id = u.realm_id
		JOIN role r ON r.id = ur.role_id AND r.realm_id = u.realm_id
		WHERE u.realm_id = $1::text::uuid
			AND u.disabled_at IS NULL
			AND lower(r.name) = lower($2)
			AND u.id <> $3::text::uuid`,
		string(realmID), domain.PlatformAdminRole, string(exclude)).Scan(&count)
	return count, mapError(err)
}

func selectRolesTx(ctx context.Context, tx pgx.Tx, realmID domain.RealmID, roleIDs []domain.RoleID) ([]domain.Role, error) {
	if len(roleIDs) == 0 {
		return []domain.Role{}, nil
	}
	ids := make([]string, 0, len(roleIDs))
	for _, id := range roleIDs {
		ids = append(ids, string(id))
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text, realm_id::text, name, description, created_at
		FROM role WHERE realm_id = $1::text::uuid AND id::text = ANY($2::text[])
		ORDER BY name`, string(realmID), ids)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	roles := make([]domain.Role, 0, len(ids))
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.RealmID, &role.Name, &role.Description,
			&role.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	if len(roles) != len(ids) {
		return nil, domain.ErrNotFound
	}
	return roles, nil
}
