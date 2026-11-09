package domain

import (
	"context"
)

// EmailTokenRepository consumes single-use email tokens. CompletePasswordReset,
// AcceptInvite, and VerifyEmail each change several tables in one transaction.
type EmailTokenRepository interface {
	Create(ctx context.Context, token *EmailToken, outbox *OutboxEvent, audit *AuditEvent) error
	GetValidUser(ctx context.Context, realmID RealmID, tokenHash string, purpose EmailTokenPurpose) (*User, error)
	CreatePasswordReset(ctx context.Context, token *EmailToken, outbox *OutboxEvent, audit *AuditEvent) error
	CompletePasswordReset(ctx context.Context, realmID RealmID, tokenHash, passwordHash string, params []byte, audit *AuditEvent) error
	AcceptInvite(ctx context.Context, realmID RealmID, tokenHash, passwordHash string, params []byte, audit *AuditEvent) error
	VerifyEmail(ctx context.Context, realmID RealmID, tokenHash string, audit *AuditEvent) error
}
