// Package caching owns the Redis client and the service key namespace.
package caching

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// New parses REDIS_URL and returns a client. The caller pings before serving.
func New(rawURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return redis.NewClient(opts), nil
}

// Ping verifies Redis connectivity within the caller's deadline.
func Ping(ctx context.Context, client *redis.Client) error {
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// Key helpers keep the ARCHITECTURE §7 namespace in one place.
func AuthRequestKey(id string) string { return "authreq:" + id }
func AuthCodeKey(hash string) string  { return "authcode:" + hash }
func MFAKey(token string) string      { return "mfa:" + token }
func SessionKey(hash string) string   { return "sess:" + hash }
func JWKSKey(realm string) string     { return "jwks:" + realm }

// RateLimitKey scopes a counter, for example rl:login:203.0.113.7.
func RateLimitKey(scope, key string) string { return "rl:" + scope + ":" + key }

// TTLs from ARCHITECTURE §7.
const (
	AuthRequestTTL = 10 * time.Minute
	AuthCodeTTL    = 60 * time.Second
	MFATTL         = 5 * time.Minute
	SessionTTL     = 30 * time.Second
	JWKSTTL        = 5 * time.Minute
)
