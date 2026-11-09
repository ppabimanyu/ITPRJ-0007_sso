package domain

import "context"

// AuthRequestRepository stores the short-lived authorize request in Redis.
type AuthRequestRepository interface {
	Put(ctx context.Context, request *AuthRequest) error
	Get(ctx context.Context, id string) (*AuthRequest, error)
	Delete(ctx context.Context, id string) error
}

// AuthCodeRepository stores single-use authorization codes in Redis.
type AuthCodeRepository interface {
	Put(ctx context.Context, codeHash string, code *AuthCode) error
	Take(ctx context.Context, codeHash string) (*AuthCode, error)
}

// TokenStateRepository records stateless-token revocations for introspection.
type TokenStateRepository interface {
	RevokeJTI(ctx context.Context, jti string, untilTime int64) error
	JTIRevoked(ctx context.Context, jti string) (bool, error)
	RevokeSession(ctx context.Context, id SessionID, ttlSeconds int64) error
	SessionRevoked(ctx context.Context, id SessionID) (bool, error)
}
