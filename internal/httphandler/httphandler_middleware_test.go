package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/config"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

type fakeSessions struct {
	current *domain.SessionWithUser
}

func (f *fakeSessions) CreateLoginSession(context.Context, *domain.Session, *domain.AuditEvent) error {
	return nil
}

func (f *fakeSessions) GetByTokenHash(context.Context, string) (*domain.SessionWithUser, error) {
	if f.current == nil {
		return nil, domain.ErrNotFound
	}
	return f.current, nil
}

func (f *fakeSessions) GetByID(context.Context, domain.SessionID) (*domain.SessionWithUser, error) {
	if f.current == nil {
		return nil, domain.ErrNotFound
	}
	return f.current, nil
}

func (f *fakeSessions) TouchLastSeen(context.Context, domain.SessionID, time.Time, time.Time) error {
	return nil
}

func (f *fakeSessions) ListByUser(context.Context, domain.UserID) ([]domain.Session, error) {
	return nil, nil
}

func (f *fakeSessions) RevokeForUser(context.Context, domain.SessionID, domain.UserID, *domain.AuditEvent) (bool, error) {
	return true, nil
}

func (f *fakeSessions) RevokeByID(context.Context, domain.SessionID, *domain.AuditEvent) (bool, error) {
	return true, nil
}

func (f *fakeSessions) RevokeAllForUser(context.Context, domain.UserID, *domain.AuditEvent) error {
	return nil
}

func TestAccessLogIncludesSessionUserID(t *testing.T) {
	var buf bytes.Buffer
	base := logger.NewWithWriter(&buf, "info")
	now := time.Now().UTC()
	fake := &fakeSessions{current: &domain.SessionWithUser{
		Session: &domain.Session{
			ID:                "session-1",
			UserID:            "user-123",
			IdleExpiresAt:     now.Add(time.Hour),
			AbsoluteExpiresAt: now.Add(time.Hour),
		},
		User: &domain.User{ID: "user-123", Email: "user@example.com"},
	}}
	cfg := &config.Config{AppEnv: "dev"}
	handler := RequestID(Logger(base)(SessionAuth(Deps{Config: cfg, Sessions: fake})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName(), Value: "raw-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	line := lastJSONLine(t, buf.Bytes())
	if got, _ := line["user_id"].(string); got != "user-123" {
		t.Errorf("access log user_id = %q, want user-123 in %s", got, buf.String())
	}
	if got, _ := line["status"].(float64); got != http.StatusOK {
		t.Errorf("access log status = %v", got)
	}
}

func TestSessionAuthRejectsExpiredSession(t *testing.T) {
	now := time.Now().UTC()
	fake := &fakeSessions{current: &domain.SessionWithUser{
		Session: &domain.Session{
			ID:                "session-1",
			UserID:            "user-123",
			IdleExpiresAt:     now.Add(-time.Minute),
			AbsoluteExpiresAt: now.Add(time.Hour),
		},
		User: &domain.User{ID: "user-123"},
	}}
	cfg := &config.Config{AppEnv: "dev"}
	handler := SessionAuth(Deps{Config: cfg, Sessions: fake})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Error("handler ran for an expired session")
		}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionCookieName(), Value: "raw-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
}

func TestOriginCheckRejectsUnknownOrigin(t *testing.T) {
	cfg := &config.Config{AppEnv: "dev", AllowedOrigins: []string{"http://localhost:5173"}}
	handler := originAllowed(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{}"))
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestOriginCheckAcceptsConfiguredOrigin(t *testing.T) {
	cfg := &config.Config{AppEnv: "dev", AllowedOrigins: []string{"http://localhost:5173"}}
	called := false
	handler := originAllowed(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{}"))
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, called = %v", rec.Code, called)
	}
}

func lastJSONLine(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) == 0 {
		t.Fatal("no log lines")
	}
	var line map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &line); err != nil {
		t.Fatalf("last log line is not JSON: %v (%s)", err, raw)
	}
	return line
}

func TestSecurityHeadersAllowGoogleFonts(t *testing.T) {
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{
		"style-src 'self' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("Content-Security-Policy = %q, missing %q", csp, want)
		}
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}
