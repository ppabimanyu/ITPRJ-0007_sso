package httphandler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/repository"
)

type oidcParams struct {
	clientID    string
	redirectURI string
	verifier    string
	scope       string
	state       string
	nonce       string
	prompt      string
}

func (it *integration) ensureSigningKey(t *testing.T, realm *domain.Realm) {
	t.Helper()
	// The shared dev database may hold a key encrypted with a different master
	// keyring. Tests need a key they can decrypt, so retire any active key first.
	if _, err := it.db.Pool.Exec(context.Background(),
		`UPDATE realm_signing_key SET active = false WHERE realm_id = $1::text::uuid AND active`,
		string(realm.ID),
	); err != nil {
		t.Fatalf("retire existing signing keys: %v", err)
	}
	key, err := repository.GenerateSigningKey(it.ring, realm.ID)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	if err := it.keys.Create(context.Background(), key); err != nil {
		t.Fatalf("store signing key: %v", err)
	}
}

func (it *integration) createOIDCClient(t *testing.T, realm *domain.Realm, clientType string, thirdParty bool, backchannel string) (*domain.Client, string) {
	t.Helper()
	clientID := "rp-" + uuid.NewString()
	client := &domain.Client{
		ID:            domain.ClientID(uuid.NewString()),
		RealmID:       realm.ID,
		ClientID:      clientID,
		Type:          clientType,
		GrantTypes:    []string{domain.GrantTypeAuthorizationCode, domain.GrantTypeRefreshToken, domain.GrantTypeClientCredentials},
		AllowedScopes: []string{domain.ScopeOpenID, domain.ScopeProfile, domain.ScopeEmail, domain.ScopeOfflineAccess, domain.ScopeRoles, domain.ScopeGroups},
		ThirdParty:    thirdParty,
	}
	if backchannel != "" {
		client.BackchannelLogoutURI = &backchannel
	}
	var secret *domain.ClientSecret
	secretValue := ""
	if clientType == domain.ClientTypeConfidential {
		secretValue = "secret-" + uuid.NewString()
		secret = &domain.ClientSecret{ClientID: client.ID, SecretHash: pkg.SHA256Hex(secretValue)}
	}
	redirects := []domain.ClientRedirectURI{
		{URI: "http://localhost:5173/callback", Kind: domain.RedirectKindAuthorize},
		{URI: "http://localhost:5173/", Kind: domain.RedirectKindPostLogout},
	}
	origins := []domain.ClientCORSOrigin{{Origin: "http://localhost:5173"}}
	if err := it.clients.Create(context.Background(), client, secret, redirects, origins); err != nil {
		t.Fatalf("create oidc client: %v", err)
	}
	t.Cleanup(func() {
		_, _ = it.db.Pool.Exec(context.Background(), `DELETE FROM client WHERE id = $1::text::uuid`, string(client.ID))
	})
	return client, secretValue
}

func (it *integration) loginForOIDC(t *testing.T, email, password string) *http.Client {
	t.Helper()
	client := it.client(t)
	token := it.csrfToken(t, client)
	resp, body := it.login(t, client, token, email, password)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", resp.StatusCode, body)
	}
	return client
}

func (it *integration) noRedirect(client *http.Client) *http.Client {
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &copyClient
}

func (it *integration) form(t *testing.T, client *http.Client, path string, values url.Values, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, it.server.URL+path, strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatalf("build form request: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("form request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read form response: %v", err)
	}
	return resp, body
}

func (it *integration) beginAuthorize(t *testing.T, client *http.Client, params oidcParams) (*http.Response, []byte) {
	t.Helper()
	values := url.Values{
		"response_type":         {domain.ResponseTypeCode},
		"client_id":             {params.clientID},
		"redirect_uri":          {params.redirectURI},
		"scope":                 {params.scope},
		"state":                 {params.state},
		"nonce":                 {params.nonce},
		"code_challenge":        {pkceS256(params.verifier)},
		"code_challenge_method": {domain.CodeChallengeMethodS256},
	}
	if params.prompt != "" {
		values.Set("prompt", params.prompt)
	}
	request, err := http.NewRequest(http.MethodGet, it.server.URL+"/realms/default/authorize?"+values.Encode(), nil)
	if err != nil {
		t.Fatalf("build authorize request: %v", err)
	}
	resp, err := it.noRedirect(client).Do(request)
	if err != nil {
		t.Fatalf("authorize request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read authorize response: %v", err)
	}
	return resp, body
}

