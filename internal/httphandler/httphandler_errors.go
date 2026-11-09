package httphandler

import (
	"encoding/json"
	"net/http"
)

// FieldError names one rejected request field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem is the RFC 9457 body used by REST and admin endpoints. Code carries
// the stable internal error code from domain/const_error.go.
type Problem struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Instance string       `json:"instance,omitempty"`
	Code     string       `json:"code,omitempty"`
	Errors   []FieldError `json:"errors,omitempty"`
}

// WriteProblem renders application/problem+json.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string) {
	writeProblem(w, Problem{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
		Code:     code,
	})
}

// WriteValidationProblem renders 422 with per-field errors.
func WriteValidationProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, fields []FieldError) {
	writeProblem(w, Problem{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
		Code:     code,
		Errors:   fields,
	})
}

func writeProblem(w http.ResponseWriter, problem Problem) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}

// OAuthError is the RFC 6749 error shape used by OIDC endpoints.
type OAuthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	State            string `json:"state,omitempty"`
}

// WriteOAuthError renders an OAuth error as application/json.
func WriteOAuthError(w http.ResponseWriter, status int, code, description, state string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(OAuthError{
		Error:            code,
		ErrorDescription: description,
		State:            state,
	})
}
