package domain

import (
	"context"
	"time"
)

// CredentialRepository owns password material and its Postgres lockout state.
type CredentialRepository interface {
	Upsert(ctx context.Context, credential *Credential) error
	GetByUserID(ctx context.Context, userID UserID) (*Credential, error)
	RecordFailedLogin(ctx context.Context, userID UserID, now time.Time, threshold int, lockWindow time.Duration) (int, *time.Time, error)
	ChangePassword(ctx context.Context, userID UserID, keepSessionID SessionID, passwordHash string, params []byte, audit *AuditEvent) error
}
