package httphandler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/config"
)

func TestPKCES256RFCVector(t *testing.T) {
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := pkceS256(verifier); got != challenge {
		t.Fatalf("pkceS256 = %q, want %q", got, challenge)
	}
	if !verifyPKCE(challenge, verifier) {
		t.Fatal("RFC 7636 vector did not verify")
	}
	if verifyPKCE(challenge, verifier+"x") {
		t.Fatal("a different verifier verified")
	}
	if verifyPKCE(challenge, "short") {
		t.Fatal("a verifier below 43 characters verified")
	}
	if validPKCEVerifier("bad verifier with spaces") {
		t.Fatal("a verifier with invalid characters was accepted")
	}
}

func TestAccessTokenHashUsesLeftHalf(t *testing.T) {
	got := accessTokenHash("example-access-token")
	if len(got) != 22 {
		t.Fatalf("at_hash length = %d, want 22 base64url characters", len(got))
	}
	if got == accessTokenHash("another-token") {
		t.Fatal("at_hash did not change with the token")
	}
}

func TestScopeHelpers(t *testing.T) {
	if !scopesSubset([]string{"openid", "email"}, []string{"openid", "email", "profile"}) {
		t.Error("subset was rejected")
	}
	if scopesSubset([]string{"admin"}, []string{"openid"}) {
		t.Error("unsupported scope was accepted")
	}
	if validScopes([]string{"openid", "unknown"}) {
		t.Error("unknown scope was accepted")
	}
	if got := domain.JoinScopes(domain.SplitScopes("  openid  email openid ")); got != "openid email" {
		t.Errorf("scope normalization = %q", got)
	}
}

func TestOIDCCORSAllowsRegisteredRealmOrigin(t *testing.T) {
	clients := &stubClients{origins: []string{"https://rp.example"}}
	deps := Deps{Realms: &stubRealms{}, Clients: clients}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := OIDCCORS(deps)(next)

	request := httptest.NewRequest(http.MethodOptions, "/realms/default/token", nil)
	request.Header.Set("Origin", "https://rp.example")
	request = withDefaultRealm(request)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("allowed preflight status = %d, want 204", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://rp.example" {
		t.Errorf("allow origin = %q", got)
	}

	request = httptest.NewRequest(http.MethodPost, "/realms/default/token", nil)
	request.Header.Set("Origin", "https://evil.example")
	request = withDefaultRealm(request)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unregistered origin status = %d, want 403", recorder.Code)
	}
}

func TestSignUserTokensMapsClaimsByScope(t *testing.T) {
	now := time.Now().UTC()
	verified := now
	username := "alice"
	locale := "en"
	issuer := &captureIssuer{}
	roles := &stubRoles{groups: []domain.Group{{Name: "engineering"}}}
	h := &handlers{deps: Deps{
		Config: &config.Config{HTTP: config.HTTPConfig{ExternalURL: "https://idp.example"}},
		Tokens: issuer,
		Roles:  roles,
	}}
	realm := &domain.Realm{ID: "realm", Name: "default", Issuer: "https://idp.example/realms/default"}
	client := &domain.Client{ID: "client", ClientID: "rp", Type: domain.ClientTypePublic}
	session := &domain.SessionWithUser{
		Session: &domain.Session{
			ID: "session", UserID: "user", AuthTime: now,
			AMR: []string{domain.AMRPassword},
		},
		User: &domain.User{
			ID: "user", Email: "alice@example.com", Name: "Alice",
			GivenName: "Alice", FamilyName: "Example", Username: &username,
			Locale: &locale, EmailVerifiedAt: &verified,
		},
		Roles: []domain.Role{{Name: "platform-admin"}},
	}

	_, _, _, err := h.signUserTokens(context.Background(), realm, client, session,
		"openid profile email roles groups", now, []string{domain.AMRPassword},
		domain.ACRPassword, "nonce-value")
	if err != nil {
		t.Fatal(err)
	}
	if issuer.id.Nonce != "nonce-value" || issuer.id.ACR != domain.ACRPassword {
		t.Errorf("id token nonce/acr = %q/%q", issuer.id.Nonce, issuer.id.ACR)
	}
	if issuer.id.Email != "alice@example.com" || issuer.id.EmailVerified == nil || !*issuer.id.EmailVerified {
		t.Errorf("email claims = %q / %v", issuer.id.Email, issuer.id.EmailVerified)
	}
	if issuer.id.Name != "Alice" || issuer.id.PreferredUsername != "alice" || issuer.id.Locale != "en" {
		t.Errorf("profile claims = %+v", issuer.id)
	}
	if len(issuer.id.Roles) != 1 || issuer.id.Roles[0] != "platform-admin" {
		t.Errorf("id roles = %v", issuer.id.Roles)
	}
	if len(issuer.id.Groups) != 1 || issuer.id.Groups[0] != "engineering" {
		t.Errorf("id groups = %v", issuer.id.Groups)
	}
	if len(issuer.access.Roles) != 1 || len(issuer.access.Groups) != 1 {
		t.Errorf("access roles/groups = %v / %v", issuer.access.Roles, issuer.access.Groups)
	}
	if issuer.id.AccessTokenHash != accessTokenHash("access-token") {
		t.Errorf("at_hash = %q", issuer.id.AccessTokenHash)
	}

	if _, _, _, err := h.signUserTokens(context.Background(), realm, client, session,
		"openid", now, nil, domain.ACRPassword, ""); err != nil {
		t.Fatal(err)
	}
	if issuer.id.Email != "" || len(issuer.id.Roles) != 0 {
		t.Errorf("openid-only token leaked claims: %+v", issuer.id)
	}
}

