package workerhandler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

const (
	defaultRetentionInterval    = time.Hour
	defaultAuditRetention       = 90 * 24 * time.Hour
	defaultSessionRetention     = 30 * 24 * time.Hour
	defaultRefreshRetention     = 30 * 24 * time.Hour
	defaultEmailTokenRetention  = 24 * time.Hour
	defaultOutboxRetention      = 30 * 24 * time.Hour
	defaultIdempotencyRetention = 24 * time.Hour
)

// RetentionStats reports how many rows each cleanup step removed.
type RetentionStats struct {
	Audit           int64
	Sessions        int64
	RefreshTokens   int64
	EmailTokens     int64
	FailedOutbox    int64
	IdempotencyKeys int64
}

// RetentionWorker deletes audit events and expired operational rows.
type RetentionWorker struct {
	audits               domain.AuditRetentionRepository
	cleanup              domain.RetentionRepository
	log                  *slog.Logger
	interval             time.Duration
	auditRetention       time.Duration
	sessionRetention     time.Duration
	refreshRetention     time.Duration
	emailTokenRetention  time.Duration
	outboxRetention      time.Duration
	idempotencyRetention time.Duration
	now                  func() time.Time
}

func NewRetentionWorker(audits domain.AuditRetentionRepository, cleanup domain.RetentionRepository, log *slog.Logger) *RetentionWorker {
	return &RetentionWorker{
		audits:               audits,
		cleanup:              cleanup,
		log:                  log,
		interval:             defaultRetentionInterval,
		auditRetention:       defaultAuditRetention,
		sessionRetention:     defaultSessionRetention,
		refreshRetention:     defaultRefreshRetention,
		emailTokenRetention:  defaultEmailTokenRetention,
		outboxRetention:      defaultOutboxRetention,
		idempotencyRetention: defaultIdempotencyRetention,
		now:                  time.Now,
	}
}

// Run sweeps once at start, then hourly until the context is cancelled.
func (w *RetentionWorker) Run(ctx context.Context) {
	ctx = logger.WithLogger(ctx, w.log)
	w.sweep(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *RetentionWorker) sweep(ctx context.Context) {
	stats, err := w.ProcessOnce(ctx)
	if err != nil {
		logger.FromContext(ctx).Warn("retention_cleanup_failed", "error", err)
	}
	logger.FromContext(ctx).Info("retention_cleanup",
		"audit", stats.Audit,
		"sessions", stats.Sessions,
		"refresh_rows", stats.RefreshTokens,
		"email_rows", stats.EmailTokens,
		"failed_outbox", stats.FailedOutbox,
		"idempotency_keys", stats.IdempotencyKeys)
}

// ProcessOnce runs every cleanup step and joins their errors.
func (w *RetentionWorker) ProcessOnce(ctx context.Context) (RetentionStats, error) {
	now := w.now().UTC()
	stats := RetentionStats{}
	var errs []error

	if w.audits != nil {
		count, err := w.audits.DeleteBefore(ctx, now.Add(-w.auditRetention))
		if err != nil {
			errs = append(errs, fmt.Errorf("audit: %w", err))
		} else {
			stats.Audit = count
		}
	}
	if w.cleanup != nil {
		if count, err := w.cleanup.DeleteExpiredSessions(ctx, now.Add(-w.sessionRetention)); err != nil {
			errs = append(errs, fmt.Errorf("sessions: %w", err))
		} else {
			stats.Sessions = count
		}
		if count, err := w.cleanup.DeleteExpiredRefreshTokens(ctx, now.Add(-w.refreshRetention)); err != nil {
			errs = append(errs, fmt.Errorf("refresh tokens: %w", err))
		} else {
			stats.RefreshTokens = count
		}
		if count, err := w.cleanup.DeleteExpiredEmailTokens(ctx, now.Add(-w.emailTokenRetention)); err != nil {
			errs = append(errs, fmt.Errorf("email tokens: %w", err))
		} else {
			stats.EmailTokens = count
		}
		if count, err := w.cleanup.DeleteFailedOutbox(ctx, now.Add(-w.outboxRetention)); err != nil {
			errs = append(errs, fmt.Errorf("failed outbox: %w", err))
		} else {
			stats.FailedOutbox = count
		}
		if count, err := w.cleanup.DeleteExpiredIdempotencyKeys(ctx, now.Add(-w.idempotencyRetention)); err != nil {
			errs = append(errs, fmt.Errorf("idempotency keys: %w", err))
		} else {
			stats.IdempotencyKeys = count
		}
	}
	return stats, errors.Join(errs...)
}
