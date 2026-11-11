package httphandler

import (
	"context"

	"github.com/ppa/itprj-0007-sso/internal/pkg/password"
)

func (h *handlers) hashPassword(ctx context.Context, plaintext string) (string, error) {
	if h.deps.Passwords != nil {
		return h.deps.Passwords.Hash(ctx, plaintext)
	}
	return password.Hash(plaintext)
}

func (h *handlers) verifyPassword(ctx context.Context, plaintext, encoded string) (bool, error) {
	if h.deps.Passwords != nil {
		return h.deps.Passwords.Verify(ctx, plaintext, encoded)
	}
	return password.Verify(plaintext, encoded)
}

func (h *handlers) verifyDummyPassword(ctx context.Context, plaintext string) {
	if h.deps.Passwords != nil {
		h.deps.Passwords.VerifyDummy(ctx, plaintext)
		return
	}
	password.VerifyDummy(plaintext)
}