func withDefaultRealm(request *http.Request) *http.Request {
	return request.WithContext(context.WithValue(request.Context(), realmContextKey{}, domain.DefaultRealmName))
}

type stubRealms struct{}

func (s *stubRealms) Create(context.Context, *domain.Realm) error { return nil }
func (s *stubRealms) GetByID(context.Context, domain.RealmID) (*domain.Realm, error) {
	return &domain.Realm{ID: "realm", Name: domain.DefaultRealmName}, nil
}
func (s *stubRealms) GetByName(context.Context, string) (*domain.Realm, error) {
	return &domain.Realm{ID: "realm", Name: domain.DefaultRealmName}, nil
}

type stubClients struct {
	origins []string
}

func (s *stubClients) Create(context.Context, *domain.Client, *domain.ClientSecret, []domain.ClientRedirectURI, []domain.ClientCORSOrigin) error {
	return nil
}
func (s *stubClients) GetByClientID(context.Context, domain.RealmID, string) (*domain.Client, error) {
	return &domain.Client{ID: "client", ClientID: "rp"}, nil
}
func (s *stubClients) ListByRealm(context.Context, domain.RealmID) ([]domain.Client, error) {
	return []domain.Client{{ID: "client", ClientID: "rp"}}, nil
}
func (s *stubClients) FindRedirectURI(context.Context, domain.ClientID, string, string) (bool, error) {
	return false, nil
}
func (s *stubClients) CORSOrigins(context.Context, domain.ClientID) ([]string, error) {
	return s.origins, nil
}
func (s *stubClients) AuthorizeSecret(context.Context, domain.ClientID, string) (bool, error) {
	return false, nil
}
func (s *stubClients) GetConsent(context.Context, domain.UserID, domain.ClientID) (*domain.Consent, error) {
	return nil, domain.ErrNotFound
}
func (s *stubClients) UpsertConsent(context.Context, *domain.Consent) error { return nil }

type stubRoles struct {
	groups []domain.Group
}

func (s *stubRoles) Create(context.Context, *domain.Role) error { return nil }
func (s *stubRoles) GetByName(context.Context, domain.RealmID, string) (*domain.Role, error) {
	return nil, domain.ErrNotFound
}
func (s *stubRoles) ListByRealm(context.Context, domain.RealmID) ([]domain.Role, error) {
	return nil, nil
}
func (s *stubRoles) AssignToUser(context.Context, domain.RealmID, domain.UserID, domain.RoleID) error {
	return nil
}
func (s *stubRoles) ListByUser(context.Context, domain.RealmID, domain.UserID) ([]domain.Role, error) {
	return nil, nil
}
func (s *stubRoles) ListGroupsByUser(context.Context, domain.RealmID, domain.UserID) ([]domain.Group, error) {
	return s.groups, nil
}

type captureIssuer struct {
	access domain.AccessTokenClaims
	id     domain.IDTokenClaims
}

func (c *captureIssuer) SignAccessToken(_ context.Context, _ domain.RealmID, claims domain.AccessTokenClaims) (string, error) {
	c.access = claims
	return "access-token", nil
}
func (c *captureIssuer) SignIDToken(_ context.Context, _ domain.RealmID, claims domain.IDTokenClaims) (string, error) {
	c.id = claims
	return "id-token", nil
}
func (c *captureIssuer) SignLogoutToken(context.Context, domain.RealmID, domain.LogoutToken) (string, error) {
	return "logout-token", nil
}
func (c *captureIssuer) VerifyAccessToken(context.Context, domain.RealmID, string) (*domain.AccessTokenClaims, error) {
	return nil, errors.New("unused")
}
func (c *captureIssuer) VerifyIDToken(context.Context, domain.RealmID, string) (*domain.IDTokenClaims, error) {
	return nil, errors.New("unused")
}
func (c *captureIssuer) VerifyLogoutToken(context.Context, domain.RealmID, string) (*domain.LogoutToken, error) {
	return nil, errors.New("unused")
}
func (c *captureIssuer) JWKS(context.Context, domain.RealmID) (*domain.JSONWebKeySet, error) {
	return &domain.JSONWebKeySet{}, nil
}
