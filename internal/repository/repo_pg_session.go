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

const sessionColumns = `s.id::text, s.realm_id::text, s.user_id::text,
	coalesce(s.token_hash, ''), coalesce(s.csrf_hash, ''),
	coalesce(host(s.ip), ''), s.user_agent, s.amr, s.auth_time, s.last_seen_at,
	s.remember_me, s.idle_expires_at, s.absolute_expires_at, s.revoked_at, s.created_at`

// PgSessionRepository stores login sessions and their token digests.
type PgSessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *PgSessionRepository {
	return &PgSessionRepository{pool: pool}
}

// CreateLoginSession inserts the session, clears any lockout for the user, and
// appends the login audit event in one transaction.
func (r *PgSessionRepository) CreateLoginSession(ctx context.Context, session *domain.Session, audit *domain.AuditEvent) error {
	if session.TokenHash == "" || session.CSRFTokenHash == "" {
		return errors.New("session token hashes are required")
	}
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO session (id, realm_id, user_id, token_hash, csrf_hash, ip,
				user_agent, amr, auth_time, last_seen_at, remember_me,
				idle_expires_at, absolute_expires_at)
			VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, $5,
				NULLIF($6, '')::inet, $7, $8, $9, $10, $11, $12, $13)`,
			string(session.ID), string(session.RealmID), string(session.UserID),
			session.TokenHash, session.CSRFTokenHash, session.IP, session.UserAgent,
			session.AMR, session.AuthTime, session.LastSeenAt, session.RememberMe,
			session.IdleExpiresAt, session.AbsoluteExpiresAt)
		if err != nil {
			return mapError(err)
		}
		_, err = tx.Exec(ctx, `
			UPDATE user_credential
			SET failed_attempts = 0, locked_until = NULL, updated_at = now()
			WHERE user_id = $1::text::uuid`, string(session.UserID))
		if err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

func (r *PgSessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.SessionWithUser, error) {
	return r.getSessionWithUser(ctx, "s.token_hash = $1", tokenHash)
}

func (r *PgSessionRepository) GetByID(ctx context.Context, id domain.SessionID) (*domain.SessionWithUser, error) {
	return r.getSessionWithUser(ctx, "s.id = $1::text::uuid", string(id))
}

func (r *PgSessionRepository) getSessionWithUser(ctx context.Context, where string, args ...any) (*domain.SessionWithUser, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+sessionColumns+`,
			u.id::text, u.realm_id::text, u.email, u.username, u.name, u.given_name,
			u.family_name, u.locale, u.email_verified_at, u.disabled_at,
			u.created_at, u.updated_at,
			EXISTS (
				SELECT 1 FROM mfa_totp m
				WHERE m.user_id = u.id AND m.confirmed_at IS NOT NULL
			)
		FROM session s
		JOIN "user" u ON u.id = s.user_id
		WHERE `+where, args...)

	result := &domain.SessionWithUser{Session: &domain.Session{}, User: &domain.User{}}
	err := row.Scan(
		&result.Session.ID, &result.Session.RealmID, &result.Session.UserID,
		&result.Session.TokenHash, &result.Session.CSRFTokenHash,
		&result.Session.IP, &result.Session.UserAgent, &result.Session.AMR,
		&result.Session.AuthTime, &result.Session.LastSeenAt,
		&result.Session.RememberMe, &result.Session.IdleExpiresAt,
		&result.Session.AbsoluteExpiresAt, &result.Session.RevokedAt,
		&result.Session.CreatedAt,
		&result.User.ID, &result.User.RealmID, &result.User.Email,
		&result.User.Username, &result.User.Name, &result.User.GivenName,
		&result.User.FamilyName, &result.User.Locale, &result.User.EmailVerifiedAt,
		&result.User.DisabledAt, &result.User.CreatedAt, &result.User.UpdatedAt,
		&result.MFAEnrolled,
	)
	if err != nil {
		return nil, mapError(err)
	}
	roles, err := r.rolesForUser(ctx, result.User.RealmID, result.User.ID)
	if err != nil {
		return nil, err
	}
	result.Roles = roles
	return result, nil
}

func (r *PgSessionRepository) rolesForUser(ctx context.Context, realmID domain.RealmID, userID domain.UserID) ([]domain.Role, error) {
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

// TouchLastSeen slides the idle window at most once per minute.
func (r *PgSessionRepository) TouchLastSeen(ctx context.Context, id domain.SessionID, at, idleExpiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE session
		SET last_seen_at = $2::timestamptz,
			idle_expires_at = LEAST($3::timestamptz, absolute_expires_at)
		WHERE id = $1::text::uuid
			AND revoked_at IS NULL
			AND last_seen_at < ($2::timestamptz - interval '1 minute')`,
		string(id), at, idleExpiresAt)
	return mapError(err)
}

