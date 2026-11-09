package httphandler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg/password"
)

func (h *handlers) getMe(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	writeJSON(w, http.StatusOK, userDTO(current.User, current.Roles, current.MFAEnrolled))
}

func (h *handlers) updateMe(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	var request domain.ProfileUpdateRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	updated := *current.User
	var fields []FieldError
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if len(name) > 200 {
			fields = append(fields, FieldError{Field: "name", Message: "must be at most 200 characters"})
		}
		updated.Name = name
	}
	if request.GivenName != nil {
		given := strings.TrimSpace(*request.GivenName)
		if len(given) > 100 {
			fields = append(fields, FieldError{Field: "given_name", Message: "must be at most 100 characters"})
		}
		updated.GivenName = given
	}
	if request.FamilyName != nil {
		family := strings.TrimSpace(*request.FamilyName)
		if len(family) > 100 {
			fields = append(fields, FieldError{Field: "family_name", Message: "must be at most 100 characters"})
		}
		updated.FamilyName = family
	}
	if request.Locale != nil {
		locale := strings.TrimSpace(*request.Locale)
		if locale != "en" && locale != "id" {
			fields = append(fields, FieldError{Field: "locale", Message: "must be en or id"})
		}
		updated.Locale = &locale
	}
	if len(fields) > 0 {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"profile does not meet the constraints", fields)
		return
	}
	if err := h.deps.Users.UpdateProfile(r.Context(), &updated); err != nil {
		h.logInternal(r.Context(), "profile_update_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, userDTO(&updated, current.Roles, current.MFAEnrolled))
}

func (h *handlers) changePassword(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	var request domain.ChangePasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.CurrentPassword == "" || request.NewPassword == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"current_password and new_password are required")
		return
	}
	credential, err := h.deps.Credentials.GetByUserID(r.Context(), current.User.ID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"current password is incorrect")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "credential_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	valid, err := password.Verify(request.CurrentPassword, credential.PasswordHash)
	if err != nil || !valid {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"current password is incorrect")
		return
	}
	policy := password.Policy{MinLength: h.deps.Config.Password.MinLength}
	if err := policy.Validate(request.NewPassword, current.User.Email, current.User.Name); err != nil {
		writePasswordPolicyProblem(w, r, err)
		return
	}
	hash, params, ok := h.hashNewPassword(w, r, request.NewPassword)
	if !ok {
		return
	}
	audit := auditEvent(current.User.RealmID, domain.ActorTypeUser, userActor(current.User.ID),
		domain.AuditPasswordChanged, nil)
	if err := h.deps.Credentials.ChangePassword(r.Context(), current.User.ID,
		current.Session.ID, hash, params, audit); err != nil {
		h.logInternal(r.Context(), "change_password_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func (h *handlers) listMySessions(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	sessions, err := h.deps.Sessions.ListByUser(r.Context(), current.User.ID)
	if err != nil {
		h.logInternal(r.Context(), "session_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	items := make([]domain.MeSession, 0, len(sessions))
	for i := range sessions {
		session := &sessions[i]
		items = append(items, domain.MeSession{
			ID:         string(session.ID),
			IP:         optionalString(session.IP),
			UserAgent:  optionalString(session.UserAgent),
			CreatedAt:  session.CreatedAt,
			LastSeenAt: session.LastSeenAt,
			ExpiresAt:  session.AbsoluteExpiresAt,
			Current:    session.ID == current.Session.ID,
		})
	}
	writeJSON(w, http.StatusOK, domain.MeSessionList{Items: items})
}

func (h *handlers) revokeMySession(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	rawID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(rawID); err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "session not found")
		return
	}
	sessionID := domain.SessionID(rawID)
	audit := auditEvent(current.User.RealmID, domain.ActorTypeUser, userActor(current.User.ID),
		domain.AuditSessionRevoked, map[string]any{"session_id": rawID})
	revoked, err := h.deps.Sessions.RevokeForUser(r.Context(), sessionID, current.User.ID, audit)
	if err != nil {
		h.logInternal(r.Context(), "session_revoke_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if !revoked {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "session not found")
		return
	}
	if sessionID == current.Session.ID {
		clearSessionCookie(w, h.deps.Config)
		clearCSRFCookie(w, h.deps.Config)
	}
	writeNoContent(w)
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
