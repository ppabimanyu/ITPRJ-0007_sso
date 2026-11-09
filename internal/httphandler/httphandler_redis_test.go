package httphandler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func TestIntegrationRedisOutageKeepsSessions(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "degrade-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"
	it.createUser(t, realm, email, password, true)

	sessionClient := it.client(t)
	sessionToken := it.csrfToken(t, sessionClient)
	resp, body := it.login(t, sessionClient, sessionToken, email, password)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, body = %s", resp.StatusCode, body)
	}

	fresh := it.client(t)
	freshToken := it.csrfToken(t, fresh)

	if err := it.redis.Close(); err != nil {
		t.Fatalf("close redis: %v", err)
	}

	resp, body = it.request(t, sessionClient, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("existing session = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = it.request(t, sessionClient, http.MethodGet, "/api/v1/me", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("existing profile = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = it.login(t, fresh, freshToken, email, password)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("new login = %d, body = %s", resp.StatusCode, body)
	}
	if code := problemCode(t, body); code != domain.CodeServiceUnavailable {
		t.Fatalf("new login code = %q", code)
	}

	resp, body = it.request(t, sessionClient, http.MethodPost, "/api/v1/auth/logout", nil,
		map[string]string{
			"X-CSRF-Token": it.cookie(t, sessionClient, csrfCookieName),
			"Origin":       "http://localhost:5173",
		})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout = %d, body = %s", resp.StatusCode, body)
	}
}
