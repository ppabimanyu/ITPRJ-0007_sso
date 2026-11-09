package domain

import "context"

// AuditRepository appends and queries audit events.
type AuditRepository interface {
	Insert(ctx context.Context, event *AuditEvent) error
	ListByRealm(ctx context.Context, realmID RealmID, limit, offset int) ([]AuditEvent, error)
}
