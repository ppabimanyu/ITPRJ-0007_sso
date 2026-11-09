package domain

import (
	"context"
	"time"
)

// SigningKeyRepository owns per-realm RSA material. ListPublished returns the
// active key plus every retired key whose retire_at is still in the future, so
// tokens signed before a rotation keep verifying.
type SigningKeyRepository interface {
	Create(ctx context.Context, key *RealmSigningKey) error
	GetActive(ctx context.Context, realmID RealmID) (*RealmSigningKey, error)
	ListActive(ctx context.Context, realmID RealmID) ([]RealmSigningKey, error)
	ListPublished(ctx context.Context, realmID RealmID) ([]RealmSigningKey, error)
	Rotate(ctx context.Context, realmID RealmID, next *RealmSigningKey, retireAt time.Time) error
}
