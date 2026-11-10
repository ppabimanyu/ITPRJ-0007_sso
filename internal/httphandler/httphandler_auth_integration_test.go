package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/caching"
	"github.com/ppa/itprj-0007-sso/internal/infra/config"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
	"github.com/ppa/itprj-0007-sso/internal/pkg/password"
	"github.com/ppa/itprj-0007-sso/internal/repository"
	"github.com/ppa/itprj-0007-sso/internal/workerhandler"
)

type integration struct {
	cfg           *config.Config
	db            *database.DB
	redis         *redis.Client
	router        http.Handler
	server        *httptest.Server
	realms        *repository.PgRealmRepository
	users         *repository.PgUserRepository
	creds         *repository.PgCredentialRepository
	sessions      *repository.PgSessionRepository
	roles         *repository.PgRoleRepository
	tokens        *repository.PgEmailTokenRepository
	outbox        *repository.PgOutboxRepository
	clients       *repository.PgClientRepository
	keys          *repository.PgSigningKeyRepository
	refresh       *repository.PgRefreshTokenRepository
	mfa           *repository.PgMFARepository
	admin         *repository.PgAdminUserRepository
	adminClients  *repository.PgAdminClientRepository
	adminSessions *repository.PgAdminSessionRepository
	adminAudit    *repository.PgAdminAuditRepository
	groups        *repository.PgGroupRepository
	realmSettings *repository.PgRealmSettingsRepository
	ring          *pkg.Keyring
}

func newIntegration(t *testing.T) *integration {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Skip("TEST_DATABASE_URL and TEST_REDIS_URL are required")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Up(ctx); err != nil {
		db.Close()
		t.Fatalf("migrate database: %v", err)
	}
	redisClient, err := caching.New(redisURL)
	if err != nil {
		db.Close()
		t.Fatalf("create redis client: %v", err)
	}
	if err := caching.Ping(ctx, redisClient); err != nil {
		db.Close()
		_ = redisClient.Close()
		t.Fatalf("ping redis: %v", err)
	}
	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		db.Close()
		_ = redisClient.Close()
		t.Fatalf("flush redis: %v", err)
	}

	cfg := &config.Config{
		AppEnv:         "dev",
		HTTP:           config.HTTPConfig{Addr: ":0", ExternalURL: "http://localhost:5173"},
		AllowedOrigins: []string{"http://localhost:5173"},
		Password:       config.PasswordConfig{MinLength: 12},
		MasterKeys: config.MasterKeyConfig{
			Keys:  map[string][]byte{"test-only": bytes.Repeat([]byte{7}, 32)},
			KeyID: "test-only",
		},
	}
	ring, err := pkg.NewKeyring(cfg.MasterKeys.Keys, cfg.MasterKeys.KeyID)
	if err != nil {
		db.Close()
		_ = redisClient.Close()
		t.Fatalf("build test keyring: %v", err)
	}
	realms := repository.NewRealmRepository(db.Pool)
	users := repository.NewUserRepository(db.Pool)
	creds := repository.NewCredentialRepository(db.Pool)
	sessions := repository.NewSessionRepository(db.Pool)
	roles := repository.NewRoleRepository(db.Pool)
	tokens := repository.NewEmailTokenRepository(db.Pool)
	outbox := repository.NewOutboxRepository(db.Pool)
	audit := repository.NewAuditRepository(db.Pool)
	clients := repository.NewClientRepository(db.Pool)
	keys := repository.NewSigningKeyRepository(db.Pool, ring)
	authRequests := repository.NewAuthRequestRepository(redisClient)
	authCodes := repository.NewAuthCodeRepository(redisClient)
	refresh := repository.NewRefreshTokenRepository(db.Pool)
	tokenState := repository.NewTokenStateRepository(redisClient)
	mfa := repository.NewMFARepository(db.Pool)
	mfaChallenges := repository.NewMFAChallengeRepository(redisClient)
	adminUsers := repository.NewAdminUserRepository(db.Pool)
	idempotency := repository.NewIdempotencyRepository(db.Pool)
	adminClients := repository.NewAdminClientRepository(db.Pool)
	adminSessions := repository.NewAdminSessionRepository(db.Pool)
	adminAudit := repository.NewAdminAuditRepository(db.Pool)
	groups := repository.NewGroupRepository(db.Pool)
	realmSettings := repository.NewRealmSettingsRepository(db.Pool)

	router := NewRouter(Deps{
		Config:        cfg,
		DB:            db,
		Redis:         redisClient,
		Logger:        slog.New(slog.DiscardHandler),
		Realms:        realms,
		Users:         users,
		Credentials:   creds,
		Sessions:      sessions,
		Roles:         roles,
		EmailTokens:   tokens,
		Audit:         audit,
		CSRFStore:     repository.NewCSRFStore(redisClient),
		RateLimit:     repository.NewRateLimitRepository(redisClient),
		MFA:           mfa,
		MFAChallenges: mfaChallenges,
		AdminUsers:    adminUsers,
		Idempotency:   idempotency,
		Keyring:       ring,
		Clients:       clients,
		SigningKeys:   keys,
		Tokens:        keys,
		AuthRequests:  authRequests,
		AuthCodes:     authCodes,
		RefreshTokens: refresh,
		TokenState:    tokenState,
		Outbox:        outbox,
		AdminClients:  adminClients,
		AdminRoles:    repository.NewAdminRoleRepository(db.Pool),
		Groups:        groups,
		AdminSessions: adminSessions,
		AdminAudit:    adminAudit,
		RealmSettings: realmSettings,
	})
	server := httptest.NewServer(router)
	t.Cleanup(func() {
		server.Close()
		_ = redisClient.Close()
		db.Close()
	})
	return &integration{
		cfg: cfg, db: db, redis: redisClient, router: router,
		server: server, realms: realms, users: users, creds: creds,
		sessions: sessions, roles: roles, tokens: tokens, outbox: outbox,
		clients: clients, keys: keys, refresh: refresh, mfa: mfa, admin: adminUsers,
		adminClients: adminClients, adminSessions: adminSessions, adminAudit: adminAudit,
		groups: groups, realmSettings: realmSettings, ring: ring,
	}
}

