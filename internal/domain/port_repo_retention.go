package domain

import (
	"context"
	"time"
)

// RetentionRepository removes expired operational rows.
type RetentionRepository interface {
	DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error)
	DeleteExpiredRefreshTokens(ctx context.Context, before time.Time) (int64, error)
	DeleteExpiredEmailTokens(ctx context.Context, before time.Time) (int64, error)
	DeleteFailedOutbox(ctx context.Context, before time.Time) (int64, error)
	DeleteExpiredIdempotencyKeys(ctx context.Context, before time.Time) (int64, error)
}
