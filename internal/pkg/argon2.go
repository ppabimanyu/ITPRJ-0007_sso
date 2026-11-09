package pkg

import "github.com/ppa/itprj-0007-sso/internal/pkg/password"

// Argon2Params keeps the original package surface for the CLI. The
// implementation lives in internal/pkg/password.
type Argon2Params = password.Params

// DefaultArgon2Params is the PRD §9 cost profile.
var DefaultArgon2Params = password.DefaultParams

// HashPassword returns a PHC-formatted Argon2id hash.
func HashPassword(plaintext string) (string, error) {
	return password.Hash(plaintext)
}

// HashPasswordWithParams hashes with explicit parameters.
func HashPasswordWithParams(plaintext string, params Argon2Params) (string, error) {
	return password.HashWithParams(plaintext, params)
}

// VerifyPassword checks a password against a PHC hash in constant time.
func VerifyPassword(plaintext, encoded string) (bool, error) {
	return password.Verify(plaintext, encoded)
}

// Argon2ParamsJSON renders the cost metadata for a credential row.
func Argon2ParamsJSON(params Argon2Params) string {
	return password.ParamsJSON(params)
}