func (it *integration) createRealm(t *testing.T) *domain.Realm {
	t.Helper()
	ctx := context.Background()
	realm, err := it.realms.GetByName(ctx, domain.DefaultRealmName)
	if err == nil {
		return realm
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("read default realm: %v", err)
	}
	realm = &domain.Realm{
		ID:       domain.RealmID(uuid.NewString()),
		Name:     domain.DefaultRealmName,
		Issuer:   "http://localhost:5173/realms/default",
		Settings: []byte("{}"),
	}
	if err := it.realms.Create(ctx, realm); err != nil {
		t.Fatalf("create default realm: %v", err)
	}
	return realm
}

func (it *integration) createUser(t *testing.T, realm *domain.Realm, email, plaintext string, verified bool) *domain.User {
	t.Helper()
	ctx := context.Background()
	user := &domain.User{
		ID:      domain.UserID(uuid.NewString()),
		RealmID: realm.ID,
		Email:   email,
		Name:    "Integration User",
	}
	if verified {
		now := time.Now().UTC()
		user.EmailVerifiedAt = &now
	}
	if err := it.users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = it.db.Pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1::text::uuid`, string(user.ID))
	})
	if plaintext != "" {
		params := password.DefaultParams
		params.Memory = 8 * 1024
		params.Iterations = 1
		params.Parallelism = 1
		hash, err := password.HashWithParams(plaintext, params)
		if err != nil {
			t.Fatalf("hash test password: %v", err)
		}
		if err := it.creds.Upsert(ctx, &domain.Credential{
			UserID:       user.ID,
			PasswordHash: hash,
			Algorithm:    "argon2id",
			Params:       []byte(password.ParamsJSON(params)),
		}); err != nil {
			t.Fatalf("store test credential: %v", err)
		}
	}
	return user
}

func (it *integration) client(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return &http.Client{Jar: jar, Timeout: 10 * time.Second}
}

func (it *integration) csrfToken(t *testing.T, client *http.Client) string {
	t.Helper()
	resp, body := it.request(t, client, http.MethodGet, "/api/v1/auth/csrf", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("csrf status = %d, body = %s", resp.StatusCode, body)
	}
	var payload domain.CSRFResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("csrf body: %v (%s)", err, body)
	}
	if payload.CSRFToken == "" {
		t.Fatal("csrf token is empty")
	}
	if cookie := it.cookie(t, client, csrfCookieName); cookie != payload.CSRFToken {
		t.Fatalf("csrf cookie = %q, body = %q", cookie, payload.CSRFToken)
	}
	return payload.CSRFToken
}

func (it *integration) request(t *testing.T, client *http.Client, method, path string, body any, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, it.server.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp, responseBody
}

func (it *integration) cookie(t *testing.T, client *http.Client, name string) string {
	t.Helper()
	parsed, err := url.Parse(it.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.Jar.Cookies(parsed) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func (it *integration) login(t *testing.T, client *http.Client, token, email, plaintext string) (*http.Response, []byte) {
	t.Helper()
	return it.request(t, client, http.MethodPost, "/api/v1/auth/login",
		domain.LoginRequest{Email: email, Password: plaintext},
		map[string]string{"X-CSRF-Token": token, "Origin": "http://localhost:5173"})
}

func problemCode(t *testing.T, body []byte) string {
	t.Helper()
	var problem Problem
	if err := json.Unmarshal(body, &problem); err != nil {
		t.Fatalf("body is not a problem document: %v (%s)", err, body)
	}
	return problem.Code
}

func TestIntegrationLoginSessionLogout(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "login-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)
	client := it.client(t)

	token := it.csrfToken(t, client)
	resp, body := it.login(t, client, token, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", resp.StatusCode, body)
	}
	var login domain.LoginResponse
	if err := json.Unmarshal(body, &login); err != nil {
		t.Fatalf("login body: %v (%s)", err, body)
	}
	if login.Status != domain.LoginStatusAuthenticated {
		t.Errorf("status = %q, want authenticated", login.Status)
	}
	if login.User == nil || login.User.Email != email {
		t.Errorf("user = %+v", login.User)
	}
	if it.cookie(t, client, it.cfg.SessionCookieName()) == "" {
		t.Fatal("session cookie is missing")
	}
	if it.cookie(t, client, csrfCookieName) == "" {
		t.Fatal("csrf cookie was not rotated")
	}

	resp, body = it.request(t, client, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, body = %s", resp.StatusCode, body)
	}

	sessionCSRF := it.cookie(t, client, csrfCookieName)
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/auth/logout", nil,
		map[string]string{"X-CSRF-Token": sessionCSRF, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, client, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session after logout = %d, body = %s", resp.StatusCode, body)
	}
	if code := problemCode(t, body); code != domain.CodeSessionExpired {
		t.Errorf("code = %q, want %q", code, domain.CodeSessionExpired)
	}
}

func TestIntegrationWrongPasswordIsGeneric(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "wrong-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)
	client := it.client(t)
	token := it.csrfToken(t, client)

	resp, body := it.login(t, client, token, email, "not the right password")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if code := problemCode(t, body); code != domain.CodeInvalidCredentials {
		t.Errorf("code = %q", code)
	}
	if strings.Contains(string(body), email) {
		t.Errorf("body leaks the email: %s", body)
	}
}

func TestIntegrationLockoutAfterFiveFailures(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "lock-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)
	client := it.client(t)
	token := it.csrfToken(t, client)

	for attempt := 1; attempt <= 5; attempt++ {
		resp, body := it.login(t, client, token, email, "wrong password")
		want := http.StatusUnauthorized
		if attempt == 5 {
			want = http.StatusLocked
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d status = %d, want %d, body = %s", attempt, resp.StatusCode, want, body)
		}
	}

	resp, body := it.login(t, client, token, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusLocked {
		t.Fatalf("correct password while locked = %d, body = %s", resp.StatusCode, body)
	}
	if code := problemCode(t, body); code != domain.CodeAccountLocked {
		t.Errorf("code = %q", code)
	}
}

func TestIntegrationCSRFAndOriginRejection(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "csrf-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)
	client := it.client(t)
	token := it.csrfToken(t, client)

	resp, body := it.request(t, client, http.MethodPost, "/api/v1/auth/login",
		domain.LoginRequest{Email: email, Password: "correct horse battery staple"},
		map[string]string{"Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing token status = %d, body = %s", resp.StatusCode, body)
	}
	if code := problemCode(t, body); code != domain.CodeCSRFInvalid {
		t.Errorf("code = %q", code)
	}

	resp, body = it.request(t, client, http.MethodPost, "/api/v1/auth/login",
		domain.LoginRequest{Email: email, Password: "correct horse battery staple"},
		map[string]string{"X-CSRF-Token": token, "Origin": "http://evil.example"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bad origin status = %d, body = %s", resp.StatusCode, body)
	}
	if code := problemCode(t, body); code != domain.CodeForbidden {
		t.Errorf("code = %q", code)
	}

	resp, body = it.request(t, client, http.MethodPost, "/api/v1/auth/login",
		domain.LoginRequest{Email: email, Password: "correct horse battery staple"},
		map[string]string{"X-CSRF-Token": "wrong", "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong token status = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationSessionListAndRevoke(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "sessions-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)

	first := it.client(t)
	firstToken := it.csrfToken(t, first)
	if resp, body := it.login(t, first, firstToken, email, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("first login = %d, body = %s", resp.StatusCode, body)
	}
	second := it.client(t)
	secondToken := it.csrfToken(t, second)
	if resp, body := it.login(t, second, secondToken, email, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("second login = %d, body = %s", resp.StatusCode, body)
	}

	resp, body := it.request(t, first, http.MethodGet, "/api/v1/me/sessions", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list sessions = %d, body = %s", resp.StatusCode, body)
	}
	var list domain.MeSessionList
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("session list body: %v (%s)", err, body)
	}
	if len(list.Items) != 2 {
		t.Fatalf("session count = %d, want 2", len(list.Items))
	}
	var otherID string
	currentCount := 0
	for _, item := range list.Items {
		if item.Current {
			currentCount++
		} else {
			otherID = item.ID
		}
	}
	if currentCount != 1 || otherID == "" {
		t.Fatalf("current flags wrong: %+v", list.Items)
	}

	resp, body = it.request(t, first, http.MethodDelete, "/api/v1/me/sessions/"+otherID, nil,
		map[string]string{
			"X-CSRF-Token": it.cookie(t, first, csrfCookieName),
			"Origin":       "http://localhost:5173",
		})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, second, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked session = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationChangePasswordRevokesOtherSessions(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "change-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)

	first := it.client(t)
	firstToken := it.csrfToken(t, first)
	if resp, body := it.login(t, first, firstToken, email, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("first login = %d, body = %s", resp.StatusCode, body)
	}
	second := it.client(t)
	secondToken := it.csrfToken(t, second)
	if resp, body := it.login(t, second, secondToken, email, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("second login = %d, body = %s", resp.StatusCode, body)
	}

	resp, body := it.request(t, first, http.MethodPost, "/api/v1/me/password",
		domain.ChangePasswordRequest{
			CurrentPassword: "correct horse battery staple",
			NewPassword:     "a fresh new passphrase",
		},
		map[string]string{
			"X-CSRF-Token": it.cookie(t, first, csrfCookieName),
			"Origin":       "http://localhost:5173",
		})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("change password = %d, body = %s", resp.StatusCode, body)
	}
	resp, _ = it.request(t, first, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("current session after change = %d", resp.StatusCode)
	}
	resp, _ = it.request(t, second, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("other session after change = %d", resp.StatusCode)
	}

	third := it.client(t)
	thirdToken := it.csrfToken(t, third)
	resp, body = it.login(t, third, thirdToken, email, "a fresh new passphrase")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login with new password = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = it.request(t, third, http.MethodPatch, "/api/v1/me",
		domain.ProfileUpdateRequest{Name: stringPtr("Renamed User")},
		map[string]string{
			"X-CSRF-Token": it.cookie(t, third, csrfCookieName),
			"Origin":       "http://localhost:5173",
		})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("profile update = %d, body = %s", resp.StatusCode, body)
	}
	var profile domain.UserDTO
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("profile body: %v (%s)", err, body)
	}
	if profile.Name == nil || *profile.Name != "Renamed User" {
		t.Errorf("profile name = %v", profile.Name)
	}
}

func TestIntegrationPasswordResetRevokesSessions(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "reset-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)
	client := it.client(t)
	token := it.csrfToken(t, client)
	if resp, body := it.login(t, client, token, email, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, body = %s", resp.StatusCode, body)
	}

	preSession := it.csrfToken(t, client)
	resp, body := it.request(t, client, http.MethodPost, "/api/v1/auth/forgot-password",
		domain.ForgotPasswordRequest{Email: email},
		map[string]string{"X-CSRF-Token": preSession, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("forgot password = %d, body = %s", resp.StatusCode, body)
	}

	var payload []byte
	err := it.db.Pool.QueryRow(context.Background(), `
		SELECT payload FROM outbox
		WHERE realm_id = $1::text::uuid AND type = 'email.reset'
			AND payload->>'to' = $2
		ORDER BY created_at DESC LIMIT 1`, string(realm.ID), email).Scan(&payload)
	if err != nil {
		t.Fatalf("read reset outbox payload: %v", err)
	}
	var job domain.MailJob
	if err := json.Unmarshal(payload, &job); err != nil {
		t.Fatalf("decode reset job: %v", err)
	}
	parsed, err := url.Parse(job.Link)
	if err != nil {
		t.Fatalf("parse reset link: %v", err)
	}
	resetToken := parsed.Query().Get("token")
	if resetToken == "" {
		t.Fatalf("reset link has no token: %s", job.Link)
	}

	resp, body = it.request(t, client, http.MethodPost, "/api/v1/auth/reset-password",
		domain.ResetPasswordRequest{Token: resetToken, NewPassword: "a whole new passphrase"},
		map[string]string{"X-CSRF-Token": preSession, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reset password = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, client, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session after reset = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationAcceptInviteAndVerifyEmail(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)

	inviteEmail := "invite-" + uuid.NewString() + "@example.com"
	inviteUser := it.createUser(t, realm, inviteEmail, "", false)
	inviteRaw, inviteHash, err := csrf.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := it.tokens.Create(context.Background(), &domain.EmailToken{
		ID:        uuid.NewString(),
		UserID:    inviteUser.ID,
		Purpose:   domain.EmailTokenInvite,
		TokenHash: inviteHash,
		ExpiresAt: time.Now().UTC().Add(inviteTokenLifetime),
	}, nil, nil); err != nil {
		t.Fatalf("create invite token: %v", err)
	}
	client := it.client(t)
	preSession := it.csrfToken(t, client)
	resp, body := it.request(t, client, http.MethodPost, "/api/v1/auth/accept-invite",
		domain.AcceptInviteRequest{Token: inviteRaw, Password: "invited person passphrase"},
		map[string]string{"X-CSRF-Token": preSession, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("accept invite = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.login(t, client, preSession, inviteEmail, "invited person passphrase")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login after invite = %d, body = %s", resp.StatusCode, body)
	}
	user, err := it.users.GetByID(context.Background(), inviteUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	if user.EmailVerifiedAt == nil {
		t.Error("accept invite did not verify the email")
	}

	verifyEmail := "verify-" + uuid.NewString() + "@example.com"
	verifyUser := it.createUser(t, realm, verifyEmail, "verified later passphrase", false)
	verifyRaw, verifyHash, err := csrf.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := it.tokens.Create(context.Background(), &domain.EmailToken{
		ID:        uuid.NewString(),
		UserID:    verifyUser.ID,
		Purpose:   domain.EmailTokenVerify,
		TokenHash: verifyHash,
		ExpiresAt: time.Now().UTC().Add(verifyTokenLifetime),
	}, nil, nil); err != nil {
		t.Fatalf("create verify token: %v", err)
	}
	verifyClient := it.client(t)
	verifyPre := it.csrfToken(t, verifyClient)
	resp, body = it.request(t, verifyClient, http.MethodPost, "/api/v1/auth/verify-email",
		domain.VerifyEmailRequest{Token: verifyRaw},
		map[string]string{"X-CSRF-Token": verifyPre, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("verify email = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.login(t, verifyClient, verifyPre, verifyEmail, "verified later passphrase")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login after verify = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationOutboxSendsToMailHog(t *testing.T) {
	smtpAddr := os.Getenv("TEST_SMTP_ADDR")
	if smtpAddr == "" {
		t.Skip("TEST_SMTP_ADDR is required")
	}
	host, port, err := net.SplitHostPort(smtpAddr)
	if err != nil {
		t.Fatalf("TEST_SMTP_ADDR: %v", err)
	}
	it := newIntegration(t)
	realm := it.createRealm(t)
	recipient := "mail-" + uuid.NewString() + "@example.com"
	payload, err := json.Marshal(domain.MailJob{
		To:       recipient,
		Name:     "Mail Test",
		Template: "reset",
		Locale:   "en",
		Link:     "http://localhost:5173/reset-password?token=test",
	})
	if err != nil {
		t.Fatal(err)
	}
	event := &domain.OutboxEvent{
		ID:      domain.OutboxEventID(uuid.NewString()),
		RealmID: realm.ID,
		Type:    domain.OutboxEmailReset,
		Payload: payload,
		Status:  domain.OutboxStatusPending,
	}
	if err := it.outbox.Enqueue(context.Background(), event); err != nil {
		t.Fatalf("enqueue outbox: %v", err)
	}

	sender := repository.NewSMTPSender(repository.SMTPSenderConfig{
		Host: host,
		Port: port,
		From: "no-reply@example.com",
	})
	worker := workerhandler.NewOutboxWorker(it.outbox, sender, it.keys, repository.NewLogoutSender(), slog.New(slog.DiscardHandler))
	if sent, err := worker.ProcessOnce(context.Background()); err != nil || sent == 0 {
		t.Fatalf("ProcessOnce() = %d, %v", sent, err)
	}

	var status domain.OutboxStatus
	if err := it.db.Pool.QueryRow(context.Background(),
		`SELECT status FROM outbox WHERE id = $1::text::uuid`, string(event.ID)).Scan(&status); err != nil {
		t.Fatalf("read outbox status: %v", err)
	}
	if status != domain.OutboxStatusSent {
		t.Fatalf("outbox status = %q, want sent", status)
	}

	mailhogURL := os.Getenv("TEST_MAILHOG_URL")
	if mailhogURL == "" {
		mailhogURL = "http://" + host + ":8025"
	}
	resp, err := http.Get(mailhogURL + "/api/v2/messages")
	if err != nil {
		t.Fatalf("read MailHog inbox: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	inbox, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read MailHog body: %v", err)
	}
	if !bytes.Contains(inbox, []byte(recipient)) {
		t.Fatalf("MailHog inbox does not contain %s: %s", recipient, inbox)
	}
}

func TestIntegrationForgotPasswordDoesNotRevealAccounts(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	client := it.client(t)
	token := it.csrfToken(t, client)
	missing := "missing-" + uuid.NewString() + "@example.com"
	resp, body := it.request(t, client, http.MethodPost, "/api/v1/auth/forgot-password",
		domain.ForgotPasswordRequest{Email: missing},
		map[string]string{"X-CSRF-Token": token, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var count int
	if err := it.db.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE realm_id = $1::text::uuid AND payload->>'to' = $2`,
		string(realm.ID), missing).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("unknown email created %d outbox rows", count)
	}
}

func TestIntegrationInviteTokenIsSingleUse(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "single-" + uuid.NewString() + "@example.com"
	user := it.createUser(t, realm, email, "", false)
	raw, hash, err := csrf.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := it.tokens.Create(context.Background(), &domain.EmailToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Purpose:   domain.EmailTokenInvite,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(inviteTokenLifetime),
	}, nil, nil); err != nil {
		t.Fatal(err)
	}
	client := it.client(t)
	token := it.csrfToken(t, client)
	body := domain.AcceptInviteRequest{Token: raw, Password: "single use passphrase"}
	resp, response := it.request(t, client, http.MethodPost, "/api/v1/auth/accept-invite", body,
		map[string]string{"X-CSRF-Token": token, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("first accept = %d, body = %s", resp.StatusCode, response)
	}
	resp, response = it.request(t, client, http.MethodPost, "/api/v1/auth/accept-invite", body,
		map[string]string{"X-CSRF-Token": token, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("reuse status = %d, body = %s", resp.StatusCode, response)
	}
}
