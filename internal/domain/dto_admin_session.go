package domain

import "time"

// AdminSessionView is the console shape for a login session.
type AdminSessionView struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	UserEmail  string     `json:"user_email"`
	IP         *string    `json:"ip"`
	UserAgent  *string    `json:"user_agent"`
	AMR        []string   `json:"amr"`
	ACR        string     `json:"acr"`
	AuthTime   time.Time  `json:"auth_time"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// AdminSessionList is the keyset-paginated admin session page.
type AdminSessionList struct {
	Items     []AdminSessionView `json:"items"`
	NextAfter *string            `json:"next_after"`
}

// AdminSessionFilter drives the admin session query.
type AdminSessionFilter struct {
	UserID string
	Status string
	After  string
	Limit  int
}
