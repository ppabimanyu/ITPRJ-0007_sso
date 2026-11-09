package domain

import "errors"

// Repository errors are mapped by handlers to problem+json codes.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// Public error codes from ARCHITECTURE §9.
const (
	CodeInvalidCredentials  = "invalid_credentials"
	CodeAccountLocked       = "account_locked"
	CodeMFARequired         = "mfa_required"
	CodeMFAInvalid          = "mfa_invalid"
	CodeSessionExpired      = "session_expired"
	CodeCSRFInvalid         = "csrf_invalid"
	CodeForbidden           = "forbidden"
	CodeNotFound            = "not_found"
	CodeConflict            = "conflict"
	CodeRateLimited         = "rate_limited"
	CodeIdempotencyMismatch = "idempotency_mismatch"
	CodeBadRequest          = "bad_request"
	CodeUnauthorized        = "unauthorized"
	CodeInternal            = "internal_error"
	CodeNotImplemented      = "not_implemented"
	CodeServiceUnavailable  = "service_unavailable"
)
