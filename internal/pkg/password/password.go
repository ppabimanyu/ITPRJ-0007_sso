// Package password hashes and validates passwords. Argon2id parameters and the
// policy threshold come from the PRD; the policy itself stays pure so handlers
// can report field errors.
package password

import (
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Params are the Argon2id cost parameters.
type Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams uses 64 MiB, three passes, four lanes, a 16-byte salt, and a
// 32-byte derived key.
var DefaultParams = Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

//go:embed common_passwords.txt
var commonPasswordsRaw string

var commonPasswords = func() map[string]struct{} {
	set := make(map[string]struct{})
	for _, line := range strings.Split(commonPasswordsRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[strings.ToLower(line)] = struct{}{}
	}
	return set
}()

// Hash returns a PHC-formatted Argon2id hash with DefaultParams.
func Hash(plaintext string) (string, error) {
	return HashWithParams(plaintext, DefaultParams)
}

// HashWithParams hashes with explicit cost parameters. Tests use cheap values.
func HashWithParams(plaintext string, params Params) (string, error) {
	if plaintext == "" {
		return "", errors.New("password must not be empty")
	}
	salt := make([]byte, params.SaltLength)
	if _, err := readRandom(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(plaintext), salt, params.Iterations,
		params.Memory, params.Parallelism, params.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, params.Memory, params.Iterations, params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// Verify checks a password against a PHC Argon2id hash in constant time.
func Verify(plaintext, encoded string) (bool, error) {
	params, salt, want, err := parseHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(plaintext), salt, params.Iterations,
		params.Memory, params.Parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// ParamsJSON renders the stored cost metadata for a credential row.
func ParamsJSON(params Params) string {
	return fmt.Sprintf(`{"m":%d,"t":%d,"p":%d,"salt_len":%d,"key_len":%d}`,
		params.Memory, params.Iterations, params.Parallelism,
		params.SaltLength, params.KeyLength)
}

// Policy rejects short, common, and identity-bearing passwords.
type Policy struct {
	MinLength int
}

// FieldError names one rejected rule for a 422 response.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError carries every failed rule rather than only the first.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("password rejected by %d rule(s)", len(e.Errors))
}

// Validate checks the password against length, the embedded common list, and
// the user's email local part and display name.
func (p Policy) Validate(plaintext, email, name string) error {
	minLength := p.MinLength
	if minLength <= 0 {
		minLength = 12
	}
	var failures []FieldError
	add := func(message string) {
		failures = append(failures, FieldError{Field: "new_password", Message: message})
	}
	if utf8.RuneCountInString(plaintext) < minLength {
		add(fmt.Sprintf("must be at least %d characters", minLength))
	}
	if len(plaintext) > 256 {
		add("must be at most 256 characters")
	}
	if _, ok := commonPasswords[strings.ToLower(plaintext)]; ok {
		add("is too common")
	}
	lower := strings.ToLower(plaintext)
	if local := emailLocalPart(email); len(local) >= 3 && strings.Contains(lower, local) {
		add("must not contain your email address")
	}
	for _, token := range strings.Fields(strings.ToLower(name)) {
		if len(token) < 3 {
			continue
		}
		if strings.Contains(lower, token) {
			add("must not contain your name")
			break
		}
	}
	if len(failures) > 0 {
		return &ValidationError{Errors: failures}
	}
	return nil
}

func emailLocalPart(email string) string {
	local, _, ok := strings.Cut(strings.TrimSpace(email), "@")
	if !ok {
		return ""
	}
	return strings.ToLower(local)
}

// dummyHash is a real Argon2id hash of a random value. Unknown-email logins
// verify against it so the response time does not reveal account existence.
var dummyHash = "$argon2id$v=19$m=65536,t=3,p=4$xNH9iFUxNjQVQf2ItN7uTA$SHooVes0BRQcFJM1IKsgYN/9PMYzdq1/4RCp/9JzNBE"

// VerifyDummy burns the same Argon2 cost as a real verification.
func VerifyDummy(plaintext string) {
	_, _ = Verify(plaintext, dummyHash)
}

func parseHash(encoded string) (Params, []byte, []byte, error) {
	var params Params
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
