package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgAdminAuditRepository reads the filtered audit stream.
type PgAdminAuditRepository struct {
	pool *pgxpool.Pool
}

func NewAdminAuditRepository(pool *pgxpool.Pool) *PgAdminAuditRepository {
	return &PgAdminAuditRepository{pool: pool}
}

func (r *PgAdminAuditRepository) ListFiltered(ctx context.Context, realmID domain.RealmID, filter domain.AuditFilter) ([]domain.AuditEventView, string, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{string(realmID)}
	clauses := []string{"realm_id = $1::text::uuid"}
	if filter.From != nil {
		args = append(args, *filter.From)
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d::timestamptz", len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d::timestamptz", len(args)))
	}
	if filter.Type != "" {
		args = append(args, filter.Type)
		clauses = append(clauses, fmt.Sprintf("type = $%d", len(args)))
	}
	if filter.ActorID != "" {
		args = append(args, filter.ActorID)
		clauses = append(clauses, fmt.Sprintf("actor_id = $%d", len(args)))
	}
	if filter.After != "" {
		createdAt, id, err := decodeAdminCursor(filter.After)
		if err != nil {
			return nil, "", domain.ErrNotFound
		}
		args = append(args, createdAt, id)
		index := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(created_at, id) < ($%d::timestamptz, $%d::text::uuid)", index-1, index))
	}
	args = append(args, limit+1)
	query := `SELECT id::text, type, actor_type, actor_id, metadata, created_at
		FROM audit_event WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY created_at DESC, id DESC LIMIT $` + fmt.Sprint(len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", mapError(err)
	}
	defer rows.Close()

	views := make([]domain.AuditEventView, 0, limit)
	for rows.Next() {
		var view domain.AuditEventView
		if err := rows.Scan(&view.ID, &view.Type, &view.ActorType, &view.ActorID,
			&view.Metadata, &view.CreatedAt); err != nil {
			return nil, "", mapError(err)
		}
		if len(view.Metadata) == 0 {
			view.Metadata = []byte("{}")
		}
		views = append(views, view)
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

var _ domain.AdminAuditRepository = (*PgAdminAuditRepository)(nil)
