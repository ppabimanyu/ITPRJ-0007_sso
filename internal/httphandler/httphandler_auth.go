package httphandler

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
)

func (h *handlers) csrfToken(w http.ResponseWriter, r *http.Request) {
	raw, err := csrf.Issue(r.Context(), h.deps.CSRFStore, csrf.PreSessionTTL)
	if err != nil {
		h.logInternal(r.Context(), "csrf_issue_failed", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"csrf storage is unavailable")
		return
	}
	expires := time.Now().UTC().Add(csrf.PreSessionTTL)
	setCSRFCookie(w, h.deps.Config, raw, expires)
	writeJSON(w, http.StatusOK, domain.CSRFResponse{CSRFToken: raw})
}

func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	var request domain.LoginRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	email, ok := validEmail(request.Email)
	if !ok || request.Password == "" || len(request.Password) > 1024 {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"email and password are required")
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

	ipDecision, err := h.deps.RateLimit.Allow(r.Context(), "login_ip", clientIP(r), h.loginIPLimit(), loginIPWindow)
	if err != nil {
		h.logInternal(r.Context(), "rate_limit_failed", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"rate limit storage is unavailable")
		return
	}
	if !ipDecision.Allowed {
		w.Header().Set("Retry-After", retryAfterSeconds(ipDecision.RetryAfter))
		WriteProblem(w, r, http.StatusTooManyRequests, domain.CodeRateLimited,
			"too many login attempts")
		return
	}

	user, lookupErr := h.deps.Users.GetByEmail(r.Context(), realm.ID, email)
	if errors.Is(lookupErr, domain.ErrNotFound) {
		accountDecision, limitErr := h.deps.RateLimit.Allow(r.Context(), "login_account",
			pkg.SHA256Hex(email), h.loginAccountLimit(), loginAccountWindow)
		if limitErr != nil {
			h.logInternal(r.Context(), "rate_limit_failed", limitErr)
			WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
				"rate limit storage is unavailable")
			return
		}
		if !accountDecision.Allowed {
			w.Header().Set("Retry-After", retryAfterSeconds(accountDecision.RetryAfter))
			WriteProblem(w, r, http.StatusTooManyRequests, domain.CodeRateLimited,
				"too many login attempts")
			return
		}
		h.verifyDummyPassword(r.Context(), request.Password)
		h.recordLoginFailure(r, realm, nil, "invalid_credentials", false)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"invalid email or password")
		return
	}
	if lookupErr != nil {
		h.logInternal(r.Context(), "user_lookup_failed", lookupErr)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}

	credential, err := h.deps.Credentials.GetByUserID(r.Context(), user.ID)
	if errors.Is(err, domain.ErrNotFound) {
		logger.FromContext(r.Context()).Warn("login_rejected", "reason", "credential_missing")
		h.verifyDummyPassword(r.Context(), request.Password)
		h.recordLoginFailure(r, realm, user, "invalid_credentials", false)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"invalid email or password")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "credential_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}

	now := nowUTC()
	if credential.LockedUntil != nil && credential.LockedUntil.After(now) {
		w.Header().Set("Retry-After", retryAfterSeconds(credential.LockedUntil.Sub(now)))
		h.recordLoginFailure(r, realm, user, "account_locked", true)
		WriteProblem(w, r, http.StatusLocked, domain.CodeAccountLocked,
			"account is temporarily locked")
		return
	}

	accountDecision, err := h.deps.RateLimit.Allow(r.Context(), "login_account",
		pkg.SHA256Hex(email), h.loginAccountLimit(), loginAccountWindow)
	if err != nil {
		h.logInternal(r.Context(), "rate_limit_failed", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"rate limit storage is unavailable")
		return
	}
	if !accountDecision.Allowed {
		w.Header().Set("Retry-After", retryAfterSeconds(accountDecision.RetryAfter))
		WriteProblem(w, r, http.StatusTooManyRequests, domain.CodeRateLimited,
			"too many login attempts")
		return
	}

	valid, err := h.verifyPassword(r.Context(), request.Password, credential.PasswordHash)
	if err != nil {
		h.logInternal(r.Context(), "password_verify_failed", err)
		h.recordLoginFailure(r, realm, user, "invalid_credentials", false)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"invalid email or password")
		return
	}
	if !valid {
		logger.FromContext(r.Context()).Warn("login_rejected", "reason", "password_mismatch")
		_, lockedUntil, recordErr := h.deps.Credentials.RecordFailedLogin(
			r.Context(), user.ID, now, lockoutThreshold, lockoutWindow)
		if recordErr != nil {
			h.logInternal(r.Context(), "lockout_write_failed", recordErr)
		}
		if lockedUntil != nil && lockedUntil.After(now) {
			w.Header().Set("Retry-After", retryAfterSeconds(lockedUntil.Sub(now)))
			h.recordLoginFailure(r, realm, user, "account_locked", true)
			WriteProblem(w, r, http.StatusLocked, domain.CodeAccountLocked,
				"account is temporarily locked")
			return
		}
		h.recordLoginFailure(r, realm, user, "invalid_credentials", false)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"invalid email or password")
		return
	}

	if user.DisabledAt != nil {
		logger.FromContext(r.Context()).Warn("login_rejected", "reason", "disabled")
		h.recordLoginFailure(r, realm, user, "disabled", false)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"invalid email or password")
		return
	}
	if user.EmailVerifiedAt == nil {
		logger.FromContext(r.Context()).Warn("login_rejected", "reason", "unverified")
		h.recordLoginFailure(r, realm, user, "unverified", false)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeInvalidCredentials,
			"invalid email or password")
		return
	}

	roles, err := h.deps.Roles.ListByUser(r.Context(), realm.ID, user.ID)
	if err != nil {
		h.logInternal(r.Context(), "role_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	mfaEnrolled, err := h.deps.Users.MFAEnrolled(r.Context(), user.ID)
	if err != nil {
		h.logInternal(r.Context(), "mfa_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}

	admin := isAdmin(roles)
	if mfaEnrolled {
		if h.deps.MFAChallenges == nil {
			WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
				"mfa storage is unavailable")
			return
		}
		token, err := pkg.RandomToken(32)
		if err != nil {
			h.logInternal(r.Context(), "mfa_token_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		challenge := &domain.MFAChallenge{
			UserID:     user.ID,
			RealmID:    realm.ID,
			RequestID:  request.RequestID,
			RememberMe: request.RememberMe,
			ExpiresAt:  now.Add(mfaChallengeTTL),
		}
		if err := h.deps.MFAChallenges.Create(r.Context(), token, challenge); err != nil {
			h.logInternal(r.Context(), "mfa_challenge_create_failed", err)
			WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
				"mfa storage is unavailable")
			return
		}
		if h.deps.Audit != nil {
			event := auditEvent(realm.ID, domain.ActorTypeUser, userActor(user.ID),
				domain.AuditMFAChallengeCreated, nil)
			if err := h.deps.Audit.Insert(r.Context(), event); err != nil {
				logger.FromContext(r.Context()).Warn("audit_insert_failed", "error", err)
			}
		}
		writeJSON(w, http.StatusOK, domain.LoginResponse{
			Status:           domain.LoginStatusMFARequired,
			MFAToken:         &token,
			SessionExpiresAt: nil,
			User:             nil,
			RedirectTo:       nil,
		})
		return
	}

	loginStatus := domain.LoginStatusAuthenticated
	if admin {
		loginStatus = domain.LoginStatusMFAEnrollmentRequired
	}
	remember := request.RememberMe && !admin
	idleExpiresAt, absoluteExpiresAt := h.realmSessionWindow(r.Context(), realm.ID, now, admin, remember)
	rawSession, sessionHash, err := csrf.NewToken()
	if err != nil {
		h.logInternal(r.Context(), "session_token_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	rawCSRF, csrfHash, err := csrf.NewToken()
	if err != nil {
		h.logInternal(r.Context(), "csrf_token_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}

	session := &domain.Session{
		ID:                domain.SessionID(uuid.NewString()),
		RealmID:           realm.ID,
		UserID:            user.ID,
		TokenHash:         sessionHash,
		CSRFTokenHash:     csrfHash,
		IP:                clientIP(r),
		UserAgent:         r.UserAgent(),
		AMR:               []string{domain.AMRPassword},
		ACR:               domain.ACRPassword,
		AuthTime:          now,
		LastSeenAt:        now,
		RememberMe:        remember,
		IdleExpiresAt:     idleExpiresAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
	}
	audit := auditEvent(realm.ID, domain.ActorTypeUser, userActor(user.ID),
		domain.AuditLoginSuccess, map[string]any{
			"amr":         session.AMR,
			"remember_me": remember,
		})
	if err := h.deps.Sessions.CreateLoginSession(r.Context(), session, audit); err != nil {
		h.logInternal(r.Context(), "session_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}

	if err := csrf.Consume(r.Context(), h.deps.CSRFStore, r.Header.Get("X-CSRF-Token")); err != nil {
		logger.FromContext(r.Context()).Warn("csrf_consume_failed", "error", err)
	}
	setSessionCookie(w, h.deps.Config, rawSession, remember, absoluteExpiresAt)
	csrfExpiry := time.Time{}
	if remember {
		csrfExpiry = absoluteExpiresAt
	}
	setCSRFCookie(w, h.deps.Config, rawCSRF, csrfExpiry)
	logger.SetUserID(r.Context(), string(user.ID))

	expires := absoluteExpiresAt
	writeJSON(w, http.StatusOK, domain.LoginResponse{
		Status:           loginStatus,
		SessionExpiresAt: &expires,
		User:             ptrUserDTO(userDTO(user, roles, mfaEnrolled)),
		RedirectTo:       nil,
	})
}

func (h *handlers) sessionInfo(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	writeJSON(w, http.StatusOK, domain.SessionInfo{
		User:          userDTO(current.User, current.Roles, current.MFAEnrolled),
		ExpiresAt:     current.Session.AbsoluteExpiresAt,
		IdleExpiresAt: current.Session.IdleExpiresAt,
		MFAEnrolled:   current.MFAEnrolled,
		IsAdmin:       isAdmin(current.Roles),
	})
}

func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	audit := auditEvent(current.User.RealmID, domain.ActorTypeUser, userActor(current.User.ID),
		domain.AuditLogout, nil)
	if _, err := h.deps.Sessions.RevokeForUser(r.Context(), current.Session.ID, current.User.ID, audit); err != nil {
		h.logInternal(r.Context(), "session_revoke_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	clearSessionCookie(w, h.deps.Config)
	clearCSRFCookie(w, h.deps.Config)
	writeNoContent(w)
}

func (h *handlers) recordLoginFailure(r *http.Request, realm *domain.Realm, user *domain.User, reason string, locked bool) {
	if h.deps.Audit == nil {
		return
	}
	var actorID *string
	actorType := domain.ActorTypeSystem
	if user != nil {
		actorID = userActor(user.ID)
		actorType = domain.ActorTypeUser
	}
	event := auditEvent(realm.ID, actorType, actorID, domain.AuditLoginFailure, map[string]any{
		"reason": reason,
		"locked": locked,
	})
	if err := h.deps.Audit.Insert(r.Context(), event); err != nil {
		logger.FromContext(r.Context()).Warn("audit_insert_failed", "error", err)
	}
}

func validEmail(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > 320 {
		return "", false
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil || !strings.EqualFold(address.Address, trimmed) {
		return "", false
	}
	return normalizedEmail(trimmed), true
}

func ptrUserDTO(value domain.UserDTO) *domain.UserDTO { return &value }
