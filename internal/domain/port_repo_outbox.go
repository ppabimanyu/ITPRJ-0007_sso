package domain

import (
	"context"
	"time"
)

// OutboxRepository is the transactional outbox used by workers.
type OutboxRepository interface {
	Enqueue(ctx context.Context, event *OutboxEvent) error
	ListPending(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkSent(ctx context.Context, id OutboxEventID) error
	MarkFailed(ctx context.Context, id OutboxEventID, lastError string, nextAttemptAt time.Time) error
}