func requestIDFromLocation(t *testing.T, resp *http.Response) string {
	t.Helper()
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize location: %v", err)
	}
	requestID := location.Query().Get("request_id")
	if requestID == "" {
		t.Fatalf("authorize location has no request_id: %s", resp.Header.Get("Location"))
	}
	return requestID
}

func (it *integration) completeAuthorize(t *testing.T, client *http.Client, requestID string) domain.RedirectResponse {
	t.Helper()
	resp, body := it.request(t, client, http.MethodPost, "/api/v1/auth/authorize/complete",
		domain.CompleteAuthorizeRequest{RequestID: requestID},
		map[string]string{
			"X-CSRF-Token": it.cookie(t, client, csrfCookieName),
			"Origin":       "http://localhost:5173",
		})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete authorize = %d, body = %s", resp.StatusCode, body)
	}
	var result domain.RedirectResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode complete response: %v (%s)", err, body)
	}
	return result
}

func (it *integration) codeFromRedirect(t *testing.T, _ *http.Client, redirectTo string) string {
	t.Helper()
	location, err := url.Parse(redirectTo)
	if err != nil {
		t.Fatalf("parse code redirect: %v", err)
	}
	code := location.Query().Get("code")
	if code == "" {
		t.Fatalf("callback location has no code: %s", redirectTo)
	}
	return code
}

func (it *integration) authorizationCode(t *testing.T, client *http.Client, params oidcParams) (string, domain.RedirectResponse) {
	t.Helper()
	resp, body := it.beginAuthorize(t, client, params)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize status = %d, body = %s", resp.StatusCode, body)
	}
	requestID := requestIDFromLocation(t, resp)
	completed := it.completeAuthorize(t, client, requestID)
	return it.codeFromRedirect(t, client, completed.RedirectTo), completed
}

func (it *integration) exchangeCode(t *testing.T, client *http.Client, clientID, code, redirectURI, verifier string) (*http.Response, []byte) {
	t.Helper()
	return it.form(t, client, "/realms/default/token", url.Values{
		"grant_type":    {domain.GrantTypeAuthorizationCode},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}, nil)
}

func decodeTokenResponse(t *testing.T, body []byte) domain.TokenResponse {
	t.Helper()
	var tokens domain.TokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		t.Fatalf("decode token response: %v (%s)", err, body)
	}
	return tokens
}

func decodeOAuthError(t *testing.T, body []byte) OAuthError {
	t.Helper()
	var oauth OAuthError
	if err := json.Unmarshal(body, &oauth); err != nil {
		t.Fatalf("decode oauth error: %v (%s)", err, body)
	}
	return oauth
}

func jwtPayload(t *testing.T, raw string) map[string]any {
	t.Helper()
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a compact JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode jwt payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal jwt payload: %v", err)
	}
	return claims
}

