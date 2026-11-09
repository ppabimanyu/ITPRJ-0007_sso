package domain

import "time"

// Login statuses from the OpenAPI LoginResponse enum. M2 emits only
// authenticated; M4 adds the MFA statuses.
const (
	LoginStatusAuthenticated         = "authenticated"
	LoginStatusMFARequired           = "mfa_required"
	LoginStatusMFAEnrollmentRequired = "mfa_enrollment_required"
)

// CSRFResponse is the body of GET /api/v1/auth/csrf.
type CSRFResponse struct {
	CSRFToken string `json:"csrf_token"`
}

// LoginRequest is the login body.
type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
	RequestID  string `json:"request_id,omitempty"`
}

// LoginResponse is the login result.
type LoginResponse struct {
	Status           string     `json:"status"`
	MFAToken         *string    `json:"mfa_token,omitempty"`
	SessionExpiresAt *time.Time `json:"session_expires_at"`
	User             *UserDTO   `json:"user"`
	RedirectTo       *string    `json:"redirect_to,omitempty"`
}

// SessionInfo is GET /api/v1/auth/session.
type SessionInfo struct {
	User          UserDTO   `json:"user"`
	ExpiresAt     time.Time `json:"expires_at"`
	IdleExpiresAt time.Time `json:"idle_expires_at"`
	MFAEnrolled   bool      `json:"mfa_enrolled"`
	IsAdmin       bool      `json:"is_admin"`
}

// ForgotPasswordRequest requests a reset link.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest completes a reset.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// VerifyEmailRequest consumes a verification token.
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// AcceptInviteRequest consumes an invite and sets a password.
type AcceptInviteRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ChangePasswordRequest changes the signed-in user's password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ProfileUpdateRequest is the PATCH /api/v1/me body.
type ProfileUpdateRequest struct {
	Name       *string `json:"name,omitempty"`
	GivenName  *string `json:"given_name,omitempty"`
	FamilyName *string `json:"family_name,omitempty"`
	Locale     *string `json:"locale,omitempty"`
}

// MeSession is one row of GET /api/v1/me/sessions.
type MeSession struct {
	ID         string    `json:"id"`
	IP         *string   `json:"ip"`
	UserAgent  *string   `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}

// MeSessionList wraps the self-service session list.
type MeSessionList struct {
	Items []MeSession `json:"items"`
}
