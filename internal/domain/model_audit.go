package domain

import (
	"encoding/json"
	"time"
)

// OutboxStatus is the lifecycle of a transactional outbox row.
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusSent       OutboxStatus = "sent"
	OutboxStatusFailed     OutboxStatus = "failed"
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
	Status        OutboxStatus
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
	LockedUntil   *time.Time
	CreatedAt     time.Time
	ProcessedAt   *time.Time
}
