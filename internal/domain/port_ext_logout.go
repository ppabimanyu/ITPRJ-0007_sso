package domain

import (
	"context"
	"time"
)

// TokenIssuer signs and verifies the realm's JWTs and publishes its JWKS.
type TokenIssuer interface {
	SignAccessToken(ctx context.Context, realmID RealmID, claims AccessTokenClaims) (string, error)
	SignIDToken(ctx context.Context, realmID RealmID, claims IDTokenClaims) (string, error)
	SignLogoutToken(ctx context.Context, realmID RealmID, token LogoutToken) (string, error)
	VerifyAccessToken(ctx context.Context, realmID RealmID, raw string) (*AccessTokenClaims, error)
	VerifyIDToken(ctx context.Context, realmID RealmID, raw string) (*IDTokenClaims, error)
	VerifyLogoutToken(ctx context.Context, realmID RealmID, raw string) (*LogoutToken, error)
	JWKS(ctx context.Context, realmID RealmID) (*JSONWebKeySet, error)
}

// KeyRotator creates and retires realm signing keys. It is implemented by the
// signing key repository and used by the migration CLI.
type KeyRotator interface {
	RotateKey(ctx context.Context, realmID RealmID, retireAfter time.Duration) (*RealmSigningKey, error)
}

// LogoutTokenSigner is the worker-facing subset needed for backchannel logout.
type LogoutTokenSigner interface {
	SignLogoutToken(ctx context.Context, realmID RealmID, token LogoutToken) (string, error)
}

// LogoutSender posts a signed logout token to a registered backchannel URI.
type LogoutSender interface {
	Send(ctx context.Context, uri, token string) error
}
