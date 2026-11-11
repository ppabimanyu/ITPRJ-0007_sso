package domain

import (
	"encoding/json"
	"time"
)

// AuditEventView is the admin audit row. Metadata never contains secrets.
type AuditEventView struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	ActorType  string          `json:"actor_type"`
	ActorID    *string         `json:"actor_id"`
	ActorEmail *string         `json:"actor_email"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

// AuditList is the keyset-paginated audit page.
type AuditList struct {
	Items     []AuditEventView `json:"items"`
	NextAfter *string          `json:"next_after"`
}

// AuditFilter drives the admin audit query.
type AuditFilter struct {
	From    *time.Time
	To      *time.Time
	Type    string
	ActorID string
	After   string
	Limit   int
}
