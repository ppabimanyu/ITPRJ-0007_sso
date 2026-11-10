package workerhandler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

type retentionRepo struct {
	cutoffs []time.Time
	deleted int64
	err     error
}

func (r *retentionRepo) DeleteBefore(_ context.Context, before time.Time) (int64, error) {
	r.cutoffs = append(r.cutoffs, before)
	return r.deleted, r.err
}

type cleanupRepo struct {
	cutoffs map[string]time.Time
	counts  map[string]int64
	errs    map[string]error
}

func (c *cleanupRepo) record(name string, before time.Time) (int64, error) {
	if c.cutoffs == nil {
		c.cutoffs = map[string]time.Time{}
	}
	c.cutoffs[name] = before
	return c.counts[name], c.errs[name]
}

func (c *cleanupRepo) DeleteExpiredSessions(_ context.Context, before time.Time) (int64, error) {
	return c.record("sessions", before)
}

func (c *cleanupRepo) DeleteExpiredRefreshTokens(_ context.Context, before time.Time) (int64, error) {
	return c.record("refresh", before)
}

func (c *cleanupRepo) DeleteExpiredEmailTokens(_ context.Context, before time.Time) (int64, error) {
	return c.record("email", before)
}

func (c *cleanupRepo) DeleteFailedOutbox(_ context.Context, before time.Time) (int64, error) {
	return c.record("outbox", before)
}

func (c *cleanupRepo) DeleteExpiredIdempotencyKeys(_ context.Context, before time.Time) (int64, error) {
	return c.record("idempotency", before)
}

func newRetentionWorker(audits *retentionRepo, cleanup domain.RetentionRepository) *RetentionWorker {
	return NewRetentionWorker(audits, cleanup, slog.New(slog.DiscardHandler))
}

func TestRetentionWorkerDeletesBeforeCutoff(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo := &retentionRepo{deleted: 4}
	worker := newRetentionWorker(repo, nil)
	worker.now = func() time.Time { return now }

	stats, err := worker.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Audit != 4 {
		t.Fatalf("audit deleted = %d, want 4", stats.Audit)
	}
	want := now.Add(-90 * 24 * time.Hour)
	if len(repo.cutoffs) != 1 || !repo.cutoffs[0].Equal(want) {
		t.Fatalf("cutoff = %v, want %v", repo.cutoffs, want)
	}
}

func TestRetentionWorkerAppliesEveryCleanupWindow(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	cleanup := &cleanupRepo{
		counts: map[string]int64{
			"sessions": 1, "refresh": 2, "email": 3, "outbox": 4, "idempotency": 5,
		},
	}
	worker := newRetentionWorker(&retentionRepo{}, cleanup)
	worker.now = func() time.Time { return now }

	stats, err := worker.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if stats.Sessions != 1 || stats.RefreshTokens != 2 || stats.EmailTokens != 3 ||
		stats.FailedOutbox != 4 || stats.IdempotencyKeys != 5 {
		t.Fatalf("stats = %+v", stats)
	}
	cases := map[string]time.Duration{
		"sessions":    30 * 24 * time.Hour,
		"refresh":     30 * 24 * time.Hour,
		"email":       24 * time.Hour,
		"outbox":      30 * 24 * time.Hour,
		"idempotency": 24 * time.Hour,
	}
	for name, retention := range cases {
		want := now.Add(-retention)
		if got := cleanup.cutoffs[name]; !got.Equal(want) {
			t.Errorf("%s cutoff = %v, want %v", name, got, want)
		}
	}
}

func TestRetentionWorkerIsIdempotentAtSameCutoff(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo := &retentionRepo{}
	worker := newRetentionWorker(repo, nil)
	worker.now = func() time.Time { return now }

	if _, err := worker.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if _, err := worker.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if len(repo.cutoffs) != 2 || !repo.cutoffs[0].Equal(repo.cutoffs[1]) {
		t.Fatalf("cutoffs = %v, want two equal values", repo.cutoffs)
	}
}

func TestRetentionWorkerContinuesAfterOneStepFails(t *testing.T) {
	repo := &retentionRepo{deleted: 2}
	cleanup := &cleanupRepo{errs: map[string]error{"sessions": errors.New("database is down")}}
	worker := newRetentionWorker(repo, cleanup)

	stats, err := worker.ProcessOnce(context.Background())
	if err == nil {
		t.Fatal("expected the cleanup error")
	}
	if stats.Audit != 2 {
		t.Fatalf("audit step was skipped: %+v", stats)
	}
}

func TestRetentionWorkerPropagatesAuditError(t *testing.T) {
	repo := &retentionRepo{err: errors.New("database is down")}
	worker := newRetentionWorker(repo, nil)

	if _, err := worker.ProcessOnce(context.Background()); err == nil {
		t.Fatal("expected the repository error")
	}
}

func TestRetentionWorkerDefaultRetention(t *testing.T) {
	worker := newRetentionWorker(&retentionRepo{}, &cleanupRepo{})
	if worker.auditRetention != 90*24*time.Hour {
		t.Fatalf("audit retention = %v, want 90 days", worker.auditRetention)
	}
	if worker.sessionRetention != 30*24*time.Hour {
		t.Fatalf("session retention = %v, want 30 days", worker.sessionRetention)
	}
	if worker.emailTokenRetention != 24*time.Hour {
		t.Fatalf("email token retention = %v, want 24 hours", worker.emailTokenRetention)
	}
}
