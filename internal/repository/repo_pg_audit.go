package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgAuditRepository appends audit events.
type PgAuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *PgAuditRepository {
	return &PgAuditRepository{pool: pool}
}

func (r *PgAuditRepository) Insert(ctx context.Context, event *domain.AuditEvent) error {
	return insertAudit(ctx, r.pool, event)
}

// DeleteBefore removes audit events older than the retention cutoff.
func (r *PgAuditRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM audit_event WHERE created_at < $1`, before)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// insertAuditTx writes the event inside a business transaction so the evidence
// and the state change commit together.
func insertAuditTx(ctx context.Context, tx pgx.Tx, event *domain.AuditEvent) error {
	if event == nil {
		return nil
	}
	return insertAudit(ctx, tx, event)
}

type execQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertAudit(ctx context.Context, q execQuerier, event *domain.AuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := q.Exec(ctx, `
		INSERT INTO audit_event (id, realm_id, actor_type, actor_id, type, metadata, created_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6::jsonb, $7)`,
		string(event.ID), string(event.RealmID), event.ActorType,
		nullString(event.ActorID), event.Type, jsonValue(event.Metadata), event.CreatedAt)
	return mapError(err)
}

func (r *PgAuditRepository) ListByRealm(ctx context.Context, realmID domain.RealmID, limit, offset int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, realm_id::text, actor_type, actor_id, type, metadata, created_at
		FROM audit_event WHERE realm_id = $1::text::uuid
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		string(realmID), limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		var event domain.AuditEvent
		if err := rows.Scan(&event.ID, &event.RealmID, &event.ActorType,
			&event.ActorID, &event.Type, &event.Metadata, &event.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		events = append(events, event)
	}
	return events, mapError(rows.Err())
}

var _ domain.AuditRetentionRepository = (*PgAuditRepository)(nil)
