package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

const adminGroupEndpoint = "POST /api/v1/groups"

func (h *handlers) listGroups(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	groups, err := h.deps.Groups.List(r.Context(), current.User.RealmID)
	if err != nil {
		h.logInternal(r.Context(), "admin_group_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, domain.GroupList{Items: groups})
}

func (h *handlers) getGroup(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	groupID, ok := adminGroupPathID(w, r)
	if !ok {
		return
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	group, err := h.deps.Groups.Get(r.Context(), current.User.RealmID, groupID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "group not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_group_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (h *handlers) createGroup(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	var request domain.CreateGroupRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	name, description, fields := validGroupFields(request.Name, request.Description)
	if len(fields) > 0 {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"group does not meet the constraints", fields)
		return
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	requestHash, proceed := h.checkIdempotency(w, r, current.User.RealmID,
		adminGroupEndpoint, key, request)
	if !proceed {
		return
	}
	group := &domain.Group{
		ID:          domain.GroupID(uuid.NewString()),
		RealmID:     current.User.RealmID,
		Name:        name,
		Description: description,
	}
	audit := adminAudit(current, domain.AuditAdminGroupCreated,
		map[string]any{"group_id": string(group.ID), "name": name})
	if err := h.deps.Groups.Create(r.Context(), group, audit); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"a group with this name already exists")
			return
		}
		h.logInternal(r.Context(), "admin_group_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	view, err := h.deps.Groups.Get(r.Context(), current.User.RealmID, group.ID)
	if err != nil {
		h.logInternal(r.Context(), "admin_group_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	body, err := json.Marshal(view)
	if err != nil {
		h.logInternal(r.Context(), "admin_group_encode_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	h.saveIdempotency(r.Context(), current.User.RealmID, adminGroupEndpoint, key,
		requestHash, http.StatusCreated, body)
	writeRawJSON(w, http.StatusCreated, body)
}

func (h *handlers) updateGroup(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	groupID, ok := adminGroupPathID(w, r)
	if !ok {
		return
	}
	var request domain.UpdateGroupRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	update := domain.GroupUpdate{Description: request.Description}
	var fields []FieldError
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if len(name) < 2 || len(name) > 100 {
			fields = append(fields, FieldError{Field: "name", Message: "must be between 2 and 100 characters"})
		}
		update.Name = &name
	}
	if request.Description.Set && request.Description.Value != nil {
		description := strings.TrimSpace(*request.Description.Value)
		if len(description) > 500 {
			fields = append(fields, FieldError{Field: "description", Message: "must be at most 500 characters"})
		}
		update.Description = domain.Optional[string]{Set: true, Value: &description}
	}
	if len(fields) > 0 {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"group does not meet the constraints", fields)
		return
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	audit := adminAudit(current, domain.AuditAdminGroupUpdated,
		map[string]any{"group_id": string(groupID)})
	group, err := h.deps.Groups.Update(r.Context(), current.User.RealmID, groupID, update, audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "group not found")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"a group with this name already exists")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_group_update_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	view, err := h.deps.Groups.Get(r.Context(), current.User.RealmID, group.ID)
	if err != nil {
		h.logInternal(r.Context(), "admin_group_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *handlers) deleteGroup(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	groupID, ok := adminGroupPathID(w, r)
	if !ok {
		return
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	audit := adminAudit(current, domain.AuditAdminGroupDeleted,
		map[string]any{"group_id": string(groupID)})
	err := h.deps.Groups.Delete(r.Context(), current.User.RealmID, groupID, audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "group not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_group_delete_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func (h *handlers) listGroupMembers(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	groupID, ok := adminGroupPathID(w, r)
	if !ok {
		return
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	members, err := h.deps.Groups.ListMembers(r.Context(), current.User.RealmID, groupID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "group not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_group_member_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, domain.GroupMemberList{Items: members})
}

func (h *handlers) addGroupMembers(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	groupID, ok := adminGroupPathID(w, r)
	if !ok {
		return
	}
	var request domain.AddGroupMembersRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if len(request.UserIDs) == 0 {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"user_ids must contain at least one user")
		return
	}
	userIDs := make([]domain.UserID, 0, len(request.UserIDs))
	seen := make(map[string]struct{}, len(request.UserIDs))
	for _, raw := range request.UserIDs {
		if _, err := uuid.Parse(raw); err != nil {
			WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
				"user_ids must contain UUIDs")
			return
		}
		if _, exists := seen[raw]; exists {
			continue
		}
		seen[raw] = struct{}{}
		userIDs = append(userIDs, domain.UserID(raw))
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	audit := adminAudit(current, domain.AuditAdminGroupMembersAdded, map[string]any{
		"group_id": string(groupID),
		"count":    len(userIDs),
	})
	if err := h.deps.Groups.AddMembers(r.Context(), current.User.RealmID, groupID, userIDs, audit); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound,
				"group or user not found")
			return
		}
		h.logInternal(r.Context(), "admin_group_members_add_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	members, err := h.deps.Groups.ListMembers(r.Context(), current.User.RealmID, groupID)
	if err != nil {
		h.logInternal(r.Context(), "admin_group_member_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, domain.GroupMemberList{Items: members})
}

func (h *handlers) removeGroupMember(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	groupID, ok := adminGroupPathID(w, r)
	if !ok {
		return
	}
	rawUserID := chi.URLParam(r, "user_id")
	if _, err := uuid.Parse(rawUserID); err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "member not found")
		return
	}
	if h.deps.Groups == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	audit := adminAudit(current, domain.AuditAdminGroupMemberRemoved, map[string]any{
		"group_id": string(groupID),
		"user_id":  rawUserID,
	})
	err := h.deps.Groups.RemoveMember(r.Context(), current.User.RealmID, groupID,
		domain.UserID(rawUserID), audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "group or member not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_group_member_remove_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func adminGroupPathID(w http.ResponseWriter, r *http.Request) (domain.GroupID, bool) {
	raw := chi.URLParam(r, "id")
	if _, err := uuid.Parse(raw); err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "group not found")
		return "", false
	}
	return domain.GroupID(raw), true
}

func validGroupFields(rawName string, description *string) (string, string, []FieldError) {
	name := strings.TrimSpace(rawName)
	var fields []FieldError
	if len(name) < 2 || len(name) > 100 {
		fields = append(fields, FieldError{Field: "name", Message: "must be between 2 and 100 characters"})
	}
	value := ""
	if description != nil {
		value = strings.TrimSpace(*description)
		if len(value) > 500 {
			fields = append(fields, FieldError{Field: "description", Message: "must be at most 500 characters"})
		}
	}
	return name, value, fields
}
