package httphandler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

func (h *handlers) token(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
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
	switch grant := r.PostForm.Get("grant_type"); grant {
	case domain.GrantTypeAuthorizationCode:
		h.exchangeCode(w, r, realm, client)
	case domain.GrantTypeRefreshToken:
		h.exchangeRefresh(w, r, realm, client)
	case domain.GrantTypeClientCredentials:
		h.clientCredentials(w, r, realm, client)
	default:
		writeTokenError(w, http.StatusBadRequest, domain.OAuthUnsupportedGrantType,
			"grant_type is not supported")
	}
}

func (h *handlers) authenticateClient(w http.ResponseWriter, r *http.Request, realm *domain.Realm) (*domain.Client, bool) {
	if h.deps.Clients == nil {
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "client storage is unavailable")
		return nil, false
	}
	clientID := r.PostForm.Get("client_id")
	secret := r.PostForm.Get("client_secret")
	if basicID, basicSecret, hasBasic := r.BasicAuth(); hasBasic {
		decodedID, errID := url.QueryUnescape(basicID)
		decodedSecret, errSecret := url.QueryUnescape(basicSecret)
		if errID != nil || errSecret != nil {
			writeTokenError(w, http.StatusUnauthorized, domain.OAuthInvalidClient, "client credentials are malformed")
			return nil, false
		}
		if clientID != "" && clientID != decodedID {
			writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "multiple client_id values")
			return nil, false
		}
		if r.PostForm.Get("client_secret") != "" {
			writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "multiple client authentication methods")
			return nil, false
		}
		clientID = decodedID
		secret = decodedSecret
	}
	if clientID == "" {
		writeTokenError(w, http.StatusUnauthorized, domain.OAuthInvalidClient, "client_id is required")
		return nil, false
	}
	client, err := h.deps.Clients.GetByClientID(r.Context(), realm.ID, clientID)
	if err != nil || client.DisabledAt != nil {
		writeTokenError(w, http.StatusUnauthorized, domain.OAuthInvalidClient, "client authentication failed")
		return nil, false
	}

	switch client.Type {
	case domain.ClientTypePublic:
		if secret != "" {
			writeTokenError(w, http.StatusUnauthorized, domain.OAuthInvalidClient, "public clients must not send a secret")
			return nil, false
		}
	case domain.ClientTypeConfidential:
		if secret == "" {
			writeTokenError(w, http.StatusUnauthorized, domain.OAuthInvalidClient, "client_secret is required")
			return nil, false
		}
		authorized, err := h.deps.Clients.AuthorizeSecret(r.Context(), client.ID, pkg.SHA256Hex(secret))
		if err != nil {
			logger.FromContext(r.Context()).Error("client_secret_lookup_failed", "error", err)
			writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "client storage is unavailable")
			return nil, false
		}
		if !authorized {
			writeTokenError(w, http.StatusUnauthorized, domain.OAuthInvalidClient, "client authentication failed")
			return nil, false
		}
	default:
		writeTokenError(w, http.StatusUnauthorized, domain.OAuthInvalidClient, "client authentication failed")
		return nil, false
	}

	allowed, err := h.clientAllowsOrigin(r.Context(), client, r.Header.Get("Origin"))
	if err != nil {
		logger.FromContext(r.Context()).Error("cors_origin_lookup_failed", "error", err)
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "client storage is unavailable")
		return nil, false
	}
	if !allowed {
		writeTokenError(w, http.StatusForbidden, domain.OAuthAccessDenied, "origin is not registered for this client")
		return nil, false
	}
	return client, true
}

