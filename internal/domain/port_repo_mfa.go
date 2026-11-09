package domain

import "context"

// MFARepository owns TOTP seeds, recovery codes, and their replay counters.
// Methods that change more than one table open their own transaction.
type MFARepository interface {
	GetTOTP(ctx context.Context, userID UserID) (*MFATOTP, error)
	SaveTOTP(ctx context.Context, totp *MFATOTP) error
	ClaimTOTPStep(ctx context.Context, userID UserID, step int64) (bool, error)
	ConfirmTOTP(ctx context.Context, userID UserID, sessionID SessionID, lastUsedStep int64, codes []MFARecoveryCode, audit *AuditEvent) error
	ReplaceRecoveryCodes(ctx context.Context, userID UserID, codes []MFARecoveryCode, audit *AuditEvent) error
	UseRecoveryCode(ctx context.Context, userID UserID, codeHash string, audit *AuditEvent) (bool, error)
	DeleteTOTP(ctx context.Context, userID UserID, audit *AuditEvent) error
}

// MFAChallengeRepository stores the pre-session MFA challenge in Redis. A
// missing or expired challenge fails closed.
type MFAChallengeRepository interface {
	Create(ctx context.Context, token string, challenge *MFAChallenge) error
	Get(ctx context.Context, token string) (*MFAChallenge, error)
	RecordFailure(ctx context.Context, token string) (int, error)
	Delete(ctx context.Context, token string) error
}
