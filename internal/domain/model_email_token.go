package domain

import "time"

// EmailTokenPurpose is the closed set of reasons an email token exists.
type EmailTokenPurpose string

// EmailToken backs verification, reset, and invite flows.
type EmailToken struct {
	ID        string
	UserID    UserID
	Purpose   EmailTokenPurpose
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedBy *UserID
	CreatedAt time.Time
}
