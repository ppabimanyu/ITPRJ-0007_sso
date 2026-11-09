package pkg

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex is the canonical digest for opaque tokens stored at rest.
func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
