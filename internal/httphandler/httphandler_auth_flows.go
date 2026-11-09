package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
	"github.com/ppa/itprj-0007-sso/internal/pkg/password"
)

func (h *handlers) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var request domain.ForgotPasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	email, ok := validEmail(request.Email)
	if !ok {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "email is required")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if h.deps.RateLimit == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"rate limit storage is unavailable")
		return
	}
	decision, err := h.deps.RateLimit.Allow(r.Context(), "forgot_account",
		pkg.SHA256Hex(email), forgotAccountLimit, forgotAccountWindow)
	if err != nil {
		h.logInternal(r.Context(), "rate_limit_failed", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"rate limit storage is unavailable")
		return
	}
	if !decision.Allowed {
		w.Header().Set("Retry-After", retryAfterSeconds(decision.RetryAfter))
		WriteProblem(w, r, http.StatusTooManyRequests, domain.CodeRateLimited,
			"too many reset requests")
		return
	}

	user, lookupErr := h.deps.Users.GetByEmail(r.Context(), realm.ID, email)
	if errors.Is(lookupErr, domain.ErrNotFound) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if lookupErr != nil {
		h.logInternal(r.Context(), "user_lookup_failed", lookupErr)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if user.DisabledAt != nil {
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
		To:       user.Email,
		Name:     user.Name,
		Template: "reset",
		Locale:   localeOf(user),
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
		UserID:    user.ID,
		Purpose:   domain.EmailTokenReset,
		TokenHash: tokenHash,
		ExpiresAt: nowUTC().Add(resetTokenLifetime),
	}
	outbox := &domain.OutboxEvent{
		ID:      domain.OutboxEventID(uuid.NewString()),
		RealmID: realm.ID,
		Type:    domain.OutboxEmailReset,
		Payload: payload,
		Status:  domain.OutboxStatusPending,
	}
	audit := auditEvent(realm.ID, domain.ActorTypeUser, userActor(user.ID),
		domain.AuditPasswordRequested, nil)
	if err := h.deps.EmailTokens.CreatePasswordReset(r.Context(), token, outbox, audit); err != nil {
		h.logInternal(r.Context(), "reset_token_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *handlers) resetPassword(w http.ResponseWriter, r *http.Request) {
	var request domain.ResetPasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Token) == "" || request.NewPassword == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"token and new_password are required")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	tokenHash := pkg.SHA256Hex(request.Token)
	user, err := h.deps.EmailTokens.GetValidUser(r.Context(), realm.ID, tokenHash, domain.EmailTokenReset)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"token is invalid or expired")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "reset_token_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	policy := password.Policy{MinLength: h.deps.Config.Password.MinLength}
	if err := policy.Validate(request.NewPassword, user.Email, user.Name); err != nil {
		writePasswordPolicyProblem(w, r, err)
		return
	}
	hash, params, ok := h.hashNewPassword(w, r, request.NewPassword)
	if !ok {
		return
	}
	audit := auditEvent(realm.ID, domain.ActorTypeUser, userActor(user.ID),
		domain.AuditPasswordReset, nil)
	if err := h.deps.EmailTokens.CompletePasswordReset(r.Context(), realm.ID, tokenHash, hash, params, audit); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
				"token is invalid or expired")
			return
		}
		h.logInternal(r.Context(), "password_reset_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func (h *handlers) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var request domain.VerifyEmailRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Token) == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "token is required")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	tokenHash := pkg.SHA256Hex(request.Token)
	user, err := h.deps.EmailTokens.GetValidUser(r.Context(), realm.ID, tokenHash, domain.EmailTokenVerify)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"token is invalid or expired")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "verify_token_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	audit := auditEvent(realm.ID, domain.ActorTypeUser, userActor(user.ID),
		domain.AuditEmailVerified, nil)
	if err := h.deps.EmailTokens.VerifyEmail(r.Context(), realm.ID, tokenHash, audit); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
				"token is invalid or expired")
			return
		}
		h.logInternal(r.Context(), "verify_email_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func (h *handlers) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var request domain.AcceptInviteRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Token) == "" || request.Password == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"token and password are required")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	tokenHash := pkg.SHA256Hex(request.Token)
	user, err := h.deps.EmailTokens.GetValidUser(r.Context(), realm.ID, tokenHash, domain.EmailTokenInvite)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"token is invalid or expired")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "invite_token_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	policy := password.Policy{MinLength: h.deps.Config.Password.MinLength}
	if err := policy.Validate(request.Password, user.Email, user.Name); err != nil {
		writePasswordPolicyProblem(w, r, err)
		return
	}
	hash, params, ok := h.hashNewPassword(w, r, request.Password)
	if !ok {
		return
	}
	audit := auditEvent(realm.ID, domain.ActorTypeUser, userActor(user.ID),
		domain.AuditInviteAccepted, nil)
	if err := h.deps.EmailTokens.AcceptInvite(r.Context(), realm.ID, tokenHash, hash, params, audit); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
				"token is invalid or expired")
			return
		}
		h.logInternal(r.Context(), "accept_invite_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func (h *handlers) hashNewPassword(w http.ResponseWriter, r *http.Request, plaintext string) (string, []byte, bool) {
	hash, err := password.Hash(plaintext)
	if err != nil {
		h.logInternal(r.Context(), "password_hash_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return "", nil, false
	}
	return hash, []byte(password.ParamsJSON(password.DefaultParams)), true
}

func writePasswordPolicyProblem(w http.ResponseWriter, r *http.Request, err error) {
	var validation *password.ValidationError
	if errors.As(err, &validation) {
		fields := make([]FieldError, 0, len(validation.Errors))
		for _, field := range validation.Errors {
			fields = append(fields, FieldError{Field: field.Field, Message: field.Message})
		}
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"password does not meet the policy", fields)
		return
	}
	logger.FromContext(r.Context()).Error("password_policy_failed", "error", err)
	WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
		"password does not meet the policy")
}

func actionLink(baseURL, path, token string) string {
	return strings.TrimRight(baseURL, "/") + path + "?token=" + url.QueryEscape(token)
}
