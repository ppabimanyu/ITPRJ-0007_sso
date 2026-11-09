package domain

import "time"

// Consent records the scopes a user granted to a client.
type Consent struct {
	UserID    UserID
	ClientID  ClientID
	Scope     string
	GrantedAt time.Time
}
