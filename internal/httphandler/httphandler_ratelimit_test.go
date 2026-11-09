package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

type fakeRateLimit struct {
	decision domain.RateLimitDecision
	err      error
	keys     []string
}

func (f *fakeRateLimit) Allow(_ context.Context, scope, key string, _ int, _ time.Duration) (domain.RateLimitDecision, error) {
	f.keys = append(f.keys, scope+":"+key)
	return f.decision, f.err
}

func newRateHandlers(repo domain.RateLimitRepository) *handlers {
	return &handlers{deps: Deps{RateLimit: repo}}
}

func rateRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r = r.WithContext(logger.WithLogger(r.Context(), slog.New(slog.DiscardHandler)))
	return r
}

func TestAllowRateReportsProblemWhenLimited(t *testing.T) {
	h := newRateHandlers(&fakeRateLimit{
		decision: domain.RateLimitDecision{Allowed: false, RetryAfter: 30 * time.Second},
	})
	rec := httptest.NewRecorder()

	allowed := h.allowRate(rec, rateRequest(), rateLimitPolicy{
		scope: "login_ip", limit: 1, window: time.Minute,
		mode: rateLimitProblem, failClosed: true,
	}, "203.0.113.9")

	if allowed {
		t.Fatal("request should be limited")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "30" {
		t.Fatalf("Retry-After = %q, want 30", got)
	}
	if code := problemCode(t, rec.Body.Bytes()); code != domain.CodeRateLimited {
		t.Fatalf("problem code = %q", code)
	}
}

func TestAllowRateReportsOAuthErrorWhenLimited(t *testing.T) {
	h := newRateHandlers(&fakeRateLimit{
		decision: domain.RateLimitDecision{Allowed: false, RetryAfter: time.Second},
	})
	rec := httptest.NewRecorder()

	allowed := h.allowRate(rec, rateRequest(), rateLimitPolicy{
		scope: "oidc_ip", limit: 1, window: time.Minute,
		mode: rateLimitOAuth, failClosed: true,
	}, "203.0.113.9")

	if allowed {
		t.Fatal("request should be limited")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	var oauth struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &oauth); err != nil {
		t.Fatalf("body is not an OAuth error: %v (%s)", err, rec.Body.String())
	}
	if oauth.Error != domain.OAuthTemporarilyUnavailable {
		t.Fatalf("error = %q", oauth.Error)
	}
}

func TestAllowRateFailsOpenOnRepositoryError(t *testing.T) {
	h := newRateHandlers(&fakeRateLimit{err: errors.New("redis is down")})
	rec := httptest.NewRecorder()

	allowed := h.allowRate(rec, rateRequest(), rateLimitPolicy{
		scope: "api_ip", limit: 10, window: time.Minute,
		mode: rateLimitProblem,
	}, "203.0.113.9")

	if !allowed {
		t.Fatalf("read request should pass, status = %d", rec.Code)
	}
}

func TestAllowRateFailsClosedOnRepositoryError(t *testing.T) {
	h := newRateHandlers(&fakeRateLimit{err: errors.New("redis is down")})
	rec := httptest.NewRecorder()

	allowed := h.allowRate(rec, rateRequest(), rateLimitPolicy{
		scope: "login_ip", limit: 10, window: time.Minute,
		mode: rateLimitProblem, failClosed: true,
	}, "203.0.113.9")

	if allowed {
		t.Fatal("auth request should fail closed")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestAllowRateWithoutRepositoryHonorsFailMode(t *testing.T) {
	h := newRateHandlers(nil)

	if h.allowRate(httptest.NewRecorder(), rateRequest(), rateLimitPolicy{
		scope: "api_ip", limit: 10, window: time.Minute,
	}, "203.0.113.9") != true {
		t.Fatal("read request should pass without storage")
	}

	rec := httptest.NewRecorder()
	if h.allowRate(rec, rateRequest(), rateLimitPolicy{
		scope: "login_ip", limit: 10, window: time.Minute, failClosed: true,
	}, "203.0.113.9") != false {
		t.Fatal("auth request should fail closed without storage")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestTokenRateKeyScopesPublicClientsByIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/realms/default/token", nil)
	req.RemoteAddr = "203.0.113.9:1234"

	public := &domain.Client{ClientID: "public-client", Type: domain.ClientTypePublic}
	if got := tokenRateKey(public, req); got != "public-client|203.0.113.9" {
		t.Fatalf("public key = %q", got)
	}

	confidential := &domain.Client{ClientID: "backend", Type: domain.ClientTypeConfidential}
	if got := tokenRateKey(confidential, req); got != "backend" {
		t.Fatalf("confidential key = %q", got)
	}
}

func TestIPRateLimitKeysByClientIP(t *testing.T) {
	repo := &fakeRateLimit{decision: domain.RateLimitDecision{Allowed: true}}
	h := newRateHandlers(repo)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := h.ipRateLimit(rateLimitPolicy{
		scope: "api_ip", limit: 10, window: time.Minute,
	})(next)

	handler.ServeHTTP(httptest.NewRecorder(), rateRequest())

	if len(repo.keys) != 1 || repo.keys[0] != "api_ip:203.0.113.9" {
		t.Fatalf("keys = %v", repo.keys)
	}
}
