package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

func (h *handlers) realm(ctx context.Context) (*domain.Realm, error) {
	if h.deps.Realms == nil {
		return nil, errors.New("realm repository is not configured")
	}
	return h.deps.Realms.GetByName(ctx, RealmFromContext(ctx))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		detail := "request body is not valid JSON"
		if !errors.Is(err, io.EOF) {
			detail = "request body is not valid JSON: " + err.Error()
		}
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, detail)
		return false
	}
	return true
}

func userDTO(user *domain.User, roles []domain.Role, mfaEnrolled bool) domain.UserDTO {
	roleRefs := make([]domain.RoleRef, 0, len(roles))
	for _, role := range roles {
		roleRefs = append(roleRefs, domain.RoleRef{ID: string(role.ID), Name: role.Name})
	}
	return domain.UserDTO{
		ID:              string(user.ID),
		Email:           user.Email,
		Username:        user.Username,
		Name:            stringPtr(user.Name),
		GivenName:       stringPtr(user.GivenName),
		FamilyName:      stringPtr(user.FamilyName),
		Locale:          user.Locale,
		EmailVerifiedAt: user.EmailVerifiedAt,
		DisabledAt:      user.DisabledAt,
		MFAEnrolled:     mfaEnrolled,
		Roles:           roleRefs,
		Groups:          []domain.GroupRef{},
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func auditEvent(realmID domain.RealmID, actorType string, actorID *string, eventType string, metadata map[string]any) *domain.AuditEvent {
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		payload = []byte("{}")
	}
	return &domain.AuditEvent{
		ID:        domain.AuditEventID(uuid.NewString()),
		RealmID:   realmID,
		ActorType: actorType,
		ActorID:   actorID,
		Type:      eventType,
		Metadata:  payload,
	}
}

func userActor(userID domain.UserID) *string {
	value := string(userID)
	return &value
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func localeOf(user *domain.User) string {
	if user.Locale != nil {
		switch *user.Locale {
		case "id":
			return "id"
		case "en":
			return "en"
		}
	}
	return "en"
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) logInternal(ctx context.Context, message string, err error) {
	logger.FromContext(ctx).Error(message, "error", err)
}

func retryAfterSeconds(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func nowUTC() time.Time { return time.Now().UTC() }

func normalizedEmail(raw string) string { return strings.ToLower(strings.TrimSpace(raw)) }
