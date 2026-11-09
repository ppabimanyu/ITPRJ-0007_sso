package domain

import "time"

// EmailToken backs verification, reset, and invite flows.
type EmailToken struct {
	ID        string
	UserID    UserID
	Purpose   string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedBy *UserID
	CreatedAt time.Time
}