func TestIntegrationOIDCFullCodeFlow(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	email := "oidc-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"
	it.createUser(t, realm, email, password, true)
	client, _ := it.createOIDCClient(t, realm, domain.ClientTypePublic, false, "")
	httpClient := it.loginForOIDC(t, email, password)

	params := oidcParams{
		clientID:    client.ClientID,
		redirectURI: "http://localhost:5173/callback",
		verifier:    "verifier-" + uuid.NewString() + "-verifier-0123456789",
		scope:       "openid profile email roles groups offline_access",
		state:       "state-" + uuid.NewString(),
		nonce:       "nonce-" + uuid.NewString(),
	}
	code, completed := it.authorizationCode(t, httpClient, params)
	if !strings.Contains(completed.RedirectTo, "state=") {
		t.Fatalf("redirect_to lost state: %s", completed.RedirectTo)
	}

	resp, body := it.exchangeCode(t, httpClient, client.ClientID, code, params.redirectURI, params.verifier)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token status = %d, body = %s", resp.StatusCode, body)
	}
	tokens := decodeTokenResponse(t, body)
	if tokens.AccessToken == "" || tokens.IDToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("token response is incomplete: %+v", tokens)
	}
	idClaims := jwtPayload(t, tokens.IDToken)
	if idClaims["nonce"] != params.nonce || idClaims["acr"] != domain.ACRPassword {
		t.Errorf("id token nonce/acr = %v / %v", idClaims["nonce"], idClaims["acr"])
	}
	if idClaims["email"] != email {
		t.Errorf("id token email = %v", idClaims["email"])
	}
	if idClaims["at_hash"] != accessTokenHash(tokens.AccessToken) {
		t.Errorf("id token at_hash does not match the access token")
	}
	if idClaims["sid"] == "" {
		t.Error("id token has no sid")
	}

	request, _ := http.NewRequest(http.MethodGet, it.server.URL+"/realms/default/userinfo", nil)
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	userinfoResp, err := httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	userinfoBody, _ := io.ReadAll(userinfoResp.Body)
	_ = userinfoResp.Body.Close()
	if userinfoResp.StatusCode != http.StatusOK {
		t.Fatalf("userinfo status = %d, body = %s", userinfoResp.StatusCode, userinfoBody)
	}
	var userinfo map[string]any
	if err := json.Unmarshal(userinfoBody, &userinfo); err != nil {
		t.Fatal(err)
	}
	if userinfo["email"] != email {
		t.Errorf("userinfo email = %v", userinfo["email"])
	}

	refreshResp, refreshBody := it.form(t, httpClient, "/realms/default/token", url.Values{
		"grant_type":    {domain.GrantTypeRefreshToken},
		"client_id":     {client.ClientID},
		"refresh_token": {tokens.RefreshToken},
	}, nil)
	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshResp.StatusCode, refreshBody)
	}
	rotated := decodeTokenResponse(t, refreshBody)
	if rotated.RefreshToken == "" || rotated.RefreshToken == tokens.RefreshToken {
		t.Fatalf("refresh token did not rotate")
	}

	reuseResp, reuseBody := it.form(t, httpClient, "/realms/default/token", url.Values{
		"grant_type":    {domain.GrantTypeRefreshToken},
		"client_id":     {client.ClientID},
		"refresh_token": {tokens.RefreshToken},
	}, nil)
	if reuseResp.StatusCode != http.StatusBadRequest ||
		decodeOAuthError(t, reuseBody).Error != domain.OAuthInvalidGrant {
		t.Fatalf("refresh reuse = %d %s", reuseResp.StatusCode, reuseBody)
	}
	familyResp, familyBody := it.form(t, httpClient, "/realms/default/token", url.Values{
		"grant_type":    {domain.GrantTypeRefreshToken},
		"client_id":     {client.ClientID},
		"refresh_token": {rotated.RefreshToken},
	}, nil)
	if familyResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("grouped family survived reuse: %d %s", familyResp.StatusCode, familyBody)
	}

	reuseCodeResp, reuseCodeBody := it.exchangeCode(t, httpClient, client.ClientID, code, params.redirectURI, params.verifier)
	if reuseCodeResp.StatusCode != http.StatusBadRequest ||
		decodeOAuthError(t, reuseCodeBody).Error != domain.OAuthInvalidGrant {
		t.Fatalf("code reuse = %d %s", reuseCodeResp.StatusCode, reuseCodeBody)
	}

	badPKCEParams := params
	badPKCEParams.verifier = "another-verifier-" + uuid.NewString() + "-0123456789"
	badCode, _ := it.authorizationCode(t, httpClient, badPKCEParams)
	badPKCEResp, badPKCEBody := it.exchangeCode(t, httpClient, client.ClientID, badCode, params.redirectURI, "wrong-verifier-"+uuid.NewString()+"-0123456789")
	if badPKCEResp.StatusCode != http.StatusBadRequest ||
		decodeOAuthError(t, badPKCEBody).Error != domain.OAuthInvalidGrant {
		t.Fatalf("wrong PKCE = %d %s", badPKCEResp.StatusCode, badPKCEBody)
	}

	wrongRedirectParams := params
	wrongRedirectParams.redirectURI = "http://evil.example/callback"
	wrongResp, wrongBody := it.beginAuthorize(t, httpClient, wrongRedirectParams)
	if wrongResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong redirect = %d %s", wrongResp.StatusCode, wrongBody)
	}
	if code := problemCode(t, wrongBody); code != domain.CodeBadRequest {
		t.Errorf("wrong redirect problem code = %q", code)
	}
}