func (h *handlers) exchangeCode(w http.ResponseWriter, r *http.Request, realm *domain.Realm, client *domain.Client) {
	if !hasScope(client.GrantTypes, domain.GrantTypeAuthorizationCode) {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthUnauthorizedClient, "client may not use the authorization code grant")
		return
	}
	codeValue := r.PostForm.Get("code")
	redirectURI := r.PostForm.Get("redirect_uri")
	if codeValue == "" || redirectURI == "" {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "code and redirect_uri are required")
		return
	}
	if h.deps.AuthCodes == nil {
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "authorization codes are unavailable")
		return
	}
	code, err := h.deps.AuthCodes.Take(r.Context(), pkg.SHA256Hex(codeValue))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "code is invalid or already used")
			return
		}
		h.logInternal(r.Context(), "auth_code_take_failed", err)
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "authorization codes are unavailable")
		return
	}
	if code.ClientID != client.ID || code.ClientIDString != client.ClientID {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "code was not issued to this client")
		return
	}
	if code.RedirectURI != redirectURI {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "redirect_uri does not match the code")
		return
	}
	if !verifyPKCE(code.CodeChallenge, r.PostForm.Get("code_verifier")) {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "PKCE verification failed")
		return
	}
	session, ok := h.activeSession(w, r, code.SessionID)
	if !ok {
		return
	}
	if session.User.ID != code.UserID || session.Session.RealmID != realm.ID {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "session does not match the code")
		return
	}
	scope := domain.JoinScopes(code.Scopes)
	access, idToken, expiresIn, err := h.signUserTokens(r.Context(), realm, client, session, scope,
		code.AuthTime, code.AMR, code.ACR, code.Nonce)
	if err != nil {
		h.logInternal(r.Context(), "token_sign_failed", err)
		writeTokenError(w, http.StatusInternalServerError, domain.OAuthServerError, "token issuance failed")
		return
	}
	response := domain.TokenResponse{
		AccessToken: access,
		TokenType:   domain.TokenTypeBearer,
		ExpiresIn:   expiresIn,
		IDToken:     idToken,
		Scope:       scope,
	}
	if hasScope(code.Scopes, domain.ScopeOfflineAccess) && hasScope(client.GrantTypes, domain.GrantTypeRefreshToken) {
		refresh, err := h.createRefreshToken(r.Context(), client, session.Session.ID, scope)
		if err != nil {
			h.logInternal(r.Context(), "refresh_token_create_failed", err)
			writeTokenError(w, http.StatusInternalServerError, domain.OAuthServerError, "token issuance failed")
			return
		}
		response.RefreshToken = refresh
	}
	h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeUser, userActor(session.User.ID),
		domain.AuditTokenIssued, map[string]any{
			"grant_type":    domain.GrantTypeAuthorizationCode,
			"client_id":     client.ClientID,
			"scopes":        code.Scopes,
			"sid":           string(session.Session.ID),
			"refresh_token": response.RefreshToken != "",
		})
	writeJSON(w, http.StatusOK, response)
}

func (h *handlers) exchangeRefresh(w http.ResponseWriter, r *http.Request, realm *domain.Realm, client *domain.Client) {
	if !hasScope(client.GrantTypes, domain.GrantTypeRefreshToken) {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthUnauthorizedClient, "client may not use the refresh token grant")
		return
	}
	oldRefresh := r.PostForm.Get("refresh_token")
	if oldRefresh == "" {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidRequest, "refresh_token is required")
		return
	}
	if h.deps.RefreshTokens == nil {
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "refresh tokens are unavailable")
		return
	}
	newRefresh, err := pkg.RandomToken(32)
	if err != nil {
		writeTokenError(w, http.StatusInternalServerError, domain.OAuthServerError, "token issuance failed")
		return
	}
	reuseAudit := auditEvent(realm.ID, domain.ActorTypeClient, &client.ClientID,
		domain.AuditRefreshReused, map[string]any{"client_id": client.ClientID})
	rotation, err := h.deps.RefreshTokens.Rotate(r.Context(), pkg.SHA256Hex(oldRefresh),
		string(client.ID), pkg.SHA256Hex(newRefresh), uuid.NewString(), nowUTC(),
		refreshIdleTTL, reuseAudit)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshExpired) || errors.Is(err, domain.ErrNotFound) {
			writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "refresh token is invalid or expired")
			return
		}
		h.logInternal(r.Context(), "refresh_rotate_failed", err)
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "token issuance failed")
		return
	}
	if rotation.Reused {
		if rotation.Token != nil && h.deps.TokenState != nil {
			_ = h.deps.TokenState.RevokeSession(r.Context(), rotation.Token.SessionID, revokedSessionTTL)
		}
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "refresh token reuse detected; the family was revoked")
		return
	}
	token := rotation.Token
	session, ok := h.activeSession(w, r, token.SessionID)
	if !ok {
		if h.deps.RefreshTokens != nil {
			_ = h.deps.RefreshTokens.RevokeFamily(r.Context(), token.FamilyID,
				auditEvent(realm.ID, domain.ActorTypeClient, &client.ClientID,
					domain.AuditRefreshRevoked, map[string]any{"client_id": client.ClientID, "family_revoked": true}))
		}
		return
	}
	scope := token.Scope
	if requested := r.PostForm.Get("scope"); requested != "" {
		requestedScopes := domain.SplitScopes(requested)
		if !scopesSubset(requestedScopes, domain.SplitScopes(token.Scope)) {
			writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidScope, "scope exceeds the original grant")
			return
		}
		scope = requested
	}
	access, idToken, expiresIn, err := h.signUserTokens(r.Context(), realm, client, session, scope,
		session.Session.AuthTime, session.Session.AMR, effectiveSessionACR(session.Session), "")
	if err != nil {
		h.logInternal(r.Context(), "token_sign_failed", err)
		writeTokenError(w, http.StatusInternalServerError, domain.OAuthServerError, "token issuance failed")
		return
	}
	h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeUser, userActor(session.User.ID),
		domain.AuditTokenIssued, map[string]any{
			"grant_type":    domain.GrantTypeRefreshToken,
			"client_id":     client.ClientID,
			"scopes":        domain.SplitScopes(scope),
			"sid":           string(session.Session.ID),
			"refresh_token": true,
		})
	writeJSON(w, http.StatusOK, domain.TokenResponse{
		AccessToken:  access,
		TokenType:    domain.TokenTypeBearer,
		ExpiresIn:    expiresIn,
		IDToken:      idToken,
		RefreshToken: newRefresh,
		Scope:        scope,
	})
}

