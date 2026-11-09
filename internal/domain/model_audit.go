package domain

import (
	"encoding/json"
	"time"
)

// AuditEvent is append-only business evidence. It never stores secrets.
type AuditEvent struct {
	ID        AuditEventID
	RealmID   RealmID
	ActorType string
	ActorID   *string
	Type      string
	Metadata  json.RawMessage
	CreatedAt time.Time
}

// OutboxEvent is a transactional job written with the business change.
type OutboxEvent struct {
	ID            OutboxEventID
	RealmID       RealmID
	Type          string
	Payload       json.RawMessage
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
	CreatedAt     time.Time
	ProcessedAt   *time.Time
}
