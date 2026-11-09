package domain

import "context"

// RealmRepository owns realm rows.
type RealmRepository interface {
	Create(ctx context.Context, realm *Realm) error
	GetByID(ctx context.Context, id RealmID) (*Realm, error)
	GetByName(ctx context.Context, name string) (*Realm, error)
}
