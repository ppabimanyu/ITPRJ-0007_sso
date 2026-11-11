package password

import "context"

// Guard bounds concurrent Argon2 work. The cost parameters use 64 MiB per hash,
// so unbounded logins can exhaust the process before the CPU saturates.
type Guard struct {
	slots chan struct{}
}

func NewGuard(concurrency int) *Guard {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Guard{slots: make(chan struct{}, concurrency)}
}

func (g *Guard) acquire(ctx context.Context) error {
	select {
	case g.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Guard) release() {
	<-g.slots
}

// Hash waits for a slot before hashing.
func (g *Guard) Hash(ctx context.Context, plaintext string) (string, error) {
	if err := g.acquire(ctx); err != nil {
		return "", err
	}
	defer g.release()
	return Hash(plaintext)
}

// Verify waits for a slot before verifying.
func (g *Guard) Verify(ctx context.Context, plaintext, encoded string) (bool, error) {
	if err := g.acquire(ctx); err != nil {
		return false, err
	}
	defer g.release()
	return Verify(plaintext, encoded)
}

// VerifyDummy waits for a slot before spending a hash on a missing account.
func (g *Guard) VerifyDummy(ctx context.Context, plaintext string) {
	if err := g.acquire(ctx); err != nil {
		return
	}
	defer g.release()
	VerifyDummy(plaintext)
}
