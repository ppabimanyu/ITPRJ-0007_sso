package domain

import "time"

// Session is a server-side login session.
type Session struct {
	ID                SessionID
	RealmID           RealmID
	UserID            UserID
	IP                string
	UserAgent         string
	AMR               []string
	AuthTime          time.Time
	LastSeenAt        time.Time
	RememberMe        bool
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	CreatedAt         time.Time
}

// RefreshToken is an opaque token stored only as a SHA-256 hash.
type RefreshToken struct {
	ID                string
	SessionID         SessionID
	TokenHash         string
	FamilyID          string
	ParentID          *string
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	UsedAt            *time.Time
	ReplacedBy        *string
	RevokedAt         *time.Time
	CreatedAt         time.Time
}
