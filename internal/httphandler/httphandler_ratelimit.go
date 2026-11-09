package httphandler

import (
	"net/http"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

var (
	apiIPLimit        = 300
	apiIPWindow       = time.Minute
	authIPLimit       = 120
	authIPWindow      = time.Minute
	oidcIPLimit       = 120
	oidcIPWindow      = time.Minute
	tokenClientLimit  = 60
	tokenClientWindow = time.Minute
	forgotIPLimit     = 10
	forgotIPWindow    = time.Minute
)

type rateLimitMode int

const (
	rateLimitProblem rateLimitMode = iota
	rateLimitOAuth
)

type rateLimitPolicy struct {
	scope      string
	limit      int
	window     time.Duration
	mode       rateLimitMode
	failClosed bool
}

func rateLimitOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func (h *handlers) ipRateLimit(policy rateLimitPolicy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !h.allowRate(w, r, policy, clientIP(r)) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h *handlers) allowRate(w http.ResponseWriter, r *http.Request, policy rateLimitPolicy, key string) bool {
	if h.deps.RateLimit == nil {
		return h.rateLimitStorageUnavailable(w, r, policy)
	}
	decision, err := h.deps.RateLimit.Allow(r.Context(), policy.scope, key, policy.limit, policy.window)
	if err != nil {
		h.logInternal(r.Context(), "rate_limit_failed", err)
		return h.rateLimitStorageUnavailable(w, r, policy)
	}
	if decision.Allowed {
		return true
	}
	w.Header().Set("Retry-After", retryAfterSeconds(decision.RetryAfter))
	h.writeRateLimitError(w, r, policy.mode)
	return false
}

// rateLimitStorageUnavailable reports whether a missing or failed counter still
// lets the request proceed. Reads fail open; auth paths fail closed.
func (h *handlers) rateLimitStorageUnavailable(w http.ResponseWriter, r *http.Request, policy rateLimitPolicy) bool {
	if !policy.failClosed {
		return true
	}
	if policy.mode == rateLimitOAuth {
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "rate limit storage is unavailable")
		return false
	}
	WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
		"rate limit storage is unavailable")
	return false
}

func (h *handlers) writeRateLimitError(w http.ResponseWriter, r *http.Request, mode rateLimitMode) {
	if mode == rateLimitOAuth {
		writeTokenError(w, http.StatusTooManyRequests, domain.OAuthTemporarilyUnavailable, "too many requests")
		return
	}
	WriteProblem(w, r, http.StatusTooManyRequests, domain.CodeRateLimited, "too many requests")
}
