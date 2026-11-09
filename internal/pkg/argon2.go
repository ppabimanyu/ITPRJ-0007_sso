package pkg

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are the bootstrap password hashing parameters from PRD §9.
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2Params uses 64 MiB, three passes, four lanes, a 16-byte salt,
// and a 32-byte derived key.
var DefaultArgon2Params = Argon2Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword returns a PHC-formatted Argon2id hash.
func HashPassword(password string) (string, error) {
	return HashPasswordWithParams(password, DefaultArgon2Params)
}

// HashPasswordWithParams is HashPassword with explicit parameters, used by
// tests to keep them fast.
func HashPasswordWithParams(password string, params Argon2Params) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	salt, err := RandomBytes(int(params.SaltLength))
	if err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, params.Iterations,
		params.Memory, params.Parallelism, params.KeyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, params.Memory, params.Iterations, params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword checks a password against a PHC Argon2id hash in constant
// time. Malformed hashes return an error; mismatches return false, nil.
func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, want, err := parseArgon2Hash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, params.Iterations,
		params.Memory, params.Parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func parseArgon2Hash(encoded string) (Argon2Params, []byte, []byte, error) {
	var params Argon2Params
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return params, nil, nil, errors.New("invalid argon2id hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return params, nil, nil, errors.New("unsupported argon2 version")
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d",
		&params.Memory, &params.Iterations, &params.Parallelism); err != nil {
		return params, nil, nil, fmt.Errorf("invalid argon2 params: %w", err)
	}
	if params.Memory == 0 || params.Iterations == 0 || params.Parallelism == 0 {
		return params, nil, nil, errors.New("invalid argon2 params")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return params, nil, nil, fmt.Errorf("decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return params, nil, nil, fmt.Errorf("decode hash: %w", err)
	}
	if len(salt) == 0 || len(want) == 0 {
		return params, nil, nil, errors.New("empty salt or hash")
	}
	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(want))
	return params, salt, want, nil
}

// Argon2ParamsJSON renders parameters for the credential row. It keeps the
// stored metadata readable without a second encoding pass.
func Argon2ParamsJSON(params Argon2Params) string {
	return fmt.Sprintf(`{"m":%d,"t":%d,"p":%d,"salt_len":%d,"key_len":%d}`,
		params.Memory, params.Iterations, params.Parallelism,
		params.SaltLength, params.KeyLength)
}