func TestIntegrationOIDCClientCredentials(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	client, secret := it.createOIDCClient(t, realm, domain.ClientTypeConfidential, false, "")
	httpClient := it.client(t)

	values := url.Values{
		"grant_type": {domain.GrantTypeClientCredentials},
		"scope":      {"openid profile"},
	}
	request, err := http.NewRequest(http.MethodPost, it.server.URL+"/realms/default/token", strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(url.QueryEscape(client.ClientID), url.QueryEscape(secret))
	resp, err := httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client credentials = %d, body = %s", resp.StatusCode, body)
	}
	tokens := decodeTokenResponse(t, body)
	if tokens.AccessToken == "" {
		t.Fatal("client credentials returned no access token")
	}
	if tokens.RefreshToken != "" || tokens.IDToken != "" {
		t.Fatalf("client credentials returned forbidden tokens: %+v", tokens)
	}

	postResp, postBody := it.form(t, httpClient, "/realms/default/token", url.Values{
		"grant_type":    {domain.GrantTypeClientCredentials},
		"client_id":     {client.ClientID},
		"client_secret": {secret},
		"scope":         {"openid"},
	}, nil)
	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("client_secret_post = %d, body = %s", postResp.StatusCode, postBody)
	}

	introspectResp, introspectBody := it.form(t, httpClient, "/realms/default/introspect", url.Values{
		"client_id":       {client.ClientID},
		"client_secret":   {secret},
		"token":           {tokens.AccessToken},
		"token_type_hint": {"access_token"},
	}, nil)
	if introspectResp.StatusCode != http.StatusOK {
		t.Fatalf("introspect = %d, body = %s", introspectResp.StatusCode, introspectBody)
	}
	var introspection domain.IntrospectionResponse
	if err := json.Unmarshal(introspectBody, &introspection); err != nil {
		t.Fatal(err)
	}
	if !introspection.Active || introspection.ClientID != client.ClientID {
		t.Fatalf("introspection = %+v", introspection)
	}
}

