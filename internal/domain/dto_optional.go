package domain

import (
	"bytes"
	"encoding/json"
)

// Optional distinguishes an absent JSON field from an explicit null. The
// zero value is absent; Set with a nil Value is an explicit null.
type Optional[T any] struct {
	Set   bool
	Value *T
}

// UnmarshalJSON records that the field was present, even when it was null.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

// MarshalJSON renders an unset Optional as null. Callers tag the surrounding
// field with omitzero so an absent field stays absent.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.Set || o.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*o.Value)
}

// IsZero lets encoding/json omit an absent field with omitzero.
func (o Optional[T]) IsZero() bool { return !o.Set }
