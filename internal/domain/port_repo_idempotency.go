package domain

import "context"

// IdempotencyRepository stores a replayable response keyed by realm, endpoint,
// and the client's Idempotency-Key. It lives in Postgres, not Redis.
type IdempotencyRepository interface {
	Get(ctx context.Context, realmID RealmID, endpoint, key string) (*IdempotencyKey, error)
	Save(ctx context.Context, record *IdempotencyKey) error
}
