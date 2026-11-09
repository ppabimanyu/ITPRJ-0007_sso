package domain

import (
	"context"
	"time"
)

// RateLimitDecision is the answer from one fixed-window counter.
type RateLimitDecision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

// RateLimitRepository owns the Redis counters that guard login and email
// endpoints. An error is reported to the handler, which fails closed.
type RateLimitRepository interface {
	Allow(ctx context.Context, scope, key string, limit int, window time.Duration) (RateLimitDecision, error)
}
