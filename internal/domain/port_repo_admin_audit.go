package domain

import "context"

// AdminAuditRepository reads the filtered audit stream.
type AdminAuditRepository interface {
	ListFiltered(ctx context.Context, realmID RealmID, filter AuditFilter) ([]AuditEventView, string, error)
}
