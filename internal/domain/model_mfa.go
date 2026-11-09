package domain

import "time"

// MFATOTP is the encrypted TOTP seed and replay marker for a user.
type MFATOTP struct {
	UserID          UserID
	SecretEncrypted []byte
	LastUsedStep    int64
	ConfirmedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MFARecoveryCode is a single-use recovery code stored as a SHA-256 hash.
type MFARecoveryCode struct {
	ID        string
	UserID    UserID
	CodeHash  string
	UsedAt    *time.Time
	CreatedAt time.Time
}

// MFAChallenge is the short-lived state between a password login and a
// completed TOTP challenge. It lives in Redis, never in the session table.
type MFAChallenge struct {
	UserID     UserID
	RealmID    RealmID
	RequestID  string
	Attempts   int
	RememberMe bool
	ExpiresAt  time.Time
}
