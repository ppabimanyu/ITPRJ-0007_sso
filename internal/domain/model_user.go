package domain

import (
	"encoding/json"
	"time"
)

// User is an identity inside one realm. Email is unique case-insensitively.
type User struct {
	ID              UserID
	RealmID         RealmID
	Email           string
	Username        *string
	Name            string
	GivenName       string
	FamilyName      string
	Locale          *string
	EmailVerifiedAt *time.Time
	DisabledAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Credential is the password material for a user. FailedAttempts and
// LockedUntil survive a Redis flush, so lockout cannot be reset by clearing
// the cache.
type Credential struct {
	UserID         UserID
	PasswordHash   string
	Algorithm      string
	Params         json.RawMessage
	FailedAttempts int
	LockedUntil    *time.Time
	UpdatedAt      time.Time
}
