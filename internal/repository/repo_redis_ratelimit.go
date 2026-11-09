package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/caching"
)

// RedisRateLimit is a fixed-window counter. The Lua script makes the increment
// and the first expiry atomic across app instances.
type RedisRateLimit struct {
	client *redis.Client
}

func NewRateLimitRepository(client *redis.Client) *RedisRateLimit {
	return &RedisRateLimit{client: client}
}

var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`)

func (r *RedisRateLimit) Allow(ctx context.Context, scope, key string, limit int, window time.Duration) (domain.RateLimitDecision, error) {
	if limit <= 0 {
		return domain.RateLimitDecision{Allowed: true, Limit: limit}, nil
	}
	result, err := rateLimitScript.Run(ctx, r.client,
		[]string{caching.RateLimitKey(scope, key)},
		window.Milliseconds()).Slice()
	if err != nil {
		return domain.RateLimitDecision{}, err
	}
	count, _ := result[0].(int64)
	ttlMillis, _ := result[1].(int64)
	return rateLimitDecision(count, time.Duration(ttlMillis)*time.Millisecond, limit, window), nil
}

// rateLimitDecision is pure so the window rule can be tested without Redis.
func rateLimitDecision(count int64, ttl time.Duration, limit int, window time.Duration) domain.RateLimitDecision {
	if ttl <= 0 {
		ttl = window
	}
	decision := domain.RateLimitDecision{
		Allowed:    count <= int64(limit),
		Limit:      limit,
		RetryAfter: ttl,
	}
	if remaining := int64(limit) - count; remaining > 0 {
		decision.Remaining = int(remaining)
	}
	return decision
}

var _ domain.RateLimitRepository = (*RedisRateLimit)(nil)