func (r *PgSessionRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]domain.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+sessionColumns+`
		FROM session s
		WHERE s.user_id = $1::text::uuid
			AND s.revoked_at IS NULL
			AND s.idle_expires_at > now()
			AND s.absolute_expires_at > now()
		ORDER BY s.created_at DESC`, string(userID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	return sessions, mapError(rows.Err())
}

// RevokeForUser revokes one session only when it belongs to the user. It also
// revokes that session's refresh tokens and records the audit event.
func (r *PgSessionRepository) RevokeForUser(ctx context.Context, id domain.SessionID, userID domain.UserID, audit *domain.AuditEvent) (bool, error) {
	revoked := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var found domain.SessionID
		err := tx.QueryRow(ctx, `
			UPDATE session SET revoked_at = now()
			WHERE id = $1::text::uuid AND user_id = $2::text::uuid
				AND revoked_at IS NULL
			RETURNING id::text`,
			string(id), string(userID)).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return mapError(err)
		}
		revoked = true
		if _, err := tx.Exec(ctx, `
			UPDATE refresh_token SET revoked_at = now()
			WHERE session_id = $1::text::uuid AND revoked_at IS NULL`,
			string(id)); err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
	return revoked, err
}

// RevokeAllForUser is the admin and incident response primitive. Password
// reset uses the same transaction helper inside its own repository method.
func (r *PgSessionRepository) RevokeAllForUser(ctx context.Context, userID domain.UserID, audit *domain.AuditEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := revokeSessionsTx(ctx, tx, userID, "", time.Now().UTC()); err != nil {
			return err
		}
		return insertAuditTx(ctx, tx, audit)
	})
}

// revokeSessionsTx is the shared write behind password reset and password
// change. It revokes every live session except keep, and their refresh tokens.
func revokeSessionsTx(ctx context.Context, tx pgx.Tx, userID domain.UserID, keep domain.SessionID, at time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE session SET revoked_at = $2
		WHERE user_id = $1::text::uuid AND revoked_at IS NULL
			AND (NULLIF($3, '') IS NULL OR id <> NULLIF($3, '')::uuid)`,
		string(userID), at, string(keep)); err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_token SET revoked_at = $2
		WHERE revoked_at IS NULL AND session_id IN (
			SELECT id FROM session
			WHERE user_id = $1::text::uuid
				AND (NULLIF($3, '') IS NULL OR id <> NULLIF($3, '')::uuid)
		)`,
		string(userID), at, string(keep)); err != nil {
		return mapError(err)
	}
	return nil
}

func scanSession(row rowScanner) (*domain.Session, error) {
	var session domain.Session
	err := row.Scan(
		&session.ID, &session.RealmID, &session.UserID, &session.TokenHash,
		&session.CSRFTokenHash, &session.IP, &session.UserAgent, &session.AMR,
		&session.AuthTime, &session.LastSeenAt, &session.RememberMe,
		&session.IdleExpiresAt, &session.AbsoluteExpiresAt, &session.RevokedAt,
		&session.CreatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &session, nil
}

// RevokeByID is the RP-initiated logout primitive. It revokes the session and
// its refresh tokens, and records the audit event in the same transaction.
func (r *PgSessionRepository) RevokeByID(ctx context.Context, id domain.SessionID, audit *domain.AuditEvent) (bool, error) {
	revoked := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var found domain.SessionID
		err := tx.QueryRow(ctx, `
			UPDATE session SET revoked_at = now()
			WHERE id = $1::text::uuid AND revoked_at IS NULL
			RETURNING id::text`, string(id)).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return mapError(err)
		}
		revoked = true
		if _, err := tx.Exec(ctx, `
			UPDATE refresh_token SET revoked_at = now()
			WHERE session_id = $1::text::uuid AND revoked_at IS NULL`,
			string(id)); err != nil {
			return mapError(err)
		}
		return insertAuditTx(ctx, tx, audit)
	})
	return revoked, err
}
