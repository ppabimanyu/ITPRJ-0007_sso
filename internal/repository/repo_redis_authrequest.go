package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

const (
	authRequestKeyPrefix = "authreq:"
	authRequestTTL       = 10 * time.Minute
)

// RedisAuthRequestRepository stores one JSON document per authorize request.
// A missing Redis fails closed, so the login cannot be completed.
type RedisAuthRequestRepository struct {
	client *redis.Client
}

func NewAuthRequestRepository(client *redis.Client) *RedisAuthRequestRepository {
	return &RedisAuthRequestRepository{client: client}
}

func (r *RedisAuthRequestRepository) Put(ctx context.Context, request *domain.AuthRequest) error {
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, authRequestKeyPrefix+request.ID, raw, authRequestTTL).Err()
}

func (r *RedisAuthRequestRepository) Get(ctx context.Context, id string) (*domain.AuthRequest, error) {
	raw, err := r.client.Get(ctx, authRequestKeyPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var request domain.AuthRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	return &request, nil
}

func (r *RedisAuthRequestRepository) Delete(ctx context.Context, id string) error {
	return r.client.Del(ctx, authRequestKeyPrefix+id).Err()
}
