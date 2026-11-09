package pkg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// RandomBytes reads n cryptographically secure bytes.
func RandomBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("random byte count must be non-negative")
	}
	out := make([]byte, n)
	if _, err := randomRead(out); err != nil {
		return nil, err
	}
	return out, nil
}

// RandomToken returns n random bytes encoded as unpadded base64url.
func RandomToken(n int) (string, error) {
	out, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(out), nil
}

func randomRead(b []byte) (int, error) {
	return rand.Read(b)
}
