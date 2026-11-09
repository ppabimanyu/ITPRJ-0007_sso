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
	authCodeKeyPrefix = "authcode:"
	authCodeTTL       = 60 * time.Second
)

// RedisAuthCodeRepository stores hashed, single-use authorization codes.
// Take uses GETDEL so exactly one caller can exchange a code.
type RedisAuthCodeRepository struct {
	client *redis.Client
}

func NewAuthCodeRepository(client *redis.Client) *RedisAuthCodeRepository {
	return &RedisAuthCodeRepository{client: client}
}

func (r *RedisAuthCodeRepository) Put(ctx context.Context, codeHash string, code *domain.AuthCode) error {
	raw, err := json.Marshal(code)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, authCodeKeyPrefix+codeHash, raw, authCodeTTL).Err()
}

func (r *RedisAuthCodeRepository) Take(ctx context.Context, codeHash string) (*domain.AuthCode, error) {
	raw, err := r.client.GetDel(ctx, authCodeKeyPrefix+codeHash).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var code domain.AuthCode
	if err := json.Unmarshal(raw, &code); err != nil {
		return nil, err
	}
	return &code, nil
}
