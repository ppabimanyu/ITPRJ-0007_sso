package domain

import (
	"context"
	"time"
)

// SessionRepository owns session rows and the token hashes that identify them.
type SessionRepository interface {
	CreateLoginSession(ctx context.Context, session *Session, audit *AuditEvent) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*SessionWithUser, error)
	GetByID(ctx context.Context, id SessionID) (*SessionWithUser, error)
	TouchLastSeen(ctx context.Context, id SessionID, at, idleExpiresAt time.Time) error
	ListByUser(ctx context.Context, userID UserID) ([]Session, error)
	RevokeForUser(ctx context.Context, id SessionID, userID UserID, audit *AuditEvent) (bool, error)
	RevokeByID(ctx context.Context, id SessionID, audit *AuditEvent) (bool, error)
	RevokeAllForUser(ctx context.Context, userID UserID, audit *AuditEvent) error
}
