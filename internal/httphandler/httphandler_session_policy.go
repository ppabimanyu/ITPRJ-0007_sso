package httphandler

import "time"

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

// sessionWindow is the PRD §8.3 lifetime matrix. Admin sessions never use
// remember-me.
func sessionWindow(now time.Time, admin, remember bool) (idle, absolute time.Time) {
	switch {
	case admin:
		return now.Add(adminIdle), now.Add(normalAbsolute)
	case remember:
		return now.Add(rememberIdle), now.Add(rememberAbsolute)
	default:
		return now.Add(normalIdle), now.Add(normalAbsolute)
	}
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
