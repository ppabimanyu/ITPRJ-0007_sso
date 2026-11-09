package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

const adminRoleEndpoint = "POST /api/v1/roles"

var roleNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)

func (h *handlers) listRoles(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	if h.deps.Roles == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	roles, err := h.deps.Roles.ListByRealm(r.Context(), current.User.RealmID)
	if err != nil {
		h.logInternal(r.Context(), "admin_role_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	items := make([]domain.RoleView, 0, len(roles))
	for _, role := range roles {
		items = append(items, roleView(role))
	}
	writeJSON(w, http.StatusOK, domain.RoleList{Items: items})
}

func (h *handlers) createRole(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	var request domain.CreateRoleRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	name := strings.TrimSpace(request.Name)
	if !roleNamePattern.MatchString(name) {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"name must match ^[a-z0-9][a-z0-9._-]{1,63}$")
		return
	}
	description := ""
	if request.Description != nil {
		description = strings.TrimSpace(*request.Description)
		if len(description) > 500 {
			WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
				"description must be at most 500 characters")
			return
		}
	}
	if h.deps.AdminRoles == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	requestHash, proceed := h.checkIdempotency(w, r, current.User.RealmID,
		adminRoleEndpoint, key, request)
	if !proceed {
		return
	}
	role := &domain.Role{
		ID:          domain.RoleID(uuid.NewString()),
		RealmID:     current.User.RealmID,
		Name:        name,
		Description: description,
	}
	audit := adminAudit(current, domain.AuditAdminRoleCreated,
		map[string]any{"role_id": string(role.ID), "name": name})
	if err := h.deps.AdminRoles.CreateWithAudit(r.Context(), role, audit); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"a role with this name already exists")
			return
		}
		h.logInternal(r.Context(), "admin_role_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	body, err := json.Marshal(roleView(*role))
	if err != nil {
		h.logInternal(r.Context(), "admin_role_encode_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	h.saveIdempotency(r.Context(), current.User.RealmID, adminRoleEndpoint, key,
		requestHash, http.StatusCreated, body)
	writeRawJSON(w, http.StatusCreated, body)
}

func roleView(role domain.Role) domain.RoleView {
	view := domain.RoleView{
		ID:       string(role.ID),
		Name:     role.Name,
		IsSystem: strings.EqualFold(role.Name, domain.PlatformAdminRole),
	}
	if role.Description != "" {
		description := role.Description
		view.Description = &description
	}
	return view
}
