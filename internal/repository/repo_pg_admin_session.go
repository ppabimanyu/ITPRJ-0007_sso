package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

// PgAdminSessionRepository reads and revokes sessions across a realm.
type PgAdminSessionRepository struct {
	pool *pgxpool.Pool
}

func NewAdminSessionRepository(pool *pgxpool.Pool) *PgAdminSessionRepository {
	return &PgAdminSessionRepository{pool: pool}
}

func (r *PgAdminSessionRepository) ListAdmin(ctx context.Context, realmID domain.RealmID, filter domain.AdminSessionFilter) ([]domain.AdminSessionView, string, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{string(realmID)}
	clauses := []string{"s.realm_id = $1::text::uuid"}
	if filter.UserID != "" {
		args = append(args, filter.UserID)
		clauses = append(clauses, fmt.Sprintf("s.user_id = $%d::text::uuid", len(args)))
	}
	switch filter.Status {
	case "":
	case "active":
		clauses = append(clauses, "s.revoked_at IS NULL AND s.idle_expires_at > now() AND s.absolute_expires_at > now()")
	case "revoked":
		clauses = append(clauses, "s.revoked_at IS NOT NULL")
	case "expired":
		clauses = append(clauses, "s.revoked_at IS NULL AND (s.idle_expires_at <= now() OR s.absolute_expires_at <= now())")
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
			"(s.created_at, s.id) < ($%d::timestamptz, $%d::text::uuid)", index-1, index))
	}
	args = append(args, limit+1)
	query := `SELECT s.id::text, s.user_id::text, u.email,
		coalesce(host(s.ip), ''), s.user_agent, s.amr, coalesce(s.acr, ''),
		s.auth_time, s.created_at, s.last_seen_at, s.absolute_expires_at,
		s.revoked_at
		FROM session s JOIN "user" u ON u.id = s.user_id
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY s.created_at DESC, s.id DESC LIMIT $` + fmt.Sprint(len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", mapError(err)
	}
	defer rows.Close()

	views := make([]domain.AdminSessionView, 0, limit)
	for rows.Next() {
		view, err := scanAdminSessionView(rows)
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

func scanAdminSessionView(row rowScanner) (*domain.AdminSessionView, error) {
	var view domain.AdminSessionView
	var ip, userAgent string
	if err := row.Scan(&view.ID, &view.UserID, &view.UserEmail, &ip, &userAgent,
		&view.AMR, &view.ACR, &view.AuthTime, &view.CreatedAt, &view.LastSeenAt,
		&view.ExpiresAt, &view.RevokedAt); err != nil {
		return nil, mapError(err)
	}
	view.IP = optionalStringValue(ip)
	view.UserAgent = optionalStringValue(userAgent)
	if view.AMR == nil {
		view.AMR = []string{}
	}
	return &view, nil
}

func (r *PgAdminSessionRepository) RevokeForRealm(ctx context.Context, realmID domain.RealmID, id domain.SessionID, audit *domain.AuditEvent) (bool, error) {
	revoked := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var found string
		err := tx.QueryRow(ctx, `
			UPDATE session SET revoked_at = now()
			WHERE realm_id = $1::text::uuid AND id = $2::text::uuid
				AND revoked_at IS NULL
			RETURNING id::text`, string(realmID), string(id)).Scan(&found)
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

var _ domain.AdminSessionRepository = (*PgAdminSessionRepository)(nil)
