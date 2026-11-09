package httphandler

import (
	"context"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg/password"
)

const (
	adminIdle           = 15 * time.Minute
	normalIdle          = 30 * time.Minute
	rememberIdle        = 7 * 24 * time.Hour
	normalAbsolute      = 12 * time.Hour
	rememberAbsolute    = 30 * 24 * time.Hour
	lockoutThreshold    = 5
	lockoutWindow       = 15 * time.Minute
	loginIPLimit        = 10
	loginIPWindow       = time.Minute
	loginAccountLimit   = 5
	loginAccountWindow  = 15 * time.Minute
	forgotAccountLimit  = 3
	forgotAccountWindow = 15 * time.Minute
	resetTokenLifetime  = 30 * time.Minute
	inviteTokenLifetime = 7 * 24 * time.Hour
	verifyTokenLifetime = 24 * time.Hour
)

// realmSessionWindow applies the realm's session policy. Admin sessions never
// use remember-me. A settings read failure falls back to the PRD defaults so a
// transient settings error cannot block login.
func (h *handlers) realmSessionWindow(ctx context.Context, realmID domain.RealmID, now time.Time, admin, remember bool) (idle, absolute time.Time) {
	policy := domain.DefaultRealmSettings().SessionPolicy
	if h.deps.RealmSettings != nil {
		if settings, err := h.deps.RealmSettings.GetSettings(ctx, realmID); err == nil {
			policy = settings.SessionPolicy
		}
	}
	return now.Add(policy.IdleDuration(admin, remember)), now.Add(policy.AbsoluteDuration(remember))
}

// realmPasswordPolicy applies the stricter of the deployment floor and the
// realm setting.
func (h *handlers) realmPasswordPolicy(ctx context.Context, realmID domain.RealmID) password.Policy {
	minLength := h.deps.Config.Password.MinLength
	if h.deps.RealmSettings != nil {
		if settings, err := h.deps.RealmSettings.GetSettings(ctx, realmID); err == nil &&
			settings.PasswordPolicy.MinLength > minLength {
			minLength = settings.PasswordPolicy.MinLength
		}
	}
	return password.Policy{MinLength: minLength}
}

func sessionIdleTTL(admin, remember bool) time.Duration {
	switch {
	case admin:
		return adminIdle
	case remember:
		return rememberIdle
	default:
		return normalIdle
	}
}

func (h *handlers) loginIPLimit() int {
	if h.deps.Config != nil && h.deps.Config.RateLimit.LoginIPLimit > 0 {
		return h.deps.Config.RateLimit.LoginIPLimit
	}
	return loginIPLimit
}

func (h *handlers) loginAccountLimit() int {
	if h.deps.Config != nil && h.deps.Config.RateLimit.LoginAccountLimit > 0 {
		return h.deps.Config.RateLimit.LoginAccountLimit
	}
	return loginAccountLimit
}

func (h *handlers) tokenClientLimit() int {
	if h.deps.Config != nil && h.deps.Config.RateLimit.TokenClientLimit > 0 {
		return h.deps.Config.RateLimit.TokenClientLimit
	}
	return tokenClientLimit
}
