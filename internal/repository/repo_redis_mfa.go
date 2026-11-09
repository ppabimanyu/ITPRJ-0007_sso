package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/caching"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

// RedisMFAChallengeRepository stores one challenge per opaque token. Only the
// token digest keys the row, so a Redis dump does not expose usable tokens.
type RedisMFAChallengeRepository struct {
	client *redis.Client
}

func NewMFAChallengeRepository(client *redis.Client) *RedisMFAChallengeRepository {
	return &RedisMFAChallengeRepository{client: client}
}

func (r *RedisMFAChallengeRepository) Create(ctx context.Context, token string, challenge *domain.MFAChallenge) error {
	raw, err := json.Marshal(challenge)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, mfaChallengeKey(token), raw, caching.MFATTL).Err()
}

func (r *RedisMFAChallengeRepository) Get(ctx context.Context, token string) (*domain.MFAChallenge, error) {
	key := mfaChallengeKey(token)
	raw, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var challenge domain.MFAChallenge
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return nil, err
	}
	if !challenge.ExpiresAt.After(time.Now().UTC()) {
		_ = r.client.Del(ctx, key).Err()
		return nil, domain.ErrNotFound
	}
	return &challenge, nil
}

// RecordFailure increments the attempt counter atomically and preserves the
// original expiry. The caller deletes the challenge at the threshold.
func (r *RedisMFAChallengeRepository) RecordFailure(ctx context.Context, token string) (int, error) {
	key := mfaChallengeKey(token)
	attempts := 0
	for attempt := 0; attempt < 3; attempt++ {
		err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			raw, err := tx.Get(ctx, key).Bytes()
			if errors.Is(err, redis.Nil) {
				return domain.ErrNotFound
			}
			if err != nil {
				return err
			}
			var challenge domain.MFAChallenge
			if err := json.Unmarshal(raw, &challenge); err != nil {
				return err
			}
			ttl := time.Until(challenge.ExpiresAt)
			if ttl <= 0 {
				return domain.ErrNotFound
			}
			challenge.Attempts++
			attempts = challenge.Attempts
			updated, err := json.Marshal(challenge)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, updated, ttl)
				return nil
			})
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		return attempts, err
	}
	return 0, errors.New("mfa challenge update contention")
}

func (r *RedisMFAChallengeRepository) Delete(ctx context.Context, token string) error {
	return r.client.Del(ctx, mfaChallengeKey(token)).Err()
}

func mfaChallengeKey(token string) string {
	return caching.MFAKey(pkg.SHA256Hex(token))
}
