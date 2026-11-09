// Package workerhandler runs background jobs that share the HTTP binary. M2
// ships the transactional outbox email worker.
package workerhandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg/mailtemplate"
)

const (
	defaultPollInterval = time.Second
	defaultLockFor      = time.Minute
	maxRetries          = 3
)

// OutboxWorker claims outbox rows, renders email, and sends them. A row that
// fails three retries is marked failed permanently.
type OutboxWorker struct {
	repo     domain.OutboxRepository
	mail     domain.MailSender
	log      *slog.Logger
	poll     time.Duration
	lockFor  time.Duration
	maxRetry int
}

func NewOutboxWorker(repo domain.OutboxRepository, mail domain.MailSender, log *slog.Logger) *OutboxWorker {
	return &OutboxWorker{
		repo:     repo,
		mail:     mail,
		log:      log,
		poll:     defaultPollInterval,
		lockFor:  defaultLockFor,
		maxRetry: maxRetries,
	}
}

// Run polls until the context is cancelled.
func (w *OutboxWorker) Run(ctx context.Context) {
	ctx = logger.WithLogger(ctx, w.log)
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := w.ProcessOnce(ctx); err != nil {
				logger.FromContext(ctx).Error("outbox_claim_failed", "error", err)
			}
		}
	}
}

// ProcessOnce claims and handles one batch. It is exported so tests can drive
// a single deterministic pass.
func (w *OutboxWorker) ProcessOnce(ctx context.Context) (int, error) {
	ctx = logger.WithLogger(ctx, w.log)
	events, err := w.repo.Claim(ctx, 10, w.lockFor)
	if err != nil {
		return 0, err
	}
	for i := range events {
		w.handle(ctx, &events[i])
	}
	return len(events), nil
}

func (w *OutboxWorker) handle(ctx context.Context, event *domain.OutboxEvent) {
	log := logger.FromContext(ctx).With(
		slog.String("outbox_id", string(event.ID)),
		slog.String("outbox_type", event.Type),
		slog.Int("attempt", event.Attempts+1),
	)
	var job domain.MailJob
	if err := json.Unmarshal(event.Payload, &job); err != nil {
		log.Error("outbox_payload_invalid")
		w.failPermanently(ctx, event, "invalid payload")
		return
	}
	message, err := mailtemplate.Render(job.Template, job.Locale, job.Name, job.Link)
	if err != nil {
		log.Error("outbox_template_invalid")
		w.failPermanently(ctx, event, "invalid template")
		return
	}
	if err := w.mail.Send(ctx, domain.MailMessage{
		To:      job.To,
		Subject: message.Subject,
		Text:    message.Text,
		HTML:    message.HTML,
	}); err != nil {
		log.Warn("outbox_send_failed")
		w.retryOrFail(ctx, event)
		return
	}
	if err := w.repo.MarkSent(ctx, event.ID); err != nil {
		log.Error("outbox_mark_sent_failed", "error", err)
		return
	}
	log.Info("outbox_sent")
}

func (w *OutboxWorker) retryOrFail(ctx context.Context, event *domain.OutboxEvent) {
	if event.Attempts >= w.maxRetry {
		w.failPermanently(ctx, event, "retry limit reached")
		return
	}
	next := time.Now().UTC().Add(outboxBackoff(event.Attempts))
	if err := w.repo.MarkRetry(ctx, event.ID, "email send failed", next); err != nil {
		logger.FromContext(ctx).Error("outbox_mark_retry_failed", "error", err)
	}
}

func (w *OutboxWorker) failPermanently(ctx context.Context, event *domain.OutboxEvent, reason string) {
	if err := w.repo.MarkFailed(ctx, event.ID, reason); err != nil {
		logger.FromContext(ctx).Error("outbox_mark_failed_failed", "error", err)
	}
}

func outboxBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt)) * 5 * time.Second
}
