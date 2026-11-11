package httphandler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func (h *handlers) userinfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, ok := bearerToken(r)
	if !ok && r.Method == http.MethodPost {
		if err := r.ParseForm(); err == nil {
			raw = strings.TrimSpace(r.PostForm.Get("access_token"))
			ok = raw != ""
		}
	}
	if !ok {
		writeBearerError(w, http.StatusUnauthorized, "a bearer access token is required")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		writeBearerError(w, http.StatusNotFound, "realm was not found")
		return
	}
	claims, err := h.deps.Tokens.VerifyAccessToken(r.Context(), realm.ID, raw)
	if err != nil || claims.ExpiresAt <= time.Now().Unix() ||
		claims.Issuer != realmIssuer(h.deps.Config.HTTP.ExternalURL, realm) {
		writeBearerError(w, http.StatusUnauthorized, "access token is invalid or expired")
		return
	}
	if claims.SessionID == "" {
		writeBearerError(w, http.StatusUnauthorized, "access token is not bound to a user session")
		return
	}
	if h.deps.TokenState != nil {
		revoked, err := h.deps.TokenState.JTIRevoked(r.Context(), claims.JTI)
		if err != nil || revoked {
			writeBearerError(w, http.StatusUnauthorized, "access token was revoked")
			return
		}
		sessionRevoked, err := h.deps.TokenState.SessionRevoked(r.Context(), domain.SessionID(claims.SessionID))
		if err != nil || sessionRevoked {
			writeBearerError(w, http.StatusUnauthorized, "access token was revoked")
			return
		}
	}
	if h.deps.Clients != nil && r.Header.Get("Origin") != "" {
		client, err := h.deps.Clients.GetByClientID(r.Context(), realm.ID, claims.Audience)
		if err != nil {
			writeBearerError(w, http.StatusUnauthorized, "access token audience is unknown")
			return
		}
		allowed, err := h.clientAllowsOrigin(r.Context(), client, r.Header.Get("Origin"))
		if err != nil || !allowed {
			WriteProblem(w, r, http.StatusForbidden, domain.CodeForbidden,
				"origin is not registered for this client")
			return
		}
	}
	session, err := h.deps.Sessions.GetByID(r.Context(), domain.SessionID(claims.SessionID))
	if err != nil {
		writeBearerError(w, http.StatusUnauthorized, "session is no longer active")
		return
	}
	now := time.Now().UTC()
	if session.Session.RevokedAt != nil || session.User.DisabledAt != nil ||
		!session.Session.IdleExpiresAt.After(now) || !session.Session.AbsoluteExpiresAt.After(now) ||
		string(session.User.ID) != claims.Subject || session.Session.RealmID != realm.ID {
		writeBearerError(w, http.StatusUnauthorized, "session is no longer active")
		return
	}

	scopes := domain.SplitScopes(claims.Scope)
	info := domain.UserInfo{
		"sub": claims.Subject,
	}
	if hasScope(scopes, domain.ScopeProfile) {
		info["name"] = session.User.Name
		info["given_name"] = session.User.GivenName
		info["family_name"] = session.User.FamilyName
		if session.User.Locale != nil {
			info["locale"] = *session.User.Locale
		}
		if session.User.Username != nil {
			info["preferred_username"] = *session.User.Username
		}
	}
	if hasScope(scopes, domain.ScopeEmail) {
		info["email"] = session.User.Email
		info["email_verified"] = session.User.EmailVerifiedAt != nil
	}
	if hasScope(scopes, domain.ScopeRoles) {
		info["roles"] = roleNames(session.Roles)
	}
	if hasScope(scopes, domain.ScopeGroups) {
		groups, err := h.deps.Roles.ListGroupsByUser(r.Context(), realm.ID, session.User.ID)
		if err != nil {
			h.logInternal(r.Context(), "group_lookup_failed", err)
			writeBearerError(w, http.StatusInternalServerError, "the server could not complete the request")
			return
		}
		info["groups"] = groupNames(groups)
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *handlers) revoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "form body is invalid")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		writeTokenError(w, http.StatusNotFound, domain.OAuthInvalidRequest, "realm was not found")
		return
	}
	client, ok := h.authenticateClient(w, r, realm)
	if !ok {
		return
	}
	raw := r.PostForm.Get("token")
	if raw == "" {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "token is required")
		return
	}

	if strings.Count(raw, ".") == 2 {
		claims, err := h.deps.Tokens.VerifyAccessToken(r.Context(), realm.ID, raw)
		if err == nil && claims.Audience == client.ClientID {
			if h.deps.TokenState != nil && claims.JTI != "" {
				if err := h.deps.TokenState.RevokeJTI(r.Context(), claims.JTI, claims.ExpiresAt); err != nil {
					h.logInternal(r.Context(), "token_state_write_failed", err)
					writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "token state is unavailable")
					return
				}
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if h.deps.RefreshTokens == nil {
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "refresh tokens are unavailable")
		return
	}
	audit := auditEvent(realm.ID, domain.ActorTypeClient, &client.ClientID,
		domain.AuditRefreshRevoked, map[string]any{"client_id": client.ClientID})
	token, err := h.deps.RefreshTokens.Revoke(r.Context(), hashToken(raw), string(client.ID), audit)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.logInternal(r.Context(), "refresh_revoke_failed", err)
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "token revocation failed")
		return
	}
	if token != nil && h.deps.TokenState != nil {
		_ = h.deps.TokenState.RevokeSession(r.Context(), token.SessionID, revokedSessionTTL)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *handlers) introspect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "form body is invalid")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		writeTokenError(w, http.StatusNotFound, domain.OAuthInvalidRequest, "realm was not found")
		return
	}
	client, ok := h.authenticateClient(w, r, realm)
	if !ok {
		return
	}
	raw := r.PostForm.Get("token")
	if raw == "" {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "token is required")
		return
	}
	var response domain.IntrospectionResponse
	if strings.Count(raw, ".") == 2 {
		response = h.introspectAccess(r, realm, client, raw)
	} else {
		response = h.introspectRefresh(r, realm, client, raw)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *handlers) introspectAccess(r *http.Request, realm *domain.Realm, client *domain.Client, raw string) domain.IntrospectionResponse {
	claims, err := h.deps.Tokens.VerifyAccessToken(r.Context(), realm.ID, raw)
	now := time.Now().Unix()
	if err != nil || claims.ExpiresAt <= now ||
		claims.Issuer != realmIssuer(h.deps.Config.HTTP.ExternalURL, realm) ||
		claims.Audience != client.ClientID {
		return domain.IntrospectionResponse{Active: false}
	}
	if h.deps.TokenState != nil {
		if revoked, err := h.deps.TokenState.JTIRevoked(r.Context(), claims.JTI); err != nil || revoked {
			return domain.IntrospectionResponse{Active: false}
		}
		if claims.SessionID != "" {
			if revoked, err := h.deps.TokenState.SessionRevoked(r.Context(), domain.SessionID(claims.SessionID)); err != nil || revoked {
				return domain.IntrospectionResponse{Active: false}
			}
		}
	}
	if claims.SessionID != "" {
		session, err := h.deps.Sessions.GetByID(r.Context(), domain.SessionID(claims.SessionID))
		if err != nil {
			return domain.IntrospectionResponse{Active: false}
		}
		sessionNow := time.Now().UTC()
		if session.Session.RevokedAt != nil || session.User.DisabledAt != nil ||
			!session.Session.IdleExpiresAt.After(sessionNow) ||
			!session.Session.AbsoluteExpiresAt.After(sessionNow) ||
			string(session.User.ID) != claims.Subject {
			return domain.IntrospectionResponse{Active: false}
		}
	} else if claims.Subject != client.ClientID {
		return domain.IntrospectionResponse{Active: false}
	}
	return domain.IntrospectionResponse{
		Active:    true,
		Scope:     claims.Scope,
		ClientID:  client.ClientID,
		TokenType: domain.TokenTypeBearer,
		ExpiresAt: claims.ExpiresAt,
		IssuedAt:  claims.IssuedAt,
		Subject:   claims.Subject,
		Audience:  claims.Audience,
		Issuer:    claims.Issuer,
		JTI:       claims.JTI,
		SessionID: claims.SessionID,
	}
}

