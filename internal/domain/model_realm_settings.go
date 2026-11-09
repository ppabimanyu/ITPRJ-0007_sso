package domain

import "time"

// RealmSettings is the realm-scoped security and mail configuration. The SMTP
// password ciphertext is never serialized; only PasswordSet leaves the server.
type RealmSettings struct {
	Realm          string         `json:"realm,omitempty"`
	PasswordPolicy PasswordPolicy `json:"password_policy"`
	SessionPolicy  SessionPolicy  `json:"session_policy"`
	SMTP           SMTPSettings   `json:"smtp"`
}

// PasswordPolicy is the realm password floor.
type PasswordPolicy struct {
	MinLength int `json:"min_length"`
}

// SessionPolicy is the realm session lifetime matrix.
type SessionPolicy struct {
	IdleMinutes      int `json:"idle_minutes"`
	AbsoluteHours    int `json:"absolute_hours"`
	AdminIdleMinutes int `json:"admin_idle_minutes"`
	RememberMeDays   int `json:"remember_me_days"`
}

// SMTPSettings is the per-realm mail transport. PasswordEncrypted is internal:
// the API exposes PasswordSet instead.
type SMTPSettings struct {
	Host              string  `json:"host"`
	Port              int     `json:"port"`
	Username          *string `json:"username"`
	FromAddress       string  `json:"from_address"`
	UseTLS            bool    `json:"use_tls"`
	PasswordSet       bool    `json:"password_set"`
	PasswordEncrypted []byte  `json:"-"`
}

// DefaultRealmSettings matches the PRD lifetime matrix before an admin
// overrides a value.
func DefaultRealmSettings() *RealmSettings {
	return &RealmSettings{
		PasswordPolicy: PasswordPolicy{MinLength: 12},
		SessionPolicy: SessionPolicy{
			IdleMinutes:      30,
			AbsoluteHours:    12,
			AdminIdleMinutes: 15,
			RememberMeDays:   7,
		},
		SMTP: SMTPSettings{Port: 587},
	}
}

// IdleDuration returns the idle window for a new session.
func (p SessionPolicy) IdleDuration(admin, remember bool) time.Duration {
	switch {
	case admin:
		return time.Duration(p.AdminIdleMinutes) * time.Minute
	case remember:
		return time.Duration(p.RememberMeDays) * 24 * time.Hour
	default:
		return time.Duration(p.IdleMinutes) * time.Minute
	}
}

// AbsoluteDuration returns the absolute lifetime for a new session. Remember-me
// keeps the PRD 30-day absolute floor even when its idle window is shorter.
func (p SessionPolicy) AbsoluteDuration(remember bool) time.Duration {
	absolute := time.Duration(p.AbsoluteHours) * time.Hour
	if remember {
		rememberAbsolute := time.Duration(p.RememberMeDays) * 24 * time.Hour
		if rememberAbsolute < 30*24*time.Hour {
			rememberAbsolute = 30 * 24 * time.Hour
		}
		if rememberAbsolute > absolute {
			return rememberAbsolute
		}
	}
	return absolute
}

// UpdateRealmSettingsRequest is the partial PATCH body.
type UpdateRealmSettingsRequest struct {
	PasswordPolicy *UpdatePasswordPolicy `json:"password_policy,omitempty"`
	SessionPolicy  *UpdateSessionPolicy  `json:"session_policy,omitempty"`
	SMTP           *UpdateSMTPSettings   `json:"smtp,omitempty"`
}

// UpdatePasswordPolicy is the password policy part of a PATCH.
type UpdatePasswordPolicy struct {
	MinLength *int `json:"min_length,omitempty"`
}

// UpdateSessionPolicy is the session policy part of a PATCH.
type UpdateSessionPolicy struct {
	IdleMinutes      *int `json:"idle_minutes,omitempty"`
	AbsoluteHours    *int `json:"absolute_hours,omitempty"`
	AdminIdleMinutes *int `json:"admin_idle_minutes,omitempty"`
	RememberMeDays   *int `json:"remember_me_days,omitempty"`
}

// UpdateSMTPSettings is the SMTP part of a PATCH. Password is write-only.
type UpdateSMTPSettings struct {
	Host        *string          `json:"host,omitempty"`
	Port        *int             `json:"port,omitempty"`
	Username    Optional[string] `json:"username,omitzero"`
	FromAddress *string          `json:"from_address,omitempty"`
	UseTLS      *bool            `json:"use_tls,omitempty"`
	Password    *string          `json:"password,omitempty"`
}
