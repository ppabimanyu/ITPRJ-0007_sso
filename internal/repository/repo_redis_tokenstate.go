package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

const (
	revokedJTIPrefix     = "revoked_jti:"
	revokedSessionPrefix = "revoked_sid:"
)

// RedisTokenStateRepository answers whether a stateless access token or an
// entire session has been revoked. Keys expire no later than the token they
// invalidate, so the store cannot grow without bound.
type RedisTokenStateRepository struct {
	client *redis.Client
}

func NewTokenStateRepository(client *redis.Client) *RedisTokenStateRepository {
	return &RedisTokenStateRepository{client: client}
}

func (r *RedisTokenStateRepository) RevokeJTI(ctx context.Context, jti string, untilUnix int64) error {
	ttl := time.Until(time.Unix(untilUnix, 0))
	if ttl <= 0 {
		return nil
	}
	return r.client.Set(ctx, revokedJTIPrefix+jti, "1", ttl).Err()
}

func (r *RedisTokenStateRepository) JTIRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := r.client.Exists(ctx, revokedJTIPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *RedisTokenStateRepository) RevokeSession(ctx context.Context, id domain.SessionID, ttlSeconds int64) error {
	if ttlSeconds <= 0 {
		return nil
	}
	return r.client.Set(ctx, revokedSessionPrefix+string(id), "1",
		time.Duration(ttlSeconds)*time.Second).Err()
}

func (r *RedisTokenStateRepository) SessionRevoked(ctx context.Context, id domain.SessionID) (bool, error) {
	n, err := r.client.Exists(ctx, revokedSessionPrefix+string(id)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
