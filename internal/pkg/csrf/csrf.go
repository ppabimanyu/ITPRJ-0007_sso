// Package csrf issues and verifies the double-submit tokens used by the auth
// endpoints. Only SHA-256 digests are stored.
package csrf

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

// PreSessionTTL is how long a pre-login token stays valid in Redis.
const PreSessionTTL = 15 * time.Minute

// Store keeps pre-session token digests. The Redis implementation lives in
// internal/repository/repo_redis_csrf.go.
type Store interface {
	Put(ctx context.Context, tokenHash string, ttl time.Duration) error
	Exists(ctx context.Context, tokenHash string) (bool, error)
	Delete(ctx context.Context, tokenHash string) error
}

// NewToken returns 32 random bytes as base64url plus its storage digest.
func NewToken() (raw, hash string, err error) {
	raw, err = pkg.RandomToken(32)
	if err != nil {
		return "", "", err
	}
	return raw, Hash(raw), nil
}

// Hash is the SHA-256 hex digest stored in Redis or on the session row.
func Hash(raw string) string {
	return pkg.SHA256Hex(raw)
}

// Issue stores a fresh pre-session token and returns the raw value.
func Issue(ctx context.Context, store Store, ttl time.Duration) (string, error) {
	if store == nil {
		return "", errors.New("csrf store is not configured")
	}
	raw, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	if err := store.Put(ctx, hash, ttl); err != nil {
		return "", err
	}
	return raw, nil
}

// Verify reports whether the raw token has a live Redis digest.
func Verify(ctx context.Context, store Store, raw string) (bool, error) {
	if store == nil || raw == "" {
		return false, nil
	}
	return store.Exists(ctx, Hash(raw))
}

// Consume deletes the pre-session digest after a successful login.
func Consume(ctx context.Context, store Store, raw string) error {
	if store == nil || raw == "" {
		return nil
	}
	return store.Delete(ctx, Hash(raw))
}

// Equal compares two tokens without an early exit on the first byte.
func Equal(a, b string) bool {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// MatchesHash compares a raw session token with the digest stored on the row.
func MatchesHash(raw, hash string) bool {
	if raw == "" || hash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(Hash(raw)), []byte(hash)) == 1
}
