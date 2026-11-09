package domain

import "context"

// SigningKeyRepository owns per-realm RSA material.
type SigningKeyRepository interface {
	Create(ctx context.Context, key *RealmSigningKey) error
	ListActive(ctx context.Context, realmID RealmID) ([]RealmSigningKey, error)
}
