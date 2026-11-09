package httphandler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

const (
	authRequestTTL    = 10 * time.Minute
	authCodeTTL       = 60 * time.Second
	accessTokenTTL    = 15 * time.Minute
	idTokenTTL        = 5 * time.Minute
	refreshIdleTTL    = 30 * 24 * time.Hour
	refreshAbsolute   = 30 * 24 * time.Hour
	keyRetireAfter    = 24 * time.Hour
	revokedSessionTTL = 3600
)

func authRequestCookieName(cfg interface{ IsDev() bool }) string {
	if cfg.IsDev() {
		return "sso_authreq"
	}
	return "__Host-sso_authreq"
}

func setAuthRequestCookie(w http.ResponseWriter, cfg interface {
	IsDev() bool
	SessionCookieSecure() bool
}, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     authRequestCookieName(cfg),
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.SessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().UTC().Add(authRequestTTL),
		MaxAge:   int(authRequestTTL.Seconds()),
	})
}

func clearAuthRequestCookie(w http.ResponseWriter, cfg interface {
	IsDev() bool
	SessionCookieSecure() bool
}) {
	http.SetCookie(w, &http.Cookie{
		Name:     authRequestCookieName(cfg),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.SessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
}

// pkceS256 is the only challenge method accepted by the provider.
func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func validPKCEVerifier(verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for _, r := range verifier {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '.', r == '_', r == '~':
		default:
			return false
		}
	}
	return true
}

func verifyPKCE(challenge, verifier string) bool {
	if challenge == "" || !validPKCEVerifier(verifier) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(challenge), []byte(pkceS256(verifier))) == 1
}

func accessTokenHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

func scopeSet(scopes []string) map[string]struct{} {
	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		set[scope] = struct{}{}
	}
	return set
}

func hasScope(scopes []string, wanted string) bool {
	for _, scope := range scopes {
		if scope == wanted {
			return true
		}
	}
	return false
}

func scopesSubset(wanted, allowed []string) bool {
	allowedSet := scopeSet(allowed)
	for _, scope := range wanted {
		if _, ok := allowedSet[scope]; !ok {
			return false
		}
	}
	return true
}

func validScopes(scopes []string) bool {
	for _, scope := range scopes {
		switch scope {
		case domain.ScopeOpenID, domain.ScopeProfile, domain.ScopeEmail,
			domain.ScopeOfflineAccess, domain.ScopeRoles, domain.ScopeGroups:
		default:
			return false
		}
	}
	return true
}

func realmIssuer(cfgExternalURL string, realm *domain.Realm) string {
	if issuer := strings.TrimRight(realm.Issuer, "/"); issuer != "" {
		return issuer
	}
	return strings.TrimRight(cfgExternalURL, "/") + "/realms/" + realm.Name
}

func sessionUsable(current *CurrentSession, now time.Time) bool {
	if current == nil || current.Session == nil || current.User == nil {
		return false
	}
	session := current.Session
	return session.RevokedAt == nil &&
		current.User.DisabledAt == nil &&
		session.IdleExpiresAt.After(now) &&
		session.AbsoluteExpiresAt.After(now)
}

func sessionACR(amr []string) string {
	if hasScope(amr, domain.AMROTP) {
		return domain.ACRMFA
	}
	return domain.ACRPassword
}

func consentCovers(consent *domain.Consent, scopes []string) bool {
	if consent == nil {
		return false
	}
	return scopesSubset(scopes, domain.SplitScopes(consent.Scope))
}

func roleNames(roles []domain.Role) []string {
	if len(roles) == 0 {
		return nil
	}
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

func groupNames(groups []domain.Group) []string {
	if len(groups) == 0 {
		return nil
	}
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Name)
	}
	return names
}

func (h *handlers) optionalSession(ctx context.Context, r *http.Request) (*CurrentSession, error) {
	cookie, err := r.Cookie(h.deps.Config.SessionCookieName())
	if err != nil || cookie.Value == "" {
		return nil, nil
	}
	record, err := h.deps.Sessions.GetByTokenHash(ctx, pkg.SHA256Hex(cookie.Value))
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	current := &CurrentSession{
		Session:     record.Session,
		User:        record.User,
		Roles:       record.Roles,
		MFAEnrolled: record.MFAEnrolled,
	}
	if !sessionUsable(current, time.Now().UTC()) {
		return nil, nil
	}
	return current, nil
}

func (h *handlers) authRequestCookieMatches(r *http.Request, request *domain.AuthRequest) bool {
	cookie, err := r.Cookie(authRequestCookieName(h.deps.Config))
	if err != nil || cookie.Value == "" {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(pkg.SHA256Hex(cookie.Value)),
		[]byte(request.BrowserHash),
	) == 1
}

func (h *handlers) clientAllowsOrigin(ctx context.Context, client *domain.Client, origin string) (bool, error) {
	if origin == "" {
		return true, nil
	}
	origins, err := h.deps.Clients.CORSOrigins(ctx, client.ID)
	if err != nil {
		return false, err
	}
	for _, allowed := range origins {
		if strings.EqualFold(allowed, origin) {
			return true, nil
		}
	}
	return false, nil
}

// redirectOAuthError sends an authorize failure to the registered redirect URI.
// Callers must validate the redirect URI before invoking it.
func redirectOAuthError(w http.ResponseWriter, r *http.Request, redirectURI, state, code, description string) {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "redirect_uri is invalid")
		return
	}
	query := parsed.Query()
	query.Set("error", code)
	if description != "" {
		query.Set("error_description", description)
	}
	if state != "" {
		query.Set("state", state)
	}
	parsed.RawQuery = query.Encode()
	http.Redirect(w, r, parsed.String(), http.StatusFound)
}

func authorizeSuccessRedirect(redirectURI, code, state string) string {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return redirectURI
	}
	query := parsed.Query()
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (h *handlers) auditOIDC(ctx context.Context, realmID domain.RealmID, actorType string, actorID *string, eventType string, metadata map[string]any) {
	if h.deps.Audit == nil {
		return
	}
	if err := h.deps.Audit.Insert(ctx, auditEvent(realmID, actorType, actorID, eventType, metadata)); err != nil {
		logger.FromContext(ctx).Warn("audit_insert_failed", "error", err)
	}
}

func parseNonNegativeInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func boolPtr(value bool) *bool { return &value }

func hashToken(raw string) string { return pkg.SHA256Hex(raw) }
