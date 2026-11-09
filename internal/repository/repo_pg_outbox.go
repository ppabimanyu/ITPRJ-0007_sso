package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// PgOutboxRepository is the transactional outbox.
type PgOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *PgOutboxRepository {
	return &PgOutboxRepository{pool: pool}
}

func (r *PgOutboxRepository) Enqueue(ctx context.Context, event *domain.OutboxEvent) error {
	if event.Status == "" {
		event.Status = domain.OutboxStatusPending
	}
	if event.NextAttemptAt.IsZero() {
		event.NextAttemptAt = time.Now()
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO outbox (id, realm_id, type, payload, status, attempts, next_attempt_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4::jsonb, $5, $6, $7)
		RETURNING created_at`,
		string(event.ID), string(event.RealmID), event.Type, jsonValue(event.Payload),
		event.Status, event.Attempts, event.NextAttemptAt,
	).Scan(&event.CreatedAt)
	return mapError(err)
}

func (r *PgOutboxRepository) ListPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, realm_id::text, type, payload, status, attempts,
			next_attempt_at, last_error, created_at, processed_at
		FROM outbox
		WHERE status = 'pending' AND next_attempt_at <= now()
		ORDER BY next_attempt_at LIMIT $1`, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var events []domain.OutboxEvent
	for rows.Next() {
		var event domain.OutboxEvent
		if err := rows.Scan(&event.ID, &event.RealmID, &event.Type, &event.Payload,
			&event.Status, &event.Attempts, &event.NextAttemptAt, &event.LastError,
			&event.CreatedAt, &event.ProcessedAt); err != nil {
			return nil, mapError(err)
		}
		events = append(events, event)
	}
	return events, mapError(rows.Err())
}

func (r *PgOutboxRepository) MarkSent(ctx context.Context, id domain.OutboxEventID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox SET status = 'sent', processed_at = now(), last_error = NULL
		WHERE id = $1::text::uuid`, string(id))
	return mapError(err)
}

func (r *PgOutboxRepository) MarkFailed(ctx context.Context, id domain.OutboxEventID, lastError string, nextAttemptAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox
		SET status = 'pending', attempts = attempts + 1, last_error = $2, next_attempt_at = $3
		WHERE id = $1::text::uuid`, string(id), lastError, nextAttemptAt)
	return mapError(err)
}
