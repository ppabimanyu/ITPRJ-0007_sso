package domain

import "context"

// UserRepository owns user rows.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id UserID) (*User, error)
	GetByEmail(ctx context.Context, realmID RealmID, email string) (*User, error)
	ListByRealm(ctx context.Context, realmID RealmID, limit, offset int) ([]User, error)
}
