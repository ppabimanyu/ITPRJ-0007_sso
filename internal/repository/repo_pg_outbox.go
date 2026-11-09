package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
)

const outboxColumns = `id::text, realm_id::text, type, payload, status, attempts,
	next_attempt_at, last_error, locked_until, created_at, processed_at`

// PgOutboxRepository is the transactional outbox worker's store.
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
		event.NextAttemptAt = time.Now().UTC()
	}
	return insertOutbox(ctx, r.pool, event)
}

// Claim locks the next batch of due rows. A processing row whose lock expired
// is claimable again, which recovers after a worker crash.
func (r *PgOutboxRepository) Claim(ctx context.Context, limit int, lockFor time.Duration) ([]domain.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if lockFor <= 0 {
		lockFor = time.Minute
	}
	var events []domain.OutboxEvent
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+outboxColumns+`
			FROM outbox
			WHERE (status = 'pending' AND next_attempt_at <= now())
				OR (status = 'processing' AND locked_until IS NOT NULL AND locked_until < now())
			ORDER BY next_attempt_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1`, limit)
		if err != nil {
			return mapError(err)
		}
		for rows.Next() {
			var event domain.OutboxEvent
			if err := rows.Scan(&event.ID, &event.RealmID, &event.Type,
				&event.Payload, &event.Status, &event.Attempts,
				&event.NextAttemptAt, &event.LastError, &event.LockedUntil,
				&event.CreatedAt, &event.ProcessedAt); err != nil {
				rows.Close()
				return mapError(err)
			}
			events = append(events, event)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return mapError(err)
		}
		for i := range events {
			lockUntil := time.Now().UTC().Add(lockFor)
			if _, err := tx.Exec(ctx, `
				UPDATE outbox
				SET status = 'processing', locked_until = $2
				WHERE id = $1::text::uuid`,
				string(events[i].ID), lockUntil); err != nil {
				return mapError(err)
			}
			events[i].Status = domain.OutboxStatusProcessing
			events[i].LockedUntil = &lockUntil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *PgOutboxRepository) MarkSent(ctx context.Context, id domain.OutboxEventID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox
		SET status = 'sent', processed_at = now(), locked_until = NULL,
			last_error = NULL
		WHERE id = $1::text::uuid`, string(id))
	return mapError(err)
}

func (r *PgOutboxRepository) MarkRetry(ctx context.Context, id domain.OutboxEventID, lastError string, nextAttemptAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox
		SET status = 'pending', attempts = attempts + 1, last_error = $2,
			next_attempt_at = $3, locked_until = NULL
		WHERE id = $1::text::uuid`, string(id), lastError, nextAttemptAt)
	return mapError(err)
}

func (r *PgOutboxRepository) MarkFailed(ctx context.Context, id domain.OutboxEventID, lastError string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox
		SET status = 'failed', attempts = attempts + 1, last_error = $2,
			processed_at = now(), locked_until = NULL
		WHERE id = $1::text::uuid`, string(id), lastError)
	return mapError(err)
}

func insertOutboxTx(ctx context.Context, tx pgx.Tx, event *domain.OutboxEvent) error {
	if event == nil {
		return nil
	}
	return insertOutbox(ctx, tx, event)
}

func insertOutbox(ctx context.Context, q execQuerier, event *domain.OutboxEvent) error {
	if event.Status == "" {
		event.Status = domain.OutboxStatusPending
	}
	if event.NextAttemptAt.IsZero() {
		event.NextAttemptAt = time.Now().UTC()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := q.Exec(ctx, `
		INSERT INTO outbox (id, realm_id, type, payload, status, attempts,
			next_attempt_at, created_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4::jsonb, $5, $6, $7, $8)`,
		string(event.ID), string(event.RealmID), event.Type, jsonValue(event.Payload),
		string(event.Status), event.Attempts, event.NextAttemptAt, event.CreatedAt)
	return mapError(err)
}
