package domain

import "context"

// RoleRepository owns realm roles and user assignments.
type RoleRepository interface {
	Create(ctx context.Context, role *Role) error
	GetByName(ctx context.Context, realmID RealmID, name string) (*Role, error)
	ListByRealm(ctx context.Context, realmID RealmID) ([]Role, error)
	AssignToUser(ctx context.Context, realmID RealmID, userID UserID, roleID RoleID) error
	ListByUser(ctx context.Context, realmID RealmID, userID UserID) ([]Role, error)
}
