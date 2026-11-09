package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

const userColumns = `id::text, realm_id::text, email, username, name, given_name,
	family_name, locale, email_verified_at, disabled_at, created_at, updated_at`

const userColumnsQualified = `u.id::text, u.realm_id::text, u.email, u.username,
	u.name, u.given_name, u.family_name, u.locale, u.email_verified_at,
	u.disabled_at, u.created_at, u.updated_at`

// PgUserRepository stores users.
type PgUserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *PgUserRepository {
	return &PgUserRepository{pool: pool}
}

func (r *PgUserRepository) Create(ctx context.Context, user *domain.User) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO "user" (id, realm_id, email, username, name, given_name,
			family_name, locale, email_verified_at, disabled_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`,
		string(user.ID), string(user.RealmID), user.Email, nullString(user.Username),
		user.Name, user.GivenName, user.FamilyName, nullString(user.Locale),
		user.EmailVerifiedAt, user.DisabledAt,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
	return mapError(err)
}

func (r *PgUserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM "user" WHERE id = $1::text::uuid`, string(id))
}

func (r *PgUserRepository) GetByEmail(ctx context.Context, realmID domain.RealmID, email string) (*domain.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+`
		FROM "user" WHERE realm_id = $1::text::uuid AND lower(email) = lower($2)`,
		string(realmID), email)
}

func (r *PgUserRepository) UpdateProfile(ctx context.Context, user *domain.User) error {
	err := r.pool.QueryRow(ctx, `
		UPDATE "user"
		SET name = $2, given_name = $3, family_name = $4, locale = $5,
			updated_at = now()
		WHERE id = $1::text::uuid
		RETURNING updated_at`,
		string(user.ID), user.Name, user.GivenName, user.FamilyName,
		nullString(user.Locale),
	).Scan(&user.UpdatedAt)
	return mapError(err)
}

func (r *PgUserRepository) MFAEnrolled(ctx context.Context, userID domain.UserID) (bool, error) {
	var enrolled bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM mfa_totp
			WHERE user_id = $1::text::uuid AND confirmed_at IS NOT NULL
		)`, string(userID)).Scan(&enrolled)
	return enrolled, mapError(err)
}

func (r *PgUserRepository) ListByRealm(ctx context.Context, realmID domain.RealmID, limit, offset int) ([]domain.User, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+`
		FROM "user" WHERE realm_id = $1::text::uuid
		ORDER BY created_at, id LIMIT $2 OFFSET $3`,
		string(realmID), limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, mapError(err)
		}
		users = append(users, *user)
	}
	return users, mapError(rows.Err())
}

func (r *PgUserRepository) getOne(ctx context.Context, query string, args ...any) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, query, args...))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	var user domain.User
	err := row.Scan(
		&user.ID, &user.RealmID, &user.Email, &user.Username, &user.Name,
		&user.GivenName, &user.FamilyName, &user.Locale, &user.EmailVerifiedAt,
		&user.DisabledAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &user, nil
}
