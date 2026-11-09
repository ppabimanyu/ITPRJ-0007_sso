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