func (h *handlers) introspectRefresh(r *http.Request, realm *domain.Realm, client *domain.Client, raw string) domain.IntrospectionResponse {
	if h.deps.RefreshTokens == nil {
		return domain.IntrospectionResponse{Active: false}
	}
	token, err := h.deps.RefreshTokens.GetByHash(r.Context(), hashToken(raw))
	if err != nil || token.RevokedAt != nil || token.UsedAt != nil ||
		token.ClientID != client.ID {
		return domain.IntrospectionResponse{Active: false}
	}
	now := time.Now().UTC()
	if !token.ExpiresAt.After(now) || !token.AbsoluteExpiresAt.After(now) {
		return domain.IntrospectionResponse{Active: false}
	}
	session, err := h.deps.Sessions.GetByID(r.Context(), token.SessionID)
	if err != nil || session.Session.RevokedAt != nil || session.User.DisabledAt != nil ||
		!session.Session.IdleExpiresAt.After(now) || !session.Session.AbsoluteExpiresAt.After(now) {
		return domain.IntrospectionResponse{Active: false}
	}
	return domain.IntrospectionResponse{
		Active:    true,
		Scope:     token.Scope,
		ClientID:  client.ClientID,
		TokenType: "refresh_token",
		ExpiresAt: token.ExpiresAt.Unix(),
		IssuedAt:  token.CreatedAt.Unix(),
		Subject:   string(session.User.ID),
		Audience:  client.ClientID,
		Issuer:    realmIssuer(h.deps.Config.HTTP.ExternalURL, realm),
		SessionID: string(token.SessionID),
	}
}

func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != ""
}

func writeBearerError(w http.ResponseWriter, status int, description string) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	WriteOAuthError(w, status, domain.OAuthInvalidToken, description, "")
}
