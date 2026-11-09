package domain

import (
	"encoding/json"
	"time"
)

// IdempotencyKey stores a replayable response for one endpoint and key.
type IdempotencyKey struct {
	ID          string
	RealmID     RealmID
	Key         string
	Endpoint    string
	RequestHash string
	Response    json.RawMessage
	CreatedAt   time.Time
	ExpiresAt   time.Time
}
