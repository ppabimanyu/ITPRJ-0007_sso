package domain

import (
	"context"
	"time"
)

// AuditRetentionRepository deletes audit events past the retention window.
type AuditRetentionRepository interface {
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}
