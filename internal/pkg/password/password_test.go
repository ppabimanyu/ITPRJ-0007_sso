package password

import (
	"errors"
	"strings"
	"testing"
)

func fastParams() Params {
	params := DefaultParams
	params.Memory = 8 * 1024
	params.Iterations = 1
	params.Parallelism = 1
	return params
}

func TestHashVerifyRoundTrip(t *testing.T) {
	encoded, err := HashWithParams("correct horse battery staple", fastParams())
	if err != nil {
		t.Fatalf("HashWithParams() error = %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Errorf("hash is not a PHC argon2id string: %q", encoded)
	}
	ok, err := Verify("correct horse battery staple", encoded)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !ok {
		t.Error("Verify() = false for the correct password")
	}
	ok, err = Verify("wrong password", encoded)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if ok {
		t.Error("Verify() = true for a wrong password")
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, encoded := range []string{
		"",
		"$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=0,t=3,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=99$m=65536,t=3,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=3,p=4$!!!$aGFzaA",
	} {
		if _, err := Verify("x", encoded); err == nil {
			t.Errorf("Verify(%q) accepted a malformed hash", encoded)
		}
	}
}

func TestPolicyRules(t *testing.T) {
	policy := Policy{MinLength: 12}
	cases := []struct {
		name     string
		password string
		email    string
		display  string
		wantErr  bool
		contains string
	}{
		{"valid", "correct horse battery staple", "user@example.com", "Sample User", false, ""},
		{"short", "short", "user@example.com", "Sample User", true, "at least 12"},
		{"common", "password123", "user@example.com", "Sample User", true, "too common"},
		{"email local part", "user-secret-123", "user@example.com", "Sample User", true, "email address"},
		{"display name", "sample-secret-123", "user@example.com", "Sample User", true, "your name"},
		{"too long", strings.Repeat("a", 257), "user@example.com", "Sample User", true, "at most 256"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.Validate(tc.password, tc.email, tc.display)
			if tc.wantErr && err == nil {
				t.Fatal("Validate() accepted a rejected password")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			var validation *ValidationError
			if tc.wantErr {
				if !errors.As(err, &validation) {
					t.Fatalf("error is not ValidationError: %v", err)
				}
				if !containsMessage(validation, tc.contains) {
					t.Errorf("messages %v do not contain %q", validation.Errors, tc.contains)
				}
			}
		})
	}
}

func TestPolicyDefaultsMinLength(t *testing.T) {
	if err := (Policy{}).Validate("short", "", ""); err == nil {
		t.Fatal("zero MinLength accepted a short password")
	}
}

func TestVerifyDummy(t *testing.T) {
	VerifyDummy("anything")
	if _, err := Verify("anything", dummyHash); err != nil {
		t.Fatalf("dummy hash is malformed: %v", err)
	}
}

func containsMessage(validation *ValidationError, fragment string) bool {
	for _, field := range validation.Errors {
		if strings.Contains(field.Message, fragment) {
			return true
		}
	}
	return false
}
