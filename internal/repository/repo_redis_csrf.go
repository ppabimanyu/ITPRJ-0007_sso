package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
)

const csrfKeyPrefix = "csrf:"

// RedisCSRFStore keeps pre-session CSRF digests. A missing Redis makes the auth
// endpoints fail closed instead of accepting an unverifiable token.
type RedisCSRFStore struct {
	client *redis.Client
}

func NewCSRFStore(client *redis.Client) *RedisCSRFStore {
	return &RedisCSRFStore{client: client}
}

func (s *RedisCSRFStore) Put(ctx context.Context, tokenHash string, ttl time.Duration) error {
	return s.client.Set(ctx, csrfKeyPrefix+tokenHash, "1", ttl).Err()
}

func (s *RedisCSRFStore) Exists(ctx context.Context, tokenHash string) (bool, error) {
	n, err := s.client.Exists(ctx, csrfKeyPrefix+tokenHash).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *RedisCSRFStore) Delete(ctx context.Context, tokenHash string) error {
	return s.client.Del(ctx, csrfKeyPrefix+tokenHash).Err()
}

var _ csrf.Store = (*RedisCSRFStore)(nil)
