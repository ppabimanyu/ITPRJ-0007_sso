// Package totp implements RFC 6238 time-based one-time passwords and the
// recovery codes that back up a lost authenticator.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// StepSeconds is the TOTP time step required by PRD FR-AUT-10.
	StepSeconds = 30
	// Digits is the length of a TOTP code.
	Digits = 6
	// Window is the accepted clock skew in steps.
	Window = 1
	// SecretBytes is 160 bits, the RFC 4226 recommended seed size.
	SecretBytes = 20
	// RecoveryCodeCount is the number of backup codes issued at enrollment.
	RecoveryCodeCount = 10
	recoveryBytes     = 10
)

// GenerateSecret returns a 160-bit secret encoded as unpadded base32.
func GenerateSecret() (string, error) {
	raw := make([]byte, SecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read random secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// Step returns the time step for an instant.
func Step(at time.Time) int64 {
	return at.UTC().Unix() / StepSeconds
}

// Code computes the six-digit code for one step.
func Code(secret string, step int64) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	return codeForKey(key, step), nil
}

// Verify checks a code against the current step and the configured window. It
// returns the matched step so the caller can reject replay of an old step.
func Verify(secret, code string, at time.Time) (int64, bool) {
	return VerifyWindow(secret, code, at, Window)
}

// VerifyWindow is Verify with an explicit window.
func VerifyWindow(secret, code string, at time.Time, window int) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != Digits {
		return 0, false
	}
	key, err := decodeSecret(secret)
	if err != nil {
		return 0, false
	}
	if window < 0 {
		window = 0
	}
	current := Step(at)
	for offset := -window; offset <= window; offset++ {
		candidate := current + int64(offset)
		expected := codeForKey(key, candidate)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return candidate, true
		}
	}
	return 0, false
}

// AcceptStep reports whether a matched step is newer than the last accepted
// one. Equal or older steps are replays.
func AcceptStep(lastUsedStep, matchedStep int64) bool {
	return matchedStep > lastUsedStep
}

// OTPAuthURI builds the otpauth:// URI an authenticator app imports.
func OTPAuthURI(issuer, account, secret string) string {
	label := url.PathEscape(strings.TrimSpace(issuer) + ":" + strings.TrimSpace(account))
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", "30")
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// GenerateRecoveryCodes returns ten one-time codes. Codes are grouped for
// readability and normalized before hashing.
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, 0, RecoveryCodeCount)
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	for i := 0; i < RecoveryCodeCount; i++ {
		raw := make([]byte, recoveryBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("read random recovery code: %w", err)
		}
		code := encoding.EncodeToString(raw)
		codes = append(codes, groupCode(code))
	}
	return codes, nil
}

// NormalizeRecoveryCode removes separators and uppercases the value so a user
// can type a displayed code with or without spaces or hyphens.
func NormalizeRecoveryCode(code string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", "\t", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(code)))
}

// RecoveryCodeHash is the SHA-256 digest stored at rest.
func RecoveryCodeHash(code string) string {
	sum := sha256.Sum256([]byte(NormalizeRecoveryCode(code)))
	return hex.EncodeToString(sum[:])
}

func decodeSecret(secret string) ([]byte, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	padded := base32.StdEncoding.WithPadding(base32.NoPadding)
	key, err := padded.DecodeString(strings.TrimRight(normalized, "="))
	if err != nil {
		return nil, fmt.Errorf("decode totp secret: %w", err)
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("decode totp secret: empty secret")
	}
	return key, nil
}

func codeForKey(key []byte, step int64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}

func groupCode(code string) string {
	var builder strings.Builder
	for i := 0; i < len(code); i += 4 {
		if i > 0 {
			builder.WriteByte('-')
		}
		end := i + 4
		if end > len(code) {
			end = len(code)
		}
		builder.WriteString(code[i:end])
	}
	return builder.String()
}
