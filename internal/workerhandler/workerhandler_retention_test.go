package workerhandler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
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

func newRetentionWorker(repo *retentionRepo) *RetentionWorker {
	return NewRetentionWorker(repo, slog.New(slog.DiscardHandler))
}

func TestRetentionWorkerDeletesBeforeCutoff(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo := &retentionRepo{deleted: 4}
	worker := newRetentionWorker(repo)
	worker.now = func() time.Time { return now }

	deleted, err := worker.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if deleted != 4 {
		t.Fatalf("deleted = %d, want 4", deleted)
	}
	want := now.Add(-90 * 24 * time.Hour)
	if len(repo.cutoffs) != 1 || !repo.cutoffs[0].Equal(want) {
		t.Fatalf("cutoff = %v, want %v", repo.cutoffs, want)
	}
}

func TestRetentionWorkerIsIdempotentAtSameCutoff(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo := &retentionRepo{}
	worker := newRetentionWorker(repo)
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

func TestRetentionWorkerPropagatesError(t *testing.T) {
	repo := &retentionRepo{err: errors.New("database is down")}
	worker := newRetentionWorker(repo)

	if _, err := worker.ProcessOnce(context.Background()); err == nil {
		t.Fatal("expected the repository error")
	}
}

func TestRetentionWorkerDefaultRetention(t *testing.T) {
	worker := newRetentionWorker(&retentionRepo{})
	if worker.retention != 90*24*time.Hour {
		t.Fatalf("retention = %v, want 90 days", worker.retention)
	}
}
