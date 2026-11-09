package domain

import "context"

// AdminRoleRepository writes roles with their audit event in one transaction.
type AdminRoleRepository interface {
	CreateWithAudit(ctx context.Context, role *Role, audit *AuditEvent) error
}
