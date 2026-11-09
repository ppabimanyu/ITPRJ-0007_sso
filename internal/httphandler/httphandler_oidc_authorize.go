package httphandler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

func (h *handlers) authorize(w http.ResponseWriter, r *http.Request) {
	realm, err := h.realm(r.Context())
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "realm was not found")
		return
	}
	query := r.URL.Query()
	clientID := strings.TrimSpace(query.Get("client_id"))
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")
	if clientID == "" || redirectURI == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"client_id and redirect_uri are required")
		return
	}

	client, err := h.deps.Clients.GetByClientID(r.Context(), realm.ID, clientID)
	if err != nil {
		h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeSystem, nil, domain.AuditAuthorizeDenied,
			map[string]any{"reason": "unknown_client"})
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "client is unknown")
		return
	}
	if client.DisabledAt != nil {
		h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeClient, &client.ClientID,
			domain.AuditAuthorizeDenied, map[string]any{"reason": "client_disabled"})
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "client is disabled")
		return
	}
	registered, err := h.deps.Clients.FindRedirectURI(r.Context(), client.ID, redirectURI, domain.RedirectKindAuthorize)
	if err != nil {
		h.logInternal(r.Context(), "redirect_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	if !registered {
		h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeClient, &client.ClientID,
			domain.AuditAuthorizeDenied, map[string]any{"reason": "redirect_uri_mismatch"})
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"redirect_uri is not registered for this client")
		return
	}

	deny := func(code, description, reason string) {
		h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeClient, &client.ClientID,
			domain.AuditAuthorizeDenied, map[string]any{"reason": reason})
		redirectOAuthError(w, r, redirectURI, state, code, description)
	}

	if responseType := query.Get("response_type"); responseType != domain.ResponseTypeCode {
		deny(domain.OAuthUnsupportedResponse, "only response_type=code is supported", "response_type")
		return
	}
	if mode := query.Get("response_mode"); mode != "" && mode != domain.ResponseModeQuery {
		deny(domain.OAuthInvalidRequest, "only response_mode=query is supported", "response_mode")
		return
	}
	scopes := domain.SplitScopes(query.Get("scope"))
	if len(scopes) == 0 || !hasScope(scopes, domain.ScopeOpenID) {
		deny(domain.OAuthInvalidScope, "the openid scope is required", "scope")
		return
	}
	if !validScopes(scopes) || !scopesSubset(scopes, client.AllowedScopes) {
		deny(domain.OAuthInvalidScope, "one or more scopes are not allowed", "scope")
		return
	}
	if hasScope(scopes, domain.ScopeOfflineAccess) && !hasScope(client.GrantTypes, domain.GrantTypeRefreshToken) {
		deny(domain.OAuthInvalidScope, "offline_access is not allowed for this client", "scope")
		return
	}

	challenge := query.Get("code_challenge")
	method := query.Get("code_challenge_method")
	if !validPKCEVerifier(challenge) || method != domain.CodeChallengeMethodS256 {
		deny(domain.OAuthInvalidRequest, "PKCE with S256 is required", "pkce")
		return
	}
	if len(state) > 2048 || len(query.Get("nonce")) > 512 {
		deny(domain.OAuthInvalidRequest, "state or nonce is too long", "state")
		return
	}

	prompt := strings.TrimSpace(query.Get("prompt"))
	switch prompt {
	case "", "none", "login", "consent":
	default:
		deny(domain.OAuthInvalidRequest, "unsupported prompt value", "prompt")
		return
	}
	if prompt == "consent" && !client.ThirdParty {
		deny(domain.OAuthInvalidRequest, "prompt=consent is only supported for third-party clients", "prompt")
		return
	}
	var maxAge *int
	if raw := query.Get("max_age"); raw != "" {
		value, ok := parseNonNegativeInt(raw)
		if !ok {
			deny(domain.OAuthInvalidRequest, "max_age must be a non-negative integer", "max_age")
			return
		}
		maxAge = &value
	}

	now := nowUTC()
	current, err := h.optionalSession(r.Context(), r)
	if err != nil {
		h.logInternal(r.Context(), "session_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	loginRequired := current == nil || prompt == "login"
	if current != nil && maxAge != nil {
		if current.Session.AuthTime.Before(now.Add(-time.Duration(*maxAge) * time.Second)) {
			loginRequired = true
		}
	}
	needConsent := false
	if current != nil && client.ThirdParty {
		consent, err := h.deps.Clients.GetConsent(r.Context(), current.User.ID, client.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			h.logInternal(r.Context(), "consent_lookup_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		needConsent = !consentCovers(consent, scopes)
	}

	request := &domain.AuthRequest{
		RealmID:             realm.ID,
		ClientID:            client.ID,
		ClientIDString:      client.ClientID,
		RedirectURI:         redirectURI,
		Scopes:              scopes,
		State:               state,
		Nonce:               query.Get("nonce"),
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
		Prompt:              prompt,
		MaxAgeSeconds:       maxAge,
		LoginHint:           query.Get("login_hint"),
		CreatedAt:           now,
		ExpiresAt:           now.Add(authRequestTTL),
	}
	if current != nil {
		request.SessionID = &current.Session.ID
		request.UserID = &current.User.ID
		request.AuthTime = &current.Session.AuthTime
	}

	if prompt == "none" {
		if loginRequired {
			deny(domain.OAuthLoginRequired, "no active session", "prompt_none_login")
			return
		}
		if needConsent {
			deny(domain.OAuthInteractionRequired, "consent is required", "prompt_none_consent")
			return
		}
		redirect, err := h.issueAuthorizationCode(r.Context(), realm.ID, client, request, current, scopes)
		if err != nil {
			h.logInternal(r.Context(), "auth_code_issue_failed", err)
			WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
				"authorization codes are unavailable")
			return
		}
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}

	requestID, err := pkg.RandomToken(16)
	if err != nil {
		h.logInternal(r.Context(), "auth_request_id_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	browserSecret, err := pkg.RandomToken(32)
	if err != nil {
		h.logInternal(r.Context(), "auth_request_secret_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	request.ID = requestID
	request.BrowserHash = pkg.SHA256Hex(browserSecret)
	if h.deps.AuthRequests == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"authorization requests are unavailable")
		return
	}
	if err := h.deps.AuthRequests.Put(r.Context(), request); err != nil {
		logger.FromContext(r.Context()).Error("auth_request_store_failed", "error", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"authorization requests are unavailable")
		return
	}
	setAuthRequestCookie(w, h.deps.Config, browserSecret)

	target := "/login?request_id=" + url.QueryEscape(requestID)
	if needConsent {
		target = "/consent?request_id=" + url.QueryEscape(requestID)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *handlers) issueAuthorizationCode(ctx context.Context, realmID domain.RealmID, client *domain.Client, request *domain.AuthRequest, current *CurrentSession, scopes []string) (string, error) {
	if current == nil || current.Session == nil || current.User == nil {
		return "", domain.ErrNotFound
	}
	if h.deps.AuthCodes == nil {
		return "", domain.ErrNotFound
	}
	rawCode, err := pkg.RandomToken(32)
	if err != nil {
		return "", err
	}
	now := nowUTC()
	code := &domain.AuthCode{
		RealmID:        realmID,
		ClientID:       client.ID,
		ClientIDString: client.ClientID,
		RedirectURI:    request.RedirectURI,
		Scopes:         scopes,
		Nonce:          request.Nonce,
		CodeChallenge:  request.CodeChallenge,
		SessionID:      current.Session.ID,
		UserID:         current.User.ID,
		AuthTime:       current.Session.AuthTime,
		AMR:            current.Session.AMR,
		ACR:            sessionACR(current.Session.AMR),
		ExpiresAt:      now.Add(authCodeTTL),
	}
	if err := h.deps.AuthCodes.Put(ctx, pkg.SHA256Hex(rawCode), code); err != nil {
		return "", err
	}
	if request.ID != "" {
		if err := h.deps.AuthRequests.Delete(ctx, request.ID); err != nil {
			logger.FromContext(ctx).Warn("auth_request_delete_failed", "error", err)
		}
	}
	h.auditOIDC(ctx, realmID, domain.ActorTypeUser, userActor(current.User.ID),
		domain.AuditAuthCodeIssued, map[string]any{
			"client_id": client.ClientID,
			"scopes":    scopes,
			"sid":       string(current.Session.ID),
		})
	return authorizeSuccessRedirect(request.RedirectURI, rawCode, request.State), nil
}
