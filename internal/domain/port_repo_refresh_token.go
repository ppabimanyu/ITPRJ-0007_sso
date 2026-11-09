package domain

import (
	"context"
	"time"
)

// RefreshTokenRepository owns opaque refresh tokens. Rotate is one transaction:
// it marks the old row used, inserts the replacement, and on reuse revokes the
// whole family before committing.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Rotate(ctx context.Context, oldHash, clientID, newHash, newID string, now time.Time, idleTTL time.Duration, audit *AuditEvent) (*RefreshRotation, error)
	RevokeFamily(ctx context.Context, familyID string, audit *AuditEvent) error
	Revoke(ctx context.Context, hash, clientID string, audit *AuditEvent) (*RefreshToken, error)
}

// RefreshRotation reports what Rotate did. Reused is true when the presented
// token was already used; the family is revoked and the caller returns
// invalid_grant.
type RefreshRotation struct {
	Token  *RefreshToken
	Reused bool
}
