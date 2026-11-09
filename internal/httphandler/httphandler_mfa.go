package httphandler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
	"github.com/ppa/itprj-0007-sso/internal/pkg/totp"
)

const (
	mfaIssuer       = "ITPRJ SSO"
	mfaMaxAttempts  = 5
	mfaChallengeTTL = 5 * time.Minute
)

func (h *handlers) verifyMFA(w http.ResponseWriter, r *http.Request) {
	var request domain.MFAVerifyRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	token := strings.TrimSpace(request.MFAToken)
	if token == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "mfa_token is required")
		return
	}
	if (request.Code == "") == (request.RecoveryCode == "") {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"exactly one of code or recovery_code is required")
		return
	}
	if h.deps.MFAChallenges == nil || h.deps.MFA == nil || h.deps.Keyring == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"mfa storage is unavailable")
		return
	}

	challenge, err := h.deps.MFAChallenges.Get(r.Context(), token)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeMFAInvalid,
			"the mfa challenge is invalid or expired")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "mfa_challenge_lookup_failed", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"mfa storage is unavailable")
		return
	}
	if challenge.Attempts >= mfaMaxAttempts {
		_ = h.deps.MFAChallenges.Delete(r.Context(), token)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeMFAInvalid,
			"the mfa challenge is invalid or expired")
		return
	}

	user, err := h.deps.Users.GetByID(r.Context(), challenge.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		_ = h.deps.MFAChallenges.Delete(r.Context(), token)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeMFAInvalid,
			"the mfa challenge is invalid or expired")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "mfa_user_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if user.DisabledAt != nil {
		_ = h.deps.MFAChallenges.Delete(r.Context(), token)
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeMFAInvalid,
			"the mfa challenge is invalid or expired")
		return
	}

	record, err := h.deps.MFA.GetTOTP(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.failMFA(w, r, token, user, "totp_missing")
			return
		}
		h.logInternal(r.Context(), "mfa_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if record.ConfirmedAt == nil {
		h.failMFA(w, r, token, user, "totp_unconfirmed")
		return
	}

	recoveryUsed := false
	if request.Code != "" {
		valid, err := h.verifyTOTPCode(r, record, request.Code)
		if err != nil {
			h.logInternal(r.Context(), "totp_verify_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		if !valid {
			h.failMFA(w, r, token, user, "totp_invalid")
			return
		}
	} else {
		audit := auditEvent(user.RealmID, domain.ActorTypeUser, userActor(user.ID),
			domain.AuditMFARecoveryUsed, nil)
		used, err := h.deps.MFA.UseRecoveryCode(r.Context(), user.ID,
			totp.RecoveryCodeHash(request.RecoveryCode), audit)
		if err != nil {
			h.logInternal(r.Context(), "recovery_code_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		if !used {
			h.failMFA(w, r, token, user, "recovery_invalid")
			return
		}
		recoveryUsed = true
	}

	roles, err := h.deps.Roles.ListByUser(r.Context(), user.RealmID, user.ID)
	if err != nil {
		h.logInternal(r.Context(), "role_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	expires, err := h.createMFASession(w, r, user, roles, challenge)
	if err != nil {
		h.logInternal(r.Context(), "mfa_session_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if err := h.deps.MFAChallenges.Delete(r.Context(), token); err != nil {
		logger.FromContext(r.Context()).Warn("mfa_challenge_delete_failed", "error", err)
	}
	if err := csrf.Consume(r.Context(), h.deps.CSRFStore, r.Header.Get("X-CSRF-Token")); err != nil {
		logger.FromContext(r.Context()).Warn("csrf_consume_failed", "error", err)
	}

	metadata := map[string]any{"recovery_code": recoveryUsed}
	if h.deps.Audit != nil {
		event := auditEvent(user.RealmID, domain.ActorTypeUser, userActor(user.ID),
			domain.AuditMFAVerified, metadata)
		if err := h.deps.Audit.Insert(r.Context(), event); err != nil {
			logger.FromContext(r.Context()).Warn("audit_insert_failed", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, domain.MFAVerifyResponse{
		Status:           domain.LoginStatusAuthenticated,
		SessionExpiresAt: expires,
		User:             userDTO(user, roles, true),
	})
}

func (h *handlers) verifyTOTPCode(r *http.Request, record *domain.MFATOTP, code string) (bool, error) {
	secret, err := h.deps.Keyring.Decrypt(record.SecretEncrypted)
	if err != nil {
		return false, err
	}
	matched, ok := totp.Verify(string(secret), strings.TrimSpace(code), nowUTC())
	if !ok || !totp.AcceptStep(record.LastUsedStep, matched) {
		return false, nil
	}
	return h.deps.MFA.ClaimTOTPStep(r.Context(), record.UserID, matched)
}

func (h *handlers) failMFA(w http.ResponseWriter, r *http.Request, token string, user *domain.User, reason string) {
	attempts, err := h.deps.MFAChallenges.RecordFailure(r.Context(), token)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		logger.FromContext(r.Context()).Warn("mfa_attempt_write_failed", "error", err)
	}
	if attempts >= mfaMaxAttempts {
		if err := h.deps.MFAChallenges.Delete(r.Context(), token); err != nil {
			logger.FromContext(r.Context()).Warn("mfa_challenge_delete_failed", "error", err)
		}
	}
	if h.deps.Audit != nil {
		event := auditEvent(user.RealmID, domain.ActorTypeUser, userActor(user.ID),
			domain.AuditMFAFailed, map[string]any{"reason": reason, "attempts": attempts})
		if err := h.deps.Audit.Insert(r.Context(), event); err != nil {
			logger.FromContext(r.Context()).Warn("audit_insert_failed", "error", err)
		}
	}
	WriteProblem(w, r, http.StatusUnauthorized, domain.CodeMFAInvalid,
		"the mfa code is invalid")
}

func (h *handlers) createMFASession(w http.ResponseWriter, r *http.Request, user *domain.User, roles []domain.Role, challenge *domain.MFAChallenge) (time.Time, error) {
	now := nowUTC()
	admin := isAdmin(roles)
	remember := challenge.RememberMe && !admin
	idleExpiresAt, absoluteExpiresAt := h.realmSessionWindow(r.Context(), user.RealmID, now, admin, remember)
	rawSession, sessionHash, err := csrf.NewToken()
	if err != nil {
		return time.Time{}, err
	}
	rawCSRF, csrfHash, err := csrf.NewToken()
	if err != nil {
		return time.Time{}, err
	}
	session := &domain.Session{
		ID:                domain.SessionID(uuid.NewString()),
		RealmID:           user.RealmID,
		UserID:            user.ID,
		TokenHash:         sessionHash,
		CSRFTokenHash:     csrfHash,
		IP:                clientIP(r),
		UserAgent:         r.UserAgent(),
		AMR:               []string{domain.AMRPassword, domain.AMROTP},
		ACR:               domain.ACRMFA,
		AuthTime:          now,
		LastSeenAt:        now,
		RememberMe:        remember,
		IdleExpiresAt:     idleExpiresAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
	}
	audit := auditEvent(user.RealmID, domain.ActorTypeUser, userActor(user.ID),
		domain.AuditLoginSuccess, map[string]any{
			"amr":         session.AMR,
			"remember_me": remember,
			"request_id":  challenge.RequestID,
		})
	if err := h.deps.Sessions.CreateLoginSession(r.Context(), session, audit); err != nil {
		return time.Time{}, err
	}
	setSessionCookie(w, h.deps.Config, rawSession, remember, absoluteExpiresAt)
	csrfExpiry := time.Time{}
	if remember {
		csrfExpiry = absoluteExpiresAt
	}
	setCSRFCookie(w, h.deps.Config, rawCSRF, csrfExpiry)
	logger.SetUserID(r.Context(), string(user.ID))
	return absoluteExpiresAt, nil
}

func (h *handlers) enrollMFA(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	var request domain.MFAEnrollRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if h.deps.MFA == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"mfa storage is unavailable")
		return
	}
	switch request.Action {
	case "start":
		h.startMFAEnrollment(w, r, current)
	case "confirm":
		h.confirmMFAEnrollment(w, r, current, request.Code)
	default:
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"action must be start or confirm")
	}
}

func (h *handlers) startMFAEnrollment(w http.ResponseWriter, r *http.Request, current *CurrentSession) {
	existing, err := h.deps.MFA.GetTOTP(r.Context(), current.User.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.logInternal(r.Context(), "mfa_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if existing != nil && existing.ConfirmedAt != nil {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"TOTP is already enrolled")
		return
	}
	if h.deps.Keyring == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"mfa encryption is unavailable")
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		h.logInternal(r.Context(), "totp_secret_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	encrypted, err := h.deps.Keyring.Encrypt([]byte(secret))
	if err != nil {
		h.logInternal(r.Context(), "totp_encrypt_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if err := h.deps.MFA.SaveTOTP(r.Context(), &domain.MFATOTP{
		UserID:          current.User.ID,
		SecretEncrypted: encrypted,
	}); err != nil {
		h.logInternal(r.Context(), "totp_save_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	uri := totp.OTPAuthURI(mfaIssuer, current.User.Email, secret)
	writeJSON(w, http.StatusOK, domain.MFAEnrollResponse{
		Action:     "start",
		Secret:     &secret,
		OTPAuthURI: &uri,
	})
}

func (h *handlers) confirmMFAEnrollment(w http.ResponseWriter, r *http.Request, current *CurrentSession, code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"code is required to confirm enrollment")
		return
	}
	if h.deps.Keyring == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"mfa encryption is unavailable")
		return
	}
	record, err := h.deps.MFA.GetTOTP(r.Context(), current.User.ID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"start enrollment before confirming")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "mfa_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if record.ConfirmedAt != nil {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"TOTP is already enrolled")
		return
	}
	secret, err := h.deps.Keyring.Decrypt(record.SecretEncrypted)
	if err != nil {
		h.logInternal(r.Context(), "totp_decrypt_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	step, ok := totp.Verify(string(secret), code, nowUTC())
	if !ok {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeMFAInvalid,
			"the mfa code is invalid")
		return
	}
	codes, err := totp.GenerateRecoveryCodes()
	if err != nil {
		h.logInternal(r.Context(), "recovery_code_generate_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	records := recoveryCodeRecords(current.User.ID, codes)
	audit := auditEvent(current.User.RealmID, domain.ActorTypeUser, userActor(current.User.ID),
		domain.AuditMFAEnrolled, map[string]any{"recovery_codes": len(codes)})
	if err := h.deps.MFA.ConfirmTOTP(r.Context(), current.User.ID, current.Session.ID,
		step, records, audit); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"TOTP is already enrolled")
			return
		}
		h.logInternal(r.Context(), "totp_confirm_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, domain.MFAEnrollResponse{
		Action:        "confirm",
		RecoveryCodes: codes,
	})
}

