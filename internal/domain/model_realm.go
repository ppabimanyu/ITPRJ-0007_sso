package domain

import (
	"encoding/json"
	"time"
)

// Realm is a security boundary. Every business query is scoped to one.
type Realm struct {
	ID        RealmID
	Name      string
	Issuer    string
	Settings  json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}
