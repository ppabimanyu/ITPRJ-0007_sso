package httphandler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func (h *handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	limit, ok := queryLimit(w, r)
	if !ok {
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID != "" {
		if _, err := uuid.Parse(userID); err != nil {
			WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
				"user_id must be a UUID")
			return
		}
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	switch status {
	case "", "active", "revoked", "expired":
	default:
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"status must be active, revoked, or expired")
		return
	}
	if h.deps.AdminSessions == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	views, next, err := h.deps.AdminSessions.ListAdmin(r.Context(), current.User.RealmID,
		domain.AdminSessionFilter{
			UserID: userID,
			Status: status,
			After:  strings.TrimSpace(r.URL.Query().Get("after")),
			Limit:  limit,
		})
	if err != nil {
		h.logInternal(r.Context(), "admin_session_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	var nextAfter *string
	if next != "" {
		nextAfter = &next
	}
	writeJSON(w, http.StatusOK, domain.AdminSessionList{Items: views, NextAfter: nextAfter})
}

func (h *handlers) revokeSession(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	raw := chi.URLParam(r, "id")
	if _, err := uuid.Parse(raw); err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "session not found")
		return
	}
	if h.deps.AdminSessions == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	audit := adminAudit(current, domain.AuditSessionRevoked,
		map[string]any{"session_id": raw})
	revoked, err := h.deps.AdminSessions.RevokeForRealm(r.Context(), current.User.RealmID,
		domain.SessionID(raw), audit)
	if err != nil {
		h.logInternal(r.Context(), "admin_session_revoke_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if !revoked {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "session not found")
		return
	}
	writeNoContent(w)
}