func TestIntegrationOIDCDiscoveryJwksAndCORS(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	it.createOIDCClient(t, realm, domain.ClientTypePublic, false, "")
	httpClient := it.client(t)

	resp, body := it.request(t, httpClient, http.MethodGet, "/realms/default/.well-known/openid-configuration", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("discovery = %d, body = %s", resp.StatusCode, body)
	}
	var discovery map[string]any
	if err := json.Unmarshal(body, &discovery); err != nil {
		t.Fatal(err)
	}
	if discovery["issuer"] != realm.Issuer {
		t.Errorf("issuer = %v, want %s", discovery["issuer"], realm.Issuer)
	}
	if len(discovery["grant_types_supported"].([]any)) != 3 {
		t.Errorf("grant types = %v", discovery["grant_types_supported"])
	}
	if discovery["code_challenge_methods_supported"].([]any)[0] != domain.CodeChallengeMethodS256 {
		t.Errorf("pkce methods = %v", discovery["code_challenge_methods_supported"])
	}

	jwksResp, jwksBody := it.request(t, httpClient, http.MethodGet, "/realms/default/jwks", nil, nil)
	if jwksResp.StatusCode != http.StatusOK {
		t.Fatalf("jwks = %d, body = %s", jwksResp.StatusCode, jwksBody)
	}
	var jwks domain.JSONWebKeySet
	if err := json.Unmarshal(jwksBody, &jwks); err != nil {
		t.Fatal(err)
	}
	active, err := it.keys.GetActive(context.Background(), realm.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, key := range jwks.Keys {
		if key.Kid == active.KID {
			found = true
		}
	}
	if !found {
		t.Fatalf("jwks does not contain active kid %s", active.KID)
	}

	preflight, _ := http.NewRequest(http.MethodOptions, it.server.URL+"/realms/default/token", nil)
	preflight.Header.Set("Origin", "http://localhost:5173")
	preflightResp, err := httpClient.Do(preflight)
	if err != nil {
		t.Fatal(err)
	}
	_ = preflightResp.Body.Close()
	if preflightResp.StatusCode != http.StatusNoContent ||
		preflightResp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("allowed preflight = %d %q", preflightResp.StatusCode, preflightResp.Header.Get("Access-Control-Allow-Origin"))
	}
	preflight.Header.Set("Origin", "http://evil.example")
	evilResp, err := httpClient.Do(preflight)
	if err != nil {
		t.Fatal(err)
	}
	_ = evilResp.Body.Close()
	if evilResp.StatusCode != http.StatusForbidden {
		t.Fatalf("unregistered origin = %d, want 403", evilResp.StatusCode)
	}

}

func TestIntegrationOIDCConsentForThirdParty(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	email := "consent-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"
	it.createUser(t, realm, email, password, true)
	client, _ := it.createOIDCClient(t, realm, domain.ClientTypePublic, true, "")
	httpClient := it.loginForOIDC(t, email, password)
	params := oidcParams{
		clientID:    client.ClientID,
		redirectURI: "http://localhost:5173/callback",
		verifier:    "verifier-" + uuid.NewString() + "-0123456789",
		scope:       "openid profile email",
		state:       "state",
		nonce:       "nonce",
	}
	resp, body := it.beginAuthorize(t, httpClient, params)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.HasPrefix(resp.Header.Get("Location"), "/consent") {
		t.Fatalf("third-party authorize did not route to consent: %s", resp.Header.Get("Location"))
	}
	requestID := requestIDFromLocation(t, resp)

	infoResp, infoBody := it.request(t, httpClient, http.MethodGet,
		"/api/v1/auth/authorize/request?request_id="+url.QueryEscape(requestID), nil, nil)
	if infoResp.StatusCode != http.StatusOK {
		t.Fatalf("authorize request info = %d, body = %s", infoResp.StatusCode, infoBody)
	}
	var info domain.AuthRequestInfo
	if err := json.Unmarshal(infoBody, &info); err != nil {
		t.Fatal(err)
	}
	if !info.ConsentRequired {
		t.Fatal("authorize request info did not report consent_required")
	}

	consentResp, consentBody := it.request(t, httpClient, http.MethodPost, "/api/v1/auth/consent",
		domain.ConsentRequest{RequestID: requestID, Scopes: []string{domain.ScopeOpenID, domain.ScopeProfile, domain.ScopeEmail}},
		map[string]string{
			"X-CSRF-Token": it.cookie(t, httpClient, csrfCookieName),
			"Origin":       "http://localhost:5173",
		})
	if consentResp.StatusCode != http.StatusOK {
		t.Fatalf("consent = %d, body = %s", consentResp.StatusCode, consentBody)
	}
	var completed domain.RedirectResponse
	if err := json.Unmarshal(consentBody, &completed); err != nil {
		t.Fatal(err)
	}
	code := it.codeFromRedirect(t, httpClient, completed.RedirectTo)
	tokenResp, tokenBody := it.exchangeCode(t, httpClient, client.ClientID, code, params.redirectURI, params.verifier)
	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("consent code exchange = %d, body = %s", tokenResp.StatusCode, tokenBody)
	}

	secondResp, secondBody := it.beginAuthorize(t, httpClient, params)
	if secondResp.StatusCode != http.StatusFound {
		t.Fatalf("second authorize = %d, body = %s", secondResp.StatusCode, secondBody)
	}
	if !strings.HasPrefix(secondResp.Header.Get("Location"), "/login") {
		t.Fatalf("consent was not remembered: %s", secondResp.Header.Get("Location"))
	}
}

