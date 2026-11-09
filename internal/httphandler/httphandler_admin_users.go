package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
)

const (
	adminUserEndpoint     = "POST /api/v1/users"
	idempotencyLifetime   = 24 * time.Hour
	maxIdempotencyKeySize = 128
)

type idempotencyPayload struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func (h *handlers) listUsers(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
				"limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	switch status {
	case "", "active", "pending", "disabled":
	default:
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"status must be active, pending, or disabled")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 200 {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"q must be at most 200 characters")
		return
	}
	if h.deps.AdminUsers == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	views, next, err := h.deps.AdminUsers.List(r.Context(), current.User.RealmID,
		domain.AdminUserFilter{
			Query:  query,
			Status: status,
			After:  strings.TrimSpace(r.URL.Query().Get("after")),
			Limit:  limit,
		})
	if err != nil {
		h.logInternal(r.Context(), "admin_user_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	var nextAfter *string
	if next != "" {
		nextAfter = &next
	}
	writeJSON(w, http.StatusOK, domain.AdminUserList{Items: views, NextAfter: nextAfter})
}

func (h *handlers) getUser(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	if h.deps.AdminUsers == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	view, err := h.deps.AdminUsers.Get(r.Context(), current.User.RealmID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "user not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_user_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *handlers) createUser(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	var request domain.CreateUserRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	email, ok := validEmail(request.Email)
	if !ok {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"email is not valid")
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len(name) > 200 {
		WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"name is required and must be at most 200 characters")
		return
	}
	var username *string
	if request.Username != nil {
		value := strings.TrimSpace(*request.Username)
		if len(value) > 100 {
			WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
				"username must be at most 100 characters")
			return
		}
		if value != "" {
			username = &value
		}
	}
	if h.deps.AdminUsers == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}

	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > maxIdempotencyKeySize {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"Idempotency-Key must be at most 128 characters")
		return
	}
	canonical, err := json.Marshal(request)
	if err != nil {
		h.logInternal(r.Context(), "idempotency_hash_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	requestHash := pkg.SHA256Hex(string(canonical))
	if key != "" && h.deps.Idempotency != nil {
		record, err := h.deps.Idempotency.Get(r.Context(), current.User.RealmID,
			adminUserEndpoint, key)
		if err == nil {
			if record.RequestHash != requestHash {
				WriteProblem(w, r, http.StatusConflict, domain.CodeIdempotencyMismatch,
					"Idempotency-Key was already used with a different request")
				return
			}
			var payload idempotencyPayload
			if len(record.Response) > 0 && json.Unmarshal(record.Response, &payload) == nil &&
				payload.Status != 0 && len(payload.Body) > 0 {
				writeRawJSON(w, payload.Status, payload.Body)
				return
			}
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"the idempotent request is still being processed")
			return
		}
		if !errors.Is(err, domain.ErrNotFound) {
			h.logInternal(r.Context(), "idempotency_lookup_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
	}

	user := &domain.User{
		ID:        domain.UserID(uuid.NewString()),
		RealmID:   current.User.RealmID,
		Email:     email,
		Username:  username,
		Name:      name,
		GivenName: "",
	}
	sendInvite := true
	if request.SendInvite != nil {
		sendInvite = *request.SendInvite
	}
	var token *domain.EmailToken
	var outbox *domain.OutboxEvent
	if sendInvite {
		token, outbox, err = h.buildInvite(r, current, user)
		if err != nil {
			h.logInternal(r.Context(), "invite_build_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
	}
	audit := adminAudit(current, domain.AuditAdminUserCreated,
		map[string]any{"target_user_id": string(user.ID), "invited": sendInvite})
	if err := h.deps.AdminUsers.CreateWithInvite(r.Context(), user, token, outbox, audit); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"a user with this email or username already exists")
			return
		}
		h.logInternal(r.Context(), "admin_user_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	view := domain.AdminUserView{UserDTO: userDTO(user, nil, false)}
	body, err := json.Marshal(view)
	if err != nil {
		h.logInternal(r.Context(), "admin_user_encode_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if key != "" && h.deps.Idempotency != nil {
		payload, err := json.Marshal(idempotencyPayload{Status: http.StatusCreated, Body: body})
		if err == nil {
			record := &domain.IdempotencyKey{
				ID:          uuid.NewString(),
				RealmID:     current.User.RealmID,
				Key:         key,
				Endpoint:    adminUserEndpoint,
				RequestHash: requestHash,
				Response:    payload,
				CreatedAt:   nowUTC(),
				ExpiresAt:   nowUTC().Add(idempotencyLifetime),
			}
			if err := h.deps.Idempotency.Save(r.Context(), record); err != nil {
				h.logInternal(r.Context(), "idempotency_save_failed", err)
			}
		}
	}
	writeRawJSON(w, http.StatusCreated, body)
}

func (h *handlers) updateUser(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	var request domain.UpdateUserRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	update := domain.AdminUserUpdate{
		GivenName:  nil,
		FamilyName: nil,
		Locale:     nil,
		Disabled:   request.Disabled,
	}
	var fields []FieldError
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if len(name) > 200 {
			fields = append(fields, FieldError{Field: "name", Message: "must be at most 200 characters"})
		}
		update.Name = &name
	}
	if request.GivenName != nil {
		given := strings.TrimSpace(*request.GivenName)
		if len(given) > 100 {
			fields = append(fields, FieldError{Field: "given_name", Message: "must be at most 100 characters"})
		}
		update.GivenName = &given
	}
	if request.FamilyName != nil {
		family := strings.TrimSpace(*request.FamilyName)
		if len(family) > 100 {
			fields = append(fields, FieldError{Field: "family_name", Message: "must be at most 100 characters"})
		}
		update.FamilyName = &family
	}
	if request.Locale != nil {
		locale := strings.TrimSpace(*request.Locale)
		if locale != "en" && locale != "id" {
			fields = append(fields, FieldError{Field: "locale", Message: "must be en or id"})
		}
		update.Locale = &locale
	}
	if len(fields) > 0 {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"user does not meet the constraints", fields)
		return
	}
	if h.deps.AdminUsers == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	eventType := domain.AuditAdminUserUpdated
	if request.Disabled != nil && *request.Disabled {
		eventType = domain.AuditAdminUserDisabled
	}
	audit := adminAudit(current, eventType, map[string]any{"target_user_id": string(userID)})
	updated, err := h.deps.AdminUsers.Update(r.Context(), current.User.RealmID, userID, update, audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "user not found")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"the last platform administrator cannot be disabled")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_user_update_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	view, err := h.deps.AdminUsers.Get(r.Context(), current.User.RealmID, updated.ID)
	if err != nil {
		h.logInternal(r.Context(), "admin_user_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *handlers) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	if h.deps.AdminUsers == nil || h.deps.EmailTokens == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	view, err := h.deps.AdminUsers.Get(r.Context(), current.User.RealmID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "user not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_user_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if view.DisabledAt != nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	raw, tokenHash, err := csrf.NewToken()
	if err != nil {
		h.logInternal(r.Context(), "reset_token_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	job := domain.MailJob{
		To:       view.Email,
		Name:     stringValue(view.Name),
		Template: "reset",
		Locale:   localeValue(view.Locale),
		Link:     actionLink(h.deps.Config.HTTP.ExternalURL, "/reset-password", raw),
	}
	payload, err := json.Marshal(job)
	if err != nil {
		h.logInternal(r.Context(), "reset_payload_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	token := &domain.EmailToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		Purpose:   domain.EmailTokenReset,
		TokenHash: tokenHash,
		ExpiresAt: nowUTC().Add(resetTokenLifetime),
		CreatedBy: &current.User.ID,
	}
	outbox := &domain.OutboxEvent{
		ID:      domain.OutboxEventID(uuid.NewString()),
		RealmID: current.User.RealmID,
		Type:    domain.OutboxEmailReset,
		Payload: payload,
		Status:  domain.OutboxStatusPending,
	}
	audit := adminAudit(current, domain.AuditAdminPasswordRequest,
		map[string]any{"target_user_id": string(userID)})
	if err := h.deps.EmailTokens.CreatePasswordReset(r.Context(), token, outbox, audit); err != nil {
		h.logInternal(r.Context(), "admin_reset_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *handlers) resendUserInvite(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	if h.deps.AdminUsers == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	view, err := h.deps.AdminUsers.Get(r.Context(), current.User.RealmID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "user not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_user_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if view.DisabledAt != nil {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"a disabled user cannot be invited")
		return
	}
	user := &domain.User{
		ID:      userID,
		RealmID: current.User.RealmID,
		Email:   view.Email,
		Name:    stringValue(view.Name),
		Locale:  view.Locale,
	}
	token, outbox, err := h.buildInvite(r, current, user)
	if err != nil {
		h.logInternal(r.Context(), "invite_build_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	audit := adminAudit(current, domain.AuditAdminUserInvited,
		map[string]any{"target_user_id": string(userID)})
	if err := h.deps.AdminUsers.EnqueueInvite(r.Context(), token, outbox, audit); err != nil {
		h.logInternal(r.Context(), "admin_invite_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *handlers) assignUserRoles(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	userID, ok := pathUserID(w, r)
	if !ok {
		return
	}
	var request domain.AssignRolesRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	seen := make(map[string]struct{}, len(request.RoleIDs))
	roleIDs := make([]domain.RoleID, 0, len(request.RoleIDs))
	for _, raw := range request.RoleIDs {
		if _, err := uuid.Parse(raw); err != nil {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
				"role_ids must contain UUIDs")
			return
		}
		if _, exists := seen[raw]; exists {
			continue
		}
		seen[raw] = struct{}{}
		roleIDs = append(roleIDs, domain.RoleID(raw))
	}
	if h.deps.AdminUsers == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	audit := adminAudit(current, domain.AuditAdminUserRoles,
		map[string]any{"target_user_id": string(userID), "role_count": len(roleIDs)})
	roles, err := h.deps.AdminUsers.ReplaceRoles(r.Context(), current.User.RealmID,
		userID, roleIDs, audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound,
			"user or role not found")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"the last platform administrator cannot lose the role")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_roles_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	refs := make([]domain.RoleRef, 0, len(roles))
	for _, role := range roles {
		refs = append(refs, domain.RoleRef{ID: string(role.ID), Name: role.Name})
	}
	writeJSON(w, http.StatusOK, domain.RolesResponse{Roles: refs})
}

func (h *handlers) buildInvite(r *http.Request, current *CurrentSession, user *domain.User) (*domain.EmailToken, *domain.OutboxEvent, error) {
	raw, tokenHash, err := csrf.NewToken()
	if err != nil {
		return nil, nil, err
	}
	job := domain.MailJob{
		To:       user.Email,
		Name:     user.Name,
		Template: "invite",
		Locale:   localeOf(user),
		Link:     actionLink(h.deps.Config.HTTP.ExternalURL, "/accept-invite", raw),
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, nil, err
	}
	return &domain.EmailToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Purpose:   domain.EmailTokenInvite,
		TokenHash: tokenHash,
		ExpiresAt: nowUTC().Add(inviteTokenLifetime),
		CreatedBy: &current.User.ID,
	}, &domain.OutboxEvent{
		ID:      domain.OutboxEventID(uuid.NewString()),
		RealmID: current.User.RealmID,
		Type:    domain.OutboxEmailInvite,
		Payload: payload,
		Status:  domain.OutboxStatusPending,
	}, nil
}

func pathUserID(w http.ResponseWriter, r *http.Request) (domain.UserID, bool) {
	raw := chi.URLParam(r, "id")
	if _, err := uuid.Parse(raw); err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "user not found")
		return "", false
	}
	return domain.UserID(raw), true
}

func adminAudit(current *CurrentSession, eventType string, metadata map[string]any) *domain.AuditEvent {
	return auditEvent(current.User.RealmID, domain.ActorTypeAdmin,
		userActor(current.User.ID), eventType, metadata)
}

func writeRawJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func localeValue(locale *string) string {
	if locale == nil {
		return "en"
	}
	switch *locale {
	case "id":
		return "id"
	default:
		return "en"
	}
}
