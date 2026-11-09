package domain

import (
	"context"
	"time"
)

// OutboxRepository claims rows with FOR UPDATE SKIP LOCKED. Claim is the only
// method that moves a row into processing; a stale locked_until puts it back in
// reach after a crash.
type OutboxRepository interface {
	Enqueue(ctx context.Context, event *OutboxEvent) error
	Claim(ctx context.Context, limit int, lockFor time.Duration) ([]OutboxEvent, error)
	MarkSent(ctx context.Context, id OutboxEventID) error
	MarkRetry(ctx context.Context, id OutboxEventID, lastError string, nextAttemptAt time.Time) error
	MarkFailed(ctx context.Context, id OutboxEventID, lastError string) error
}
