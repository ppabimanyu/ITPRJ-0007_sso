package domain

import "time"

// Session is a server-side login session. TokenHash and CSRFTokenHash are
// SHA-256 hex digests; the raw values live only in cookies.
type Session struct {
	ID                SessionID
	RealmID           RealmID
	UserID            UserID
	TokenHash         string
	CSRFTokenHash     string
	IP                string
	UserAgent         string
	AMR               []string
	ACR               string
	AuthTime          time.Time
	LastSeenAt        time.Time
	RememberMe        bool
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	CreatedAt         time.Time
}

// SessionWithUser is the authenticated view of one session: the session row,
// its user, the user's roles, and whether MFA is enrolled.
type SessionWithUser struct {
	Session     *Session
	User        *User
	Roles       []Role
	MFAEnrolled bool
}

// RefreshToken is an opaque token stored only as a SHA-256 hash.
type RefreshToken struct {
	ID                string
	SessionID         SessionID
	ClientID          ClientID
	TokenHash         string
	FamilyID          string
	ParentID          *string
	Scope             string
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	UsedAt            *time.Time
	ReplacedBy        *string
	RevokedAt         *time.Time
	CreatedAt         time.Time
}
