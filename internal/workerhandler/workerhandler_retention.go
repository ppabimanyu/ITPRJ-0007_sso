package workerhandler

import (
	"context"
	"log/slog"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

const (
	defaultRetentionInterval = time.Hour
	defaultAuditRetention    = 90 * 24 * time.Hour
)

// RetentionWorker deletes audit events older than the retention window.
type RetentionWorker struct {
	audits    domain.AuditRetentionRepository
	log       *slog.Logger
	interval  time.Duration
	retention time.Duration
	now       func() time.Time
}

func NewRetentionWorker(audits domain.AuditRetentionRepository, log *slog.Logger) *RetentionWorker {
	return &RetentionWorker{
		audits:    audits,
		log:       log,
		interval:  defaultRetentionInterval,
		retention: defaultAuditRetention,
		now:       time.Now,
	}
}

// Run sweeps once at start, then hourly until the context is cancelled.
func (w *RetentionWorker) Run(ctx context.Context) {
	ctx = logger.WithLogger(ctx, w.log)
	if _, err := w.ProcessOnce(ctx); err != nil {
		logger.FromContext(ctx).Error("retention_cleanup_failed", "error", err)
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := w.ProcessOnce(ctx); err != nil {
				logger.FromContext(ctx).Error("retention_cleanup_failed", "error", err)
			}
		}
	}
}

// ProcessOnce deletes events before the cutoff and reports the row count.
func (w *RetentionWorker) ProcessOnce(ctx context.Context) (int64, error) {
	cutoff := w.now().UTC().Add(-w.retention)
	deleted, err := w.audits.DeleteBefore(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	logger.FromContext(ctx).Info("audit_retention_deleted", "deleted", deleted, "cutoff", cutoff)
	return deleted, nil
}
