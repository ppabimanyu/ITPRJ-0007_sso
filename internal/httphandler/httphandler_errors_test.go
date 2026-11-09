package httphandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func TestWriteProblemShape(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	WriteProblem(rec, req, http.StatusNotFound, domain.CodeNotFound, "user not found")

	if got, want := rec.Header().Get("Content-Type"), "application/problem+json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	for key, want := range map[string]any{
		"type":     "about:blank",
		"title":    "Not Found",
		"status":   float64(http.StatusNotFound),
		"detail":   "user not found",
		"instance": "/api/v1/users",
		"code":     domain.CodeNotFound,
	} {
		if got := body[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}

func TestWriteOAuthErrorShape(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteOAuthError(rec, http.StatusBadRequest, "invalid_grant", "code expired", "state-1")

	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	for key, want := range map[string]any{
		"error":             "invalid_grant",
		"error_description": "code expired",
		"state":             "state-1",
	} {
		if got := body[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}
