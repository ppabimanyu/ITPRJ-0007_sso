package pkg

import (
	"bytes"
	"testing"
)

func testKeyring(t *testing.T) *Keyring {
	t.Helper()
	keyA := bytes.Repeat([]byte{0x11}, 32)
	keyB := bytes.Repeat([]byte{0x22}, 32)
	ring, err := NewKeyring(map[string][]byte{"a": keyA, "b": keyB}, "a")
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	return ring
}

func TestEnvelopeRoundTrip(t *testing.T) {
	ring := testKeyring(t)
	plaintext := []byte("rsa-private-key-material")
	envelope, err := ring.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	keyID, err := ring.KeyID(envelope)
	if err != nil {
		t.Fatalf("KeyID() error = %v", err)
	}
	if keyID != "a" {
		t.Errorf("KeyID() = %q, want a", keyID)
	}
	got, err := ring.Decrypt(envelope)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt() = %q", got)
	}
}

func TestEnvelopeWrongKeyID(t *testing.T) {
	ring := testKeyring(t)
	envelope, err := ring.EncryptWith([]byte("x"), "b")
	if err != nil {
		t.Fatalf("EncryptWith() error = %v", err)
	}

	other, err := NewKeyring(map[string][]byte{"a": bytes.Repeat([]byte{0x11}, 32)}, "a")
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	if _, err := other.Decrypt(envelope); err == nil {
		t.Fatal("Decrypt() with a foreign keyring succeeded")
	}
}

func TestEnvelopeTamper(t *testing.T) {
	ring := testKeyring(t)
	envelope, err := ring.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	envelope[len(envelope)-1] ^= 0xff
	if _, err := ring.Decrypt(envelope); err == nil {
		t.Fatal("Decrypt() accepted a tampered envelope")
	}
}

func TestNewKeyringRejectsBadInput(t *testing.T) {
	if _, err := NewKeyring(map[string][]byte{"a": []byte("short")}, "a"); err == nil {
		t.Error("short key accepted")
	}
	if _, err := NewKeyring(map[string][]byte{"a": bytes.Repeat([]byte{1}, 32)}, "b"); err == nil {
		t.Error("missing active key accepted")
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	params := DefaultArgon2Params
	params.Memory = 8 * 1024
	params.Iterations = 1
	params.Parallelism = 1

	encoded, err := HashPasswordWithParams("correct horse battery staple", params)
	if err != nil {
		t.Fatalf("HashPasswordWithParams() error = %v", err)
	}
	ok, err := VerifyPassword("correct horse battery staple", encoded)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() = false for the correct password")
	}
	ok, err = VerifyPassword("wrong password", encoded)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Error("VerifyPassword() = true for a wrong password")
	}
}

func TestVerifyPasswordMalformed(t *testing.T) {
	for _, encoded := range []string{
		"",
		"$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=0,t=3,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=99$m=65536,t=3,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=3,p=4$!!!$aGFzaA",
	} {
		if _, err := VerifyPassword("x", encoded); err == nil {
			t.Errorf("VerifyPassword(%q) accepted a malformed hash", encoded)
		}
	}
}

func TestRandomToken(t *testing.T) {
	a, err := RandomToken(32)
	if err != nil {
		t.Fatalf("RandomToken() error = %v", err)
	}
	b, err := RandomToken(32)
	if err != nil {
		t.Fatalf("RandomToken() error = %v", err)
	}
	if a == b {
		t.Fatal("RandomToken() returned the same value twice")
	}
	if len(a) < 40 {
		t.Errorf("RandomToken(32) = %q, too short", a)
	}
}