func (h *handlers) clientCredentials(w http.ResponseWriter, r *http.Request, realm *domain.Realm, client *domain.Client) {
	if client.Type != domain.ClientTypeConfidential {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthUnauthorizedClient, "client_credentials requires a confidential client")
		return
	}
	if !hasScope(client.GrantTypes, domain.GrantTypeClientCredentials) {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthUnauthorizedClient, "client may not use the client credentials grant")
		return
	}
	scopes := domain.SplitScopes(r.PostForm.Get("scope"))
	if len(scopes) == 0 {
		scopes = make([]string, 0, len(client.AllowedScopes))
		for _, scope := range client.AllowedScopes {
			if scope != domain.ScopeOfflineAccess {
				scopes = append(scopes, scope)
			}
		}
	}
	if !validScopes(scopes) || !scopesSubset(scopes, client.AllowedScopes) {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidScope, "one or more scopes are not allowed")
		return
	}
	now := nowUTC()
	ttl := h.accessTokenTTL(client)
	claims := domain.AccessTokenClaims{
		Issuer:    realmIssuer(h.deps.Config.HTTP.ExternalURL, realm),
		Subject:   client.ClientID,
		Audience:  client.ClientID,
		ExpiresAt: now.Add(ttl).Unix(),
		IssuedAt:  now.Unix(),
		JTI:       uuid.NewString(),
		Scope:     domain.JoinScopes(scopes),
	}
	access, err := h.deps.Tokens.SignAccessToken(r.Context(), realm.ID, claims)
	if err != nil {
		h.logInternal(r.Context(), "token_sign_failed", err)
		writeTokenError(w, http.StatusInternalServerError, domain.OAuthServerError, "token issuance failed")
		return
	}
	h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeClient, &client.ClientID,
		domain.AuditTokenIssued, map[string]any{
			"grant_type": domain.GrantTypeClientCredentials,
			"client_id":  client.ClientID,
			"scopes":     scopes,
		})
	writeJSON(w, http.StatusOK, domain.TokenResponse{
		AccessToken: access,
		TokenType:   domain.TokenTypeBearer,
		ExpiresIn:   int(ttl.Seconds()),
		Scope:       domain.JoinScopes(scopes),
	})
}

