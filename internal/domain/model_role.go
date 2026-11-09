package domain

import "time"

// Role is a realm-scoped authorization label.
type Role struct {
	ID          RoleID
	RealmID     RealmID
	Name        string
	Description string
	CreatedAt   time.Time
}

// Group is a realm-scoped collection of users with no role inheritance.
type Group struct {
	ID          GroupID
	RealmID     RealmID
	Name        string
	Description string
	CreatedAt   time.Time
}
