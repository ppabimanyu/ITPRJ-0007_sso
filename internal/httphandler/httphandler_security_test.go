package httphandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{
		"default-src 'self'", "script-src 'self'", "object-src 'none'",
		"frame-ancestors 'none'", "base-uri 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP = %q, want %q", csp, want)
		}
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	if rec.Header().Get("Permissions-Policy") == "" {
		t.Error("Permissions-Policy is missing")
	}
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("HSTS is missing in production")
	}
}

func TestSecurityHeadersDevOmitsHSTS(t *testing.T) {
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS in dev = %q, want empty", got)
	}
}

func TestIntegrationAdminOriginRequired(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	adminEmail := "admin-origin-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, adminEmail, "correct horse battery staple")
	admin, headers := it.adminSession(t, adminEmail, "correct horse battery staple")

	resp, _ := it.request(t, admin, http.MethodGet, "/api/v1/version", nil, nil)
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("API response is missing the CSP header")
	}

	name := "origin-role-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	payload := domain.CreateRoleRequest{Name: name}

	resp, body := it.request(t, admin, http.MethodPost, "/api/v1/roles", payload, map[string]string{
		"X-CSRF-Token": headers["X-CSRF-Token"],
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing origin = %d, body = %s", resp.StatusCode, body)
	}

	badHeaders := map[string]string{
		"X-CSRF-Token": headers["X-CSRF-Token"],
		"Origin":       "https://evil.example",
	}
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/roles", payload, badHeaders)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bad origin = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/roles", payload, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("allowed origin = %d, body = %s", resp.StatusCode, body)
	}
}