func (h *handlers) deleteMyTOTP(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	if h.deps.MFA == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"mfa storage is unavailable")
		return
	}
	if _, err := h.deps.MFA.GetTOTP(r.Context(), current.User.ID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"TOTP is not enrolled")
			return
		}
		h.logInternal(r.Context(), "mfa_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	audit := auditEvent(current.User.RealmID, domain.ActorTypeUser, userActor(current.User.ID),
		domain.AuditMFADisabled, nil)
	if err := h.deps.MFA.DeleteTOTP(r.Context(), current.User.ID, audit); err != nil {
		h.logInternal(r.Context(), "totp_delete_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func (h *handlers) regenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	if h.deps.MFA == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"mfa storage is unavailable")
		return
	}
	record, err := h.deps.MFA.GetTOTP(r.Context(), current.User.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"TOTP is not enrolled")
			return
		}
		h.logInternal(r.Context(), "mfa_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if record.ConfirmedAt == nil {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"TOTP is not confirmed")
		return
	}
	codes, err := totp.GenerateRecoveryCodes()
	if err != nil {
		h.logInternal(r.Context(), "recovery_code_generate_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	audit := auditEvent(current.User.RealmID, domain.ActorTypeUser, userActor(current.User.ID),
		domain.AuditMFARecoveryCreated, map[string]any{"count": len(codes)})
	if err := h.deps.MFA.ReplaceRecoveryCodes(r.Context(), current.User.ID,
		recoveryCodeRecords(current.User.ID, codes), audit); err != nil {
		h.logInternal(r.Context(), "recovery_code_replace_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, domain.RecoveryCodeResponse{RecoveryCodes: codes})
}

func recoveryCodeRecords(userID domain.UserID, codes []string) []domain.MFARecoveryCode {
	records := make([]domain.MFARecoveryCode, 0, len(codes))
	for _, code := range codes {
		records = append(records, domain.MFARecoveryCode{
			ID:       uuid.NewString(),
			UserID:   userID,
			CodeHash: totp.RecoveryCodeHash(code),
		})
	}
	return records
}
