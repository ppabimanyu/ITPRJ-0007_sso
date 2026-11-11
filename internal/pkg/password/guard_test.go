package password

import (
	"context"
	"testing"
	"time"
)

func TestGuardLimitsConcurrency(t *testing.T) {
	guard := NewGuard(2)
	ctx := context.Background()
	if err := guard.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := guard.acquire(ctx); err != nil {
		t.Fatal(err)
	}

	acquired := make(chan struct{})
	go func() {
		_ = guard.acquire(ctx)
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("third acquire did not block")
	case <-time.After(50 * time.Millisecond):
	}

	guard.release()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("blocked acquire did not resume after release")
	}
	guard.release()
	guard.release()
}

func TestGuardHonorsContextCancellation(t *testing.T) {
	guard := NewGuard(1)
	ctx, cancel := context.WithCancel(context.Background())
	if err := guard.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()

	if err := guard.acquire(ctx); err == nil {
		t.Fatal("acquire should fail after cancellation")
	}
	guard.release()
}

func TestGuardHashesAndVerifies(t *testing.T) {
	guard := NewGuard(1)
	ctx := context.Background()
	hash, err := guard.Hash(ctx, "correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := guard.Verify(ctx, "correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("verify = %v, err = %v", ok, err)
	}
}
