package domain

import "context"

// AdminUserRepository owns the multi-table admin user operations. Last-admin
// protection is enforced inside the same transaction as the change.
type AdminUserRepository interface {
	List(ctx context.Context, realmID RealmID, filter AdminUserFilter) ([]AdminUserView, string, error)
	Get(ctx context.Context, realmID RealmID, userID UserID) (*AdminUserView, error)
	CreateWithInvite(ctx context.Context, user *User, token *EmailToken, outbox *OutboxEvent, audit *AuditEvent) error
	Update(ctx context.Context, realmID RealmID, userID UserID, update AdminUserUpdate, audit *AuditEvent) (*User, error)
	ReplaceRoles(ctx context.Context, realmID RealmID, userID UserID, roleIDs []RoleID, audit *AuditEvent) ([]Role, error)
	EnqueueInvite(ctx context.Context, token *EmailToken, outbox *OutboxEvent, audit *AuditEvent) error
}
