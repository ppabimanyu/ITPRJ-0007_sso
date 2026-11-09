package httphandler

import (
	"net/http"
	"strings"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func (h *handlers) listAuditEvents(w http.ResponseWriter, r *http.Request) {
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
	from, ok := parseAuditTime(w, r, "from")
	if !ok {
		return
	}
	to, ok := parseAuditTime(w, r, "to")
	if !ok {
		return
	}
	if from != nil && to != nil && from.After(*to) {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"from must not be after to")
		return
	}
	eventType := strings.TrimSpace(r.URL.Query().Get("type"))
	if len(eventType) > 100 {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"type must be at most 100 characters")
		return
	}
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	if len(actorID) > 100 {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"actor_id must be at most 100 characters")
		return
	}
	if h.deps.AdminAudit == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	events, next, err := h.deps.AdminAudit.ListFiltered(r.Context(), current.User.RealmID,
		domain.AuditFilter{
			From:    from,
			To:      to,
			Type:    eventType,
			ActorID: actorID,
			After:   strings.TrimSpace(r.URL.Query().Get("after")),
			Limit:   limit,
		})
	if err != nil {
		h.logInternal(r.Context(), "admin_audit_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	var nextAfter *string
	if next != "" {
		nextAfter = &next
	}
	writeJSON(w, http.StatusOK, domain.AuditList{Items: events, NextAfter: nextAfter})
}

func parseAuditTime(w http.ResponseWriter, r *http.Request, name string) (*time.Time, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			name+" must be an RFC 3339 timestamp")
		return nil, false
	}
	value := parsed.UTC()
	return &value, true
}
