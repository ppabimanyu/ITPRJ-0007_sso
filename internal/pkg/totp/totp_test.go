package totp

import (
	"strings"
	"testing"
	"time"
)

func TestVerifyAcceptsWindowAndRejectsOutside(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	current := Step(now)

	for _, offset := range []int64{-1, 0, 1} {
		code, err := Code(secret, current+offset)
		if err != nil {
			t.Fatalf("code: %v", err)
		}
		matched, ok := Verify(secret, code, now)
		if !ok {
			t.Fatalf("code at offset %d was rejected", offset)
		}
		if matched != current+offset {
			t.Fatalf("matched step = %d, want %d", matched, current+offset)
		}
	}

	for _, offset := range []int64{-2, 2} {
		code, err := Code(secret, current+offset)
		if err != nil {
			t.Fatalf("code: %v", err)
		}
		if _, ok := Verify(secret, code, now); ok {
			t.Fatalf("code at offset %d was accepted", offset)
		}
	}
}

func TestVerifyRejectsReplayedStep(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	now := time.Unix(1_700_000_100, 0).UTC()
	step := Step(now)
	code, err := Code(secret, step)
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	matched, ok := Verify(secret, code, now)
	if !ok {
		t.Fatal("current code was rejected")
	}
	if AcceptStep(step, matched) {
		t.Fatal("reused step was accepted")
	}
	if AcceptStep(matched, matched) {
		t.Fatal("same step was accepted twice")
	}
	if !AcceptStep(step-1, step) {
		t.Fatal("newer step was rejected")
	}
}

func TestRecoveryCodesAreUniqueAndNormalized(t *testing.T) {
	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("generate recovery codes: %v", err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("code count = %d, want %d", len(codes), RecoveryCodeCount)
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		hash := RecoveryCodeHash(code)
		if _, ok := seen[hash]; ok {
			t.Fatalf("duplicate recovery code hash for %q", code)
		}
		seen[hash] = struct{}{}
		lower := strings.ToLower(strings.ReplaceAll(code, "-", " "))
		if RecoveryCodeHash(lower) != hash {
			t.Fatalf("normalization changed the hash for %q", code)
		}
	}
}

func TestOTPAuthURIContainsIssuerAndSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	uri := OTPAuthURI("ITPRJ SSO", "admin@example.com", secret)
	for _, want := range []string{"otpauth://totp/", "secret=" + secret, "issuer=ITPRJ+SSO"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("uri %q does not contain %q", uri, want)
		}
	}
}
