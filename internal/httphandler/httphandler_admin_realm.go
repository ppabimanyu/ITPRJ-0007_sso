package httphandler

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func (h *handlers) getRealmSettings(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	realm, err := h.deps.Realms.GetByID(r.Context(), current.User.RealmID)
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if h.deps.RealmSettings == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"realm settings storage is unavailable")
		return
	}
	settings, err := h.deps.RealmSettings.GetSettings(r.Context(), realm.ID)
	if err != nil {
		h.logInternal(r.Context(), "realm_settings_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	settings.Realm = realm.Name
	writeJSON(w, http.StatusOK, settings)
}

func (h *handlers) updateRealmSettings(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	realm, err := h.deps.Realms.GetByID(r.Context(), current.User.RealmID)
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	var request domain.UpdateRealmSettingsRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if h.deps.RealmSettings == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"realm settings storage is unavailable")
		return
	}
	settings, err := h.deps.RealmSettings.GetSettings(r.Context(), realm.ID)
	if err != nil {
		h.logInternal(r.Context(), "realm_settings_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	fields := applyRealmSettingsPatch(settings, request)
	if len(fields) > 0 {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"realm settings do not meet the constraints", fields)
		return
	}
	if request.SMTP != nil && request.SMTP.Password != nil && *request.SMTP.Password != "" {
		if h.deps.Keyring == nil {
			WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
				"the master keyring is unavailable")
			return
		}
		ciphertext, err := h.deps.Keyring.Encrypt([]byte(*request.SMTP.Password))
		if err != nil {
			h.logInternal(r.Context(), "smtp_password_encrypt_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		settings.SMTP.PasswordEncrypted = ciphertext
		settings.SMTP.PasswordSet = true
	}
	if request.SMTP != nil && request.SMTP.Password != nil && *request.SMTP.Password == "" {
		settings.SMTP.PasswordEncrypted = nil
		settings.SMTP.PasswordSet = false
	}
	audit := adminAudit(current, domain.AuditAdminRealmSettings, map[string]any{
		"password_policy":   request.PasswordPolicy != nil,
		"session_policy":    request.SessionPolicy != nil,
		"smtp":              request.SMTP != nil,
		"smtp_password_set": settings.SMTP.PasswordSet,
	})
	updated, err := h.deps.RealmSettings.UpdateSettings(r.Context(), realm.ID, settings, audit)
	if err != nil {
		h.logInternal(r.Context(), "realm_settings_update_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	updated.Realm = realm.Name
	writeJSON(w, http.StatusOK, updated)
}

func applyRealmSettingsPatch(settings *domain.RealmSettings, request domain.UpdateRealmSettingsRequest) []FieldError {
	var fields []FieldError
	if request.PasswordPolicy != nil && request.PasswordPolicy.MinLength != nil {
		value := *request.PasswordPolicy.MinLength
		if value < 12 || value > 64 {
			fields = append(fields, FieldError{Field: "password_policy.min_length", Message: "must be between 12 and 64"})
		} else {
			settings.PasswordPolicy.MinLength = value
		}
	}
	if request.SessionPolicy != nil {
		policy := request.SessionPolicy
		if policy.IdleMinutes != nil {
			if *policy.IdleMinutes < 5 || *policy.IdleMinutes > 1440 {
				fields = append(fields, FieldError{Field: "session_policy.idle_minutes", Message: "must be between 5 and 1440"})
			} else {
				settings.SessionPolicy.IdleMinutes = *policy.IdleMinutes
			}
		}
		if policy.AbsoluteHours != nil {
			if *policy.AbsoluteHours < 1 || *policy.AbsoluteHours > 168 {
				fields = append(fields, FieldError{Field: "session_policy.absolute_hours", Message: "must be between 1 and 168"})
			} else {
				settings.SessionPolicy.AbsoluteHours = *policy.AbsoluteHours
			}
		}
		if policy.AdminIdleMinutes != nil {
			if *policy.AdminIdleMinutes < 5 || *policy.AdminIdleMinutes > 60 {
				fields = append(fields, FieldError{Field: "session_policy.admin_idle_minutes", Message: "must be between 5 and 60"})
			} else {
				settings.SessionPolicy.AdminIdleMinutes = *policy.AdminIdleMinutes
			}
		}
		if policy.RememberMeDays != nil {
			if *policy.RememberMeDays < 1 || *policy.RememberMeDays > 90 {
				fields = append(fields, FieldError{Field: "session_policy.remember_me_days", Message: "must be between 1 and 90"})
			} else {
				settings.SessionPolicy.RememberMeDays = *policy.RememberMeDays
			}
		}
	}
	if request.SMTP != nil {
		smtp := request.SMTP
		if smtp.Host != nil {
			host := strings.TrimSpace(*smtp.Host)
			if host != *smtp.Host || strings.ContainsAny(host, " \t\r\n") || len(host) > 255 {
				fields = append(fields, FieldError{Field: "smtp.host", Message: "must be a hostname without spaces"})
			} else {
				settings.SMTP.Host = host
			}
		}
		if smtp.Port != nil {
			if *smtp.Port < 1 || *smtp.Port > 65535 {
				fields = append(fields, FieldError{Field: "smtp.port", Message: "must be between 1 and 65535"})
			} else {
				settings.SMTP.Port = *smtp.Port
			}
		}
		if smtp.Username.Set {
			if smtp.Username.Value == nil {
				settings.SMTP.Username = nil
			} else {
				username := strings.TrimSpace(*smtp.Username.Value)
				if len(username) > 255 {
					fields = append(fields, FieldError{Field: "smtp.username", Message: "must be at most 255 characters"})
				} else {
					settings.SMTP.Username = &username
				}
			}
		}
		if smtp.FromAddress != nil {
			from := strings.TrimSpace(*smtp.FromAddress)
			if from == "" {
				settings.SMTP.FromAddress = ""
			} else if parsed, err := mail.ParseAddress(from); err != nil {
				fields = append(fields, FieldError{Field: "smtp.from_address", Message: "must be a valid email address"})
			} else {
				settings.SMTP.FromAddress = parsed.Address
			}
		}
		if smtp.UseTLS != nil {
			settings.SMTP.UseTLS = *smtp.UseTLS
		}
		if smtp.Password != nil && len(*smtp.Password) > 512 {
			fields = append(fields, FieldError{Field: "smtp.password", Message: "must be at most 512 characters"})
		}
	}
	if settings.SMTP.Host == "" && settings.SMTP.FromAddress != "" {
		fields = append(fields, FieldError{Field: "smtp.host", Message: "is required when a from address is set"})
	}
	if settings.SMTP.Host != "" && settings.SMTP.FromAddress == "" {
		fields = append(fields, FieldError{Field: "smtp.from_address", Message: "is required when a host is set"})
	}
	if request.SMTP != nil && request.SMTP.Password != nil && *request.SMTP.Password != "" &&
		settings.SMTP.Host == "" {
		fields = append(fields, FieldError{Field: "smtp.host", Message: "is required when a password is set"})
	}
	return fields
}