func TestIntegrationOIDCLogoutAndBackchannel(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	email := "logout-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"
	it.createUser(t, realm, email, password, true)
	client, _ := it.createOIDCClient(t, realm, domain.ClientTypePublic, false, "https://rp.example/backchannel")
	httpClient := it.loginForOIDC(t, email, password)
	params := oidcParams{
		clientID:    client.ClientID,
		redirectURI: "http://localhost:5173/callback",
		verifier:    "verifier-" + uuid.NewString() + "-0123456789",
		scope:       "openid profile email offline_access",
		state:       "state",
		nonce:       "nonce",
	}
	code, _ := it.authorizationCode(t, httpClient, params)
	resp, body := it.exchangeCode(t, httpClient, client.ClientID, code, params.redirectURI, params.verifier)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token = %d, body = %s", resp.StatusCode, body)
	}
	tokens := decodeTokenResponse(t, body)

	logoutURL := it.server.URL + "/realms/default/logout?" + url.Values{
		"id_token_hint":            {tokens.IDToken},
		"client_id":                {client.ClientID},
		"post_logout_redirect_uri": {"http://localhost:5173/"},
		"state":                    {"bye"},
	}.Encode()
	request, _ := http.NewRequest(http.MethodGet, logoutURL, nil)
	logoutResp, err := it.noRedirect(httpClient).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusFound {
		t.Fatalf("logout = %d", logoutResp.StatusCode)
	}
	if !strings.Contains(logoutResp.Header.Get("Location"), "state=bye") {
		t.Errorf("logout redirect lost state: %s", logoutResp.Header.Get("Location"))
	}

	var count int
	if err := it.db.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM outbox
		WHERE realm_id = $1::text::uuid AND type = $2`,
		string(realm.ID), domain.OutboxLogoutBackchannel).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("logout did not enqueue a backchannel event")
	}

	sessionResp, sessionBody := it.request(t, httpClient, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if sessionResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session after logout = %d, body = %s", sessionResp.StatusCode, sessionBody)
	}
}

func TestIntegrationOIDCRevokeAndIntrospect(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	email := "revoke-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"
	user := it.createUser(t, realm, email, password, true)
	client, _ := it.createOIDCClient(t, realm, domain.ClientTypePublic, false, "")
	httpClient := it.loginForOIDC(t, email, password)
	params := oidcParams{
		clientID:    client.ClientID,
		redirectURI: "http://localhost:5173/callback",
		verifier:    "verifier-" + uuid.NewString() + "-0123456789",
		scope:       "openid email offline_access",
		state:       "state",
	}
	code, _ := it.authorizationCode(t, httpClient, params)
	resp, body := it.exchangeCode(t, httpClient, client.ClientID, code, params.redirectURI, params.verifier)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token = %d, body = %s", resp.StatusCode, body)
	}
	tokens := decodeTokenResponse(t, body)

	introspect := func(raw string) domain.IntrospectionResponse {
		t.Helper()
		response, payload := it.form(t, httpClient, "/realms/default/introspect", url.Values{
			"client_id": {client.ClientID},
			"token":     {raw},
		}, nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("introspect = %d, body = %s", response.StatusCode, payload)
		}
		var result domain.IntrospectionResponse
		if err := json.Unmarshal(payload, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if !introspect(tokens.AccessToken).Active || !introspect(tokens.RefreshToken).Active {
		t.Fatal("fresh tokens are not active")
	}

	for _, token := range []string{tokens.AccessToken, tokens.RefreshToken} {
		response, payload := it.form(t, httpClient, "/realms/default/revoke", url.Values{
			"client_id": {client.ClientID},
			"token":     {token},
		}, nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("revoke = %d, body = %s", response.StatusCode, payload)
		}
	}
	if introspect(tokens.AccessToken).Active {
		t.Error("revoked access token is still active")
	}
	if introspect(tokens.RefreshToken).Active {
		t.Error("revoked refresh token is still active")
	}

	request, _ := http.NewRequest(http.MethodGet, it.server.URL+"/realms/default/userinfo", nil)
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	userinfoResp, err := httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = userinfoResp.Body.Close()
	if userinfoResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("userinfo after revoke = %d, want 401", userinfoResp.StatusCode)
	}
	_ = user
}

func TestIntegrationOIDCBackchannelLogoutInbound(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	email := "backchannel-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"
	user := it.createUser(t, realm, email, password, true)
	client, _ := it.createOIDCClient(t, realm, domain.ClientTypePublic, false, "")
	httpClient := it.loginForOIDC(t, email, password)
	params := oidcParams{
		clientID:    client.ClientID,
		redirectURI: "http://localhost:5173/callback",
		verifier:    "verifier-" + uuid.NewString() + "-0123456789",
		scope:       "openid",
		state:       "state",
	}
	code, _ := it.authorizationCode(t, httpClient, params)
	resp, body := it.exchangeCode(t, httpClient, client.ClientID, code, params.redirectURI, params.verifier)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token = %d, body = %s", resp.StatusCode, body)
	}
	tokens := decodeTokenResponse(t, body)
	idClaims, err := it.keys.VerifyIDToken(context.Background(), realm.ID, tokens.IDToken)
	if err != nil {
		t.Fatal(err)
	}
	logoutToken, err := it.keys.SignLogoutToken(context.Background(), realm.ID, domain.LogoutToken{
		Issuer:    realm.Issuer,
		Subject:   string(user.ID),
		Audience:  client.ClientID,
		IssuedAt:  time.Now().UTC().Unix(),
		JTI:       uuid.NewString(),
		Events:    map[string]any{domain.BackchannelLogoutEvent: map[string]any{}},
		SessionID: idClaims.SessionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, payload := it.form(t, httpClient, "/realms/default/logout/backchannel", url.Values{
		"logout_token": {logoutToken},
	}, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("inbound backchannel = %d, body = %s", response.StatusCode, payload)
	}
	sessionResp, sessionBody := it.request(t, httpClient, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if sessionResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session after inbound backchannel = %d, body = %s", sessionResp.StatusCode, sessionBody)
	}
}

func TestIntegrationOIDCPromptNone(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	it.ensureSigningKey(t, realm)
	email := "prompt-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"
	it.createUser(t, realm, email, password, true)
	client, _ := it.createOIDCClient(t, realm, domain.ClientTypePublic, false, "")
	httpClient := it.loginForOIDC(t, email, password)
	params := oidcParams{
		clientID:    client.ClientID,
		redirectURI: "http://localhost:5173/callback",
		verifier:    "verifier-" + uuid.NewString() + "-0123456789",
		scope:       "openid",
		state:       "silent",
		prompt:      "none",
	}
	resp, body := it.beginAuthorize(t, httpClient, params)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("prompt=none with a session = %d, body = %s", resp.StatusCode, body)
	}
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("code") == "" {
		t.Fatalf("prompt=none did not issue a code: %s", resp.Header.Get("Location"))
	}

	anonymous := it.client(t)
	resp, body = it.beginAuthorize(t, anonymous, params)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("prompt=none without a session = %d, body = %s", resp.StatusCode, body)
	}
	location, err = url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("error") != domain.OAuthLoginRequired {
		t.Fatalf("prompt=none without a session error = %q", location.Query().Get("error"))
	}
	if location.Query().Get("state") != params.state {
		t.Errorf("prompt=none error redirect lost state")
	}
}
