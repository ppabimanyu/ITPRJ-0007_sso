package domain

import "context"

// AdminSessionRepository owns the console session list and realm-scoped revoke.
type AdminSessionRepository interface {
	ListAdmin(ctx context.Context, realmID RealmID, filter AdminSessionFilter) ([]AdminSessionView, string, error)
	RevokeForRealm(ctx context.Context, realmID RealmID, id SessionID, audit *AuditEvent) (bool, error)
}