func (h *handlers) signUserTokens(ctx context.Context, realm *domain.Realm, client *domain.Client, session *domain.SessionWithUser, scope string, authTime time.Time, amr []string, acr, nonce string) (string, string, int, error) {
	scopes := domain.SplitScopes(scope)
	roles := []string(nil)
	groups := []string(nil)
	if hasScope(scopes, domain.ScopeRoles) {
		roles = roleNames(session.Roles)
	}
	if hasScope(scopes, domain.ScopeGroups) && h.deps.Roles != nil {
		userGroups, err := h.deps.Roles.ListGroupsByUser(ctx, realm.ID, session.User.ID)
		if err != nil {
			return "", "", 0, err
		}
		groups = groupNames(userGroups)
	}
	issuer := realmIssuer(h.deps.Config.HTTP.ExternalURL, realm)
	now := nowUTC()
	accessTTL := h.accessTokenTTL(client)
	idTTL := h.idTokenTTL(client)
	accessClaims := domain.AccessTokenClaims{
		Issuer:    issuer,
		Subject:   string(session.User.ID),
		Audience:  client.ClientID,
		ExpiresAt: now.Add(accessTTL).Unix(),
		IssuedAt:  now.Unix(),
		JTI:       uuid.NewString(),
		Scope:     scope,
		SessionID: string(session.Session.ID),
		Roles:     roles,
		Groups:    groups,
	}
	access, err := h.deps.Tokens.SignAccessToken(ctx, realm.ID, accessClaims)
	if err != nil {
		return "", "", 0, err
	}
	idClaims := domain.IDTokenClaims{
		Issuer:          issuer,
		Subject:         string(session.User.ID),
		Audience:        client.ClientID,
		ExpiresAt:       now.Add(idTTL).Unix(),
		IssuedAt:        now.Unix(),
		AuthTime:        authTime.Unix(),
		Nonce:           nonce,
		AMR:             amr,
		ACR:             acr,
		AccessTokenHash: accessTokenHash(access),
		SessionID:       string(session.Session.ID),
		Roles:           roles,
		Groups:          groups,
	}
	if hasScope(scopes, domain.ScopeEmail) {
		idClaims.Email = session.User.Email
		idClaims.EmailVerified = boolPtr(session.User.EmailVerifiedAt != nil)
	}
	if hasScope(scopes, domain.ScopeProfile) {
		idClaims.Name = session.User.Name
		idClaims.GivenName = session.User.GivenName
		idClaims.FamilyName = session.User.FamilyName
		idClaims.Locale = derefString(session.User.Locale)
		if session.User.Username != nil {
			idClaims.PreferredUsername = *session.User.Username
		}
	}
	idToken, err := h.deps.Tokens.SignIDToken(ctx, realm.ID, idClaims)
	if err != nil {
		return "", "", 0, err
	}
	return access, idToken, int(accessTTL.Seconds()), nil
}

func (h *handlers) createRefreshToken(ctx context.Context, client *domain.Client, sessionID domain.SessionID, scope string) (string, error) {
	if h.deps.RefreshTokens == nil {
		return "", errors.New("refresh token storage is unavailable")
	}
	raw, err := pkg.RandomToken(32)
	if err != nil {
		return "", err
	}
	now := nowUTC()
	token := &domain.RefreshToken{
		ID:                uuid.NewString(),
		SessionID:         sessionID,
		ClientID:          client.ID,
		TokenHash:         pkg.SHA256Hex(raw),
		FamilyID:          uuid.NewString(),
		Scope:             scope,
		ExpiresAt:         now.Add(refreshIdleTTL),
		AbsoluteExpiresAt: now.Add(refreshAbsolute),
		CreatedAt:         now,
	}
	if err := h.deps.RefreshTokens.Create(ctx, token); err != nil {
		return "", err
	}
	return raw, nil
}

func (h *handlers) activeSession(w http.ResponseWriter, r *http.Request, id domain.SessionID) (*domain.SessionWithUser, bool) {
	if h.deps.Sessions == nil {
		writeTokenError(w, http.StatusServiceUnavailable, domain.OAuthServerError, "session storage is unavailable")
		return nil, false
	}
	session, err := h.deps.Sessions.GetByID(r.Context(), id)
	if err != nil {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "session is no longer active")
		return nil, false
	}
	now := time.Now().UTC()
	if session.Session.RevokedAt != nil || session.User.DisabledAt != nil ||
		!session.Session.IdleExpiresAt.After(now) || !session.Session.AbsoluteExpiresAt.After(now) {
		writeTokenError(w, http.StatusBadRequest, domain.OAuthInvalidGrant, "session is no longer active")
		return nil, false
	}
	return session, true
}

func (h *handlers) accessTokenTTL(client *domain.Client) time.Duration {
	if client.AccessTokenTTLSeconds != nil && *client.AccessTokenTTLSeconds > 0 {
		return time.Duration(*client.AccessTokenTTLSeconds) * time.Second
	}
	return accessTokenTTL
}

func (h *handlers) idTokenTTL(client *domain.Client) time.Duration {
	if client.IDTokenTTLSeconds != nil && *client.IDTokenTTLSeconds > 0 {
		return time.Duration(*client.IDTokenTTLSeconds) * time.Second
	}
	return idTokenTTL
}

func writeTokenError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if code == domain.OAuthInvalidClient {
		w.Header().Set("WWW-Authenticate", `Basic realm="token"`)
	}
	WriteOAuthError(w, status, code, description, "")
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
