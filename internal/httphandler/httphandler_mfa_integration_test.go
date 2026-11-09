package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg/totp"
)

func (it *integration) ensureRole(t *testing.T, realm *domain.Realm, name string) *domain.Role {
	t.Helper()
	role, err := it.roles.GetByName(context.Background(), realm.ID, name)
	if err == nil {
		return role
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("read role %s: %v", name, err)
	}
	role = &domain.Role{
		ID:      domain.RoleID(uuid.NewString()),
		RealmID: realm.ID,
		Name:    name,
	}
	if err := it.roles.Create(context.Background(), role); err != nil {
		t.Fatalf("create role %s: %v", name, err)
	}
	return role
}

func (it *integration) createAdmin(t *testing.T, realm *domain.Realm, email, password string) *domain.User {
	t.Helper()
	user := it.createUser(t, realm, email, password, true)
	role := it.ensureRole(t, realm, domain.PlatformAdminRole)
	if err := it.roles.AssignToUser(context.Background(), realm.ID, user.ID, role.ID); err != nil {
		t.Fatalf("assign platform-admin: %v", err)
	}
	return user
}

func (it *integration) loginAndEnrollAdmin(t *testing.T, client *http.Client, email, password string) (*domain.LoginResponse, []string) {
	t.Helper()
	token := it.csrfToken(t, client)
	resp, body := it.login(t, client, token, email, password)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin login = %d, body = %s", resp.StatusCode, body)
	}
	var login domain.LoginResponse
	if err := json.Unmarshal(body, &login); err != nil {
		t.Fatalf("login body: %v (%s)", err, body)
	}
	if login.Status != domain.LoginStatusMFAEnrollmentRequired {
		t.Fatalf("admin login status = %q, want enrollment required", login.Status)
	}
	if login.User == nil {
		t.Fatal("enrollment response has no user")
	}
	if it.cookie(t, client, it.cfg.SessionCookieName()) == "" {
		t.Fatal("restricted session cookie is missing")
	}
	sessionToken := it.cookie(t, client, csrfCookieName)
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/auth/mfa/enroll",
		domain.MFAEnrollRequest{Action: "start"},
		map[string]string{"X-CSRF-Token": sessionToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enroll start = %d, body = %s", resp.StatusCode, body)
	}
	var started domain.MFAEnrollResponse
	if err := json.Unmarshal(body, &started); err != nil {
		t.Fatalf("enroll start body: %v (%s)", err, body)
	}
	if started.Secret == nil || started.OTPAuthURI == nil {
		t.Fatalf("enroll start is missing secret or uri: %+v", started)
	}
	code, err := totp.Code(*started.Secret, totp.Step(time.Now().UTC()))
	if err != nil {
		t.Fatalf("compute totp code: %v", err)
	}
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/auth/mfa/enroll",
		domain.MFAEnrollRequest{Action: "confirm", Code: code},
		map[string]string{"X-CSRF-Token": sessionToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enroll confirm = %d, body = %s", resp.StatusCode, body)
	}
	var confirmed domain.MFAEnrollResponse
	if err := json.Unmarshal(body, &confirmed); err != nil {
		t.Fatalf("enroll confirm body: %v (%s)", err, body)
	}
	if len(confirmed.RecoveryCodes) != totp.RecoveryCodeCount {
		t.Fatalf("recovery code count = %d, want %d", len(confirmed.RecoveryCodes), totp.RecoveryCodeCount)
	}
	return &login, confirmed.RecoveryCodes
}

func TestIntegrationForcedAdminEnrollment(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "admin-enroll-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, email, "correct horse battery staple")
	client := it.client(t)

	_, codes := it.loginAndEnrollAdmin(t, client, email, "correct horse battery staple")
	if len(codes) != totp.RecoveryCodeCount {
		t.Fatalf("recovery codes = %d", len(codes))
	}
	resp, body := it.request(t, client, http.MethodGet, "/api/v1/users", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin users after enrollment = %d, body = %s", resp.StatusCode, body)
	}
	var list domain.AdminUserList
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("user list body: %v (%s)", err, body)
	}
	if list.Items == nil {
		t.Fatal("user list items is null")
	}
}

func TestIntegrationRestrictedAdminSessionIsBlocked(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "admin-blocked-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, email, "correct horse battery staple")
	client := it.client(t)
	token := it.csrfToken(t, client)
	resp, body := it.login(t, client, token, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, body = %s", resp.StatusCode, body)
	}
	var login domain.LoginResponse
	if err := json.Unmarshal(body, &login); err != nil {
		t.Fatalf("login body: %v (%s)", err, body)
	}
	if login.Status != domain.LoginStatusMFAEnrollmentRequired {
		t.Fatalf("status = %q", login.Status)
	}
	if login.SessionExpiresAt == nil {
		t.Fatal("restricted session has no expiry")
	}
	resp, body = it.request(t, client, http.MethodGet, "/api/v1/users", nil, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("admin route = %d, body = %s", resp.StatusCode, body)
	}
	if code := problemCode(t, body); code != domain.CodeMFARequired {
		t.Fatalf("code = %q, want %q", code, domain.CodeMFARequired)
	}
	resp, body = it.request(t, client, http.MethodGet, "/api/v1/me", nil, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("self-service route = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationMFALoginAndFailedAttempts(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "admin-login-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, email, "correct horse battery staple")

	enrollClient := it.client(t)
	_, codes := it.loginAndEnrollAdmin(t, enrollClient, email, "correct horse battery staple")
	if err := it.requestNoContent(t, enrollClient, http.MethodPost, "/api/v1/auth/logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	client := it.client(t)
	token := it.csrfToken(t, client)
	resp, body := it.login(t, client, token, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mfa login = %d, body = %s", resp.StatusCode, body)
	}
	var challenge domain.LoginResponse
	if err := json.Unmarshal(body, &challenge); err != nil {
		t.Fatalf("challenge body: %v (%s)", err, body)
	}
	if challenge.Status != domain.LoginStatusMFARequired {
		t.Fatalf("status = %q", challenge.Status)
	}
	if challenge.MFAToken == nil || challenge.User != nil || challenge.SessionExpiresAt != nil {
		t.Fatalf("challenge shape = %+v", challenge)
	}
	if it.cookie(t, client, it.cfg.SessionCookieName()) != "" {
		t.Fatal("a session was created before MFA completed")
	}

	record, err := it.mfa.GetTOTP(context.Background(), mustUserID(t, it, realm.ID, email))
	if err != nil {
		t.Fatalf("read totp: %v", err)
	}
	secret, err := it.ring.Decrypt(record.SecretEncrypted)
	if err != nil {
		t.Fatalf("decrypt totp: %v", err)
	}
	code, err := totp.Code(string(secret), totp.Step(time.Now().UTC())+1)
	if err != nil {
		t.Fatal(err)
	}
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/auth/mfa/verify",
		domain.MFAVerifyRequest{MFAToken: *challenge.MFAToken, Code: code},
		map[string]string{"X-CSRF-Token": token, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mfa verify = %d, body = %s", resp.StatusCode, body)
	}
	var verified domain.MFAVerifyResponse
	if err := json.Unmarshal(body, &verified); err != nil {
		t.Fatalf("verify body: %v (%s)", err, body)
	}
	if verified.Status != domain.LoginStatusAuthenticated || it.cookie(t, client, it.cfg.SessionCookieName()) == "" {
		t.Fatalf("verify status = %q", verified.Status)
	}
	resp, body = it.request(t, client, http.MethodGet, "/api/v1/users", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin after mfa = %d, body = %s", resp.StatusCode, body)
	}

	if err := it.requestNoContent(t, client, http.MethodPost, "/api/v1/auth/logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	recovery := it.client(t)
	recoveryToken := it.csrfToken(t, recovery)
	resp, body = it.login(t, recovery, recoveryToken, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("recovery login = %d, body = %s", resp.StatusCode, body)
	}
	var recoveryChallenge domain.LoginResponse
	if err := json.Unmarshal(body, &recoveryChallenge); err != nil {
		t.Fatalf("recovery challenge: %v (%s)", err, body)
	}
	resp, body = it.request(t, recovery, http.MethodPost, "/api/v1/auth/mfa/verify",
		domain.MFAVerifyRequest{MFAToken: *recoveryChallenge.MFAToken, RecoveryCode: codes[0]},
		map[string]string{"X-CSRF-Token": recoveryToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("recovery verify = %d, body = %s", resp.StatusCode, body)
	}
	if err := it.requestNoContent(t, recovery, http.MethodPost, "/api/v1/auth/logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	replay := it.client(t)
	replayToken := it.csrfToken(t, replay)
	resp, body = it.login(t, replay, replayToken, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replay login = %d, body = %s", resp.StatusCode, body)
	}
	var replayChallenge domain.LoginResponse
	if err := json.Unmarshal(body, &replayChallenge); err != nil {
		t.Fatalf("replay challenge: %v (%s)", err, body)
	}
	resp, body = it.request(t, replay, http.MethodPost, "/api/v1/auth/mfa/verify",
		domain.MFAVerifyRequest{MFAToken: *replayChallenge.MFAToken, RecoveryCode: codes[0]},
		map[string]string{"X-CSRF-Token": replayToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("recovery replay = %d, body = %s", resp.StatusCode, body)
	}

	failed := it.client(t)
	failedToken := it.csrfToken(t, failed)
	resp, body = it.login(t, failed, failedToken, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed login = %d, body = %s", resp.StatusCode, body)
	}
	var failedChallenge domain.LoginResponse
	if err := json.Unmarshal(body, &failedChallenge); err != nil {
		t.Fatalf("failed challenge: %v (%s)", err, body)
	}
	wrong, err := totp.Code(string(secret), totp.Step(time.Now().UTC())+10)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 5; attempt++ {
		resp, body = it.request(t, failed, http.MethodPost, "/api/v1/auth/mfa/verify",
			domain.MFAVerifyRequest{MFAToken: *failedChallenge.MFAToken, Code: wrong},
			map[string]string{"X-CSRF-Token": failedToken, "Origin": "http://localhost:5173"})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, body = %s", attempt, resp.StatusCode, body)
		}
	}
	resp, body = it.request(t, failed, http.MethodPost, "/api/v1/auth/mfa/verify",
		domain.MFAVerifyRequest{MFAToken: *failedChallenge.MFAToken, Code: code},
		map[string]string{"X-CSRF-Token": failedToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("challenge after five failures = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationSelfServiceMFA(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "self-mfa-" + uuid.NewString() + "@example.com"
	it.createUser(t, realm, email, "correct horse battery staple", true)
	client := it.client(t)
	token := it.csrfToken(t, client)
	resp, body := it.login(t, client, token, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, body = %s", resp.StatusCode, body)
	}
	var login domain.LoginResponse
	if err := json.Unmarshal(body, &login); err != nil {
		t.Fatalf("login body: %v (%s)", err, body)
	}
	if login.Status != domain.LoginStatusAuthenticated {
		t.Fatalf("status = %q", login.Status)
	}
	sessionToken := it.cookie(t, client, csrfCookieName)
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/me/mfa/totp",
		domain.MFAEnrollRequest{Action: "start"},
		map[string]string{"X-CSRF-Token": sessionToken})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("self enroll start = %d, body = %s", resp.StatusCode, body)
	}
	var started domain.MFAEnrollResponse
	if err := json.Unmarshal(body, &started); err != nil {
		t.Fatalf("self enroll body: %v (%s)", err, body)
	}
	code, err := totp.Code(*started.Secret, totp.Step(time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/me/mfa/totp",
		domain.MFAEnrollRequest{Action: "confirm", Code: code},
		map[string]string{"X-CSRF-Token": sessionToken})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("self enroll confirm = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/me/mfa/recovery-codes", nil,
		map[string]string{"X-CSRF-Token": sessionToken})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate codes = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, client, http.MethodDelete, "/api/v1/me/mfa/totp", nil,
		map[string]string{"X-CSRF-Token": sessionToken})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("disable totp = %d, body = %s", resp.StatusCode, body)
	}
	next := it.client(t)
	nextToken := it.csrfToken(t, next)
	resp, body = it.login(t, next, nextToken, email, "correct horse battery staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login after disable = %d, body = %s", resp.StatusCode, body)
	}
	var after domain.LoginResponse
	if err := json.Unmarshal(body, &after); err != nil {
		t.Fatalf("login after disable: %v (%s)", err, body)
	}
	if after.Status != domain.LoginStatusAuthenticated {
		t.Fatalf("status after disable = %q", after.Status)
	}
}

func TestIntegrationAdminUserCRUDAndIdempotency(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	adminEmail := "admin-crud-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, adminEmail, "correct horse battery staple")
	admin := it.client(t)
	it.loginAndEnrollAdmin(t, admin, adminEmail, "correct horse battery staple")
	adminToken := it.cookie(t, admin, csrfCookieName)

	targetEmail := "target-" + uuid.NewString() + "@example.com"
	create := domain.CreateUserRequest{Email: targetEmail, Name: "Target User"}
	key := uuid.NewString()
	headers := map[string]string{
		"X-CSRF-Token":    adminToken,
		"Idempotency-Key": key,
		"Origin":          "http://localhost:5173",
	}
	resp, body := it.request(t, admin, http.MethodPost, "/api/v1/users", create, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user = %d, body = %s", resp.StatusCode, body)
	}
	var created domain.AdminUserView
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("create body: %v (%s)", err, body)
	}
	if created.ID == "" || created.Email != targetEmail {
		t.Fatalf("created user = %+v", created)
	}
	resp, replay := it.request(t, admin, http.MethodPost, "/api/v1/users", create, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("replay = %d, body = %s", resp.StatusCode, replay)
	}
	var replayed domain.AdminUserView
	if err := json.Unmarshal(replay, &replayed); err != nil {
		t.Fatalf("replay body: %v (%s)", err, replay)
	}
	if replayed.ID != created.ID {
		t.Fatalf("replay id = %s, want %s", replayed.ID, created.ID)
	}
	changed := create
	changed.Name = "Different Name"
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/users", changed, headers)
	if resp.StatusCode != http.StatusConflict || problemCode(t, body) != domain.CodeIdempotencyMismatch {
		t.Fatalf("mismatch = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = it.request(t, admin, http.MethodGet,
		"/api/v1/users?q="+targetEmail+"&status=pending", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list = %d, body = %s", resp.StatusCode, body)
	}
	var list domain.AdminUserList
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("list body: %v (%s)", err, body)
	}
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("list items = %+v", list.Items)
	}
	if list.Items[0].SessionCount != 0 {
		t.Fatalf("session count = %d", list.Items[0].SessionCount)
	}

	resp, body = it.request(t, admin, http.MethodGet, "/api/v1/users/"+created.ID, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get user = %d, body = %s", resp.StatusCode, body)
	}
	name := "Renamed Target"
	resp, body = it.request(t, admin, http.MethodPatch, "/api/v1/users/"+created.ID,
		domain.UpdateUserRequest{Name: &name},
		map[string]string{"X-CSRF-Token": adminToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch user = %d, body = %s", resp.StatusCode, body)
	}
	var updated domain.AdminUserView
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("patch body: %v (%s)", err, body)
	}
	if updated.Name == nil || *updated.Name != name {
		t.Fatalf("updated name = %v", updated.Name)
	}

	member := it.ensureRole(t, realm, domain.MemberRole)
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/users/"+created.ID+"/roles",
		domain.AssignRolesRequest{RoleIDs: []string{string(member.ID)}},
		map[string]string{"X-CSRF-Token": adminToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("assign roles = %d, body = %s", resp.StatusCode, body)
	}
	var roles domain.RolesResponse
	if err := json.Unmarshal(body, &roles); err != nil {
		t.Fatalf("roles body: %v (%s)", err, body)
	}
	if len(roles.Roles) != 1 || roles.Roles[0].Name != domain.MemberRole {
		t.Fatalf("roles = %+v", roles.Roles)
	}

	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/users/"+created.ID+"/invite", nil,
		map[string]string{"X-CSRF-Token": adminToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("resend invite = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/users/"+created.ID+"/reset-password", nil,
		map[string]string{"X-CSRF-Token": adminToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("reset password = %d, body = %s", resp.StatusCode, body)
	}
	var invites int
	if err := it.db.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM outbox
		WHERE realm_id = $1::text::uuid AND type = 'email.invite'
			AND payload->>'to' = $2`, string(realm.ID), targetEmail).Scan(&invites); err != nil {
		t.Fatal(err)
	}
	if invites < 2 {
		t.Fatalf("invite outbox rows = %d, want at least 2", invites)
	}
	var resets int
	if err := it.db.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM outbox
		WHERE realm_id = $1::text::uuid AND type = 'email.reset'
			AND payload->>'to' = $2`, string(realm.ID), targetEmail).Scan(&resets); err != nil {
		t.Fatal(err)
	}
	if resets < 1 {
		t.Fatalf("reset outbox rows = %d, want at least 1", resets)
	}
}

func TestIntegrationLastAdminProtection(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	email := "last-admin-" + uuid.NewString() + "@example.com"
	adminUser := it.createAdmin(t, realm, email, "correct horse battery staple")
	client := it.client(t)
	it.loginAndEnrollAdmin(t, client, email, "correct horse battery staple")
	token := it.cookie(t, client, csrfCookieName)

	if _, err := it.db.Pool.Exec(context.Background(), `
		DELETE FROM user_role ur
		USING role r
		WHERE ur.role_id = r.id
			AND r.realm_id = $1::text::uuid
			AND lower(r.name) = lower($2)
			AND ur.user_id <> $3::text::uuid`,
		string(realm.ID), domain.PlatformAdminRole, string(adminUser.ID)); err != nil {
		t.Fatalf("remove competing admins: %v", err)
	}

	disabled := true
	resp, body := it.request(t, client, http.MethodPatch, "/api/v1/users/"+string(adminUser.ID),
		domain.UpdateUserRequest{Disabled: &disabled},
		map[string]string{"X-CSRF-Token": token, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("disable last admin = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, client, http.MethodPost, "/api/v1/users/"+string(adminUser.ID)+"/roles",
		domain.AssignRolesRequest{RoleIDs: []string{}},
		map[string]string{"X-CSRF-Token": token, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("remove last admin role = %d, body = %s", resp.StatusCode, body)
	}
	user, err := it.users.GetByID(context.Background(), adminUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	if user.DisabledAt != nil {
		t.Fatal("last admin was disabled")
	}
	roles, err := it.roles.ListByUser(context.Background(), realm.ID, adminUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].Name != domain.PlatformAdminRole {
		t.Fatalf("last admin roles = %+v", roles)
	}
}

func TestIntegrationDisableUserRevokesSessions(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	adminEmail := "admin-disable-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, adminEmail, "correct horse battery staple")
	admin := it.client(t)
	it.loginAndEnrollAdmin(t, admin, adminEmail, "correct horse battery staple")
	adminToken := it.cookie(t, admin, csrfCookieName)

	userEmail := "disable-" + uuid.NewString() + "@example.com"
	user := it.createUser(t, realm, userEmail, "correct horse battery staple", true)
	userClient := it.client(t)
	userToken := it.csrfToken(t, userClient)
	if resp, body := it.login(t, userClient, userToken, userEmail, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("user login = %d, body = %s", resp.StatusCode, body)
	}
	if resp, _ := it.request(t, userClient, http.MethodGet, "/api/v1/auth/session", nil, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("user session = %d", resp.StatusCode)
	}

	disabled := true
	resp, body := it.request(t, admin, http.MethodPatch, "/api/v1/users/"+string(user.ID),
		domain.UpdateUserRequest{Disabled: &disabled},
		map[string]string{"X-CSRF-Token": adminToken, "Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("disable user = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, userClient, http.MethodGet, "/api/v1/auth/session", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("disabled session = %d, body = %s", resp.StatusCode, body)
	}
	blocked := it.client(t)
	resp, body = it.login(t, blocked, it.csrfToken(t, blocked), userEmail, "correct horse battery staple")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("disabled login = %d, body = %s", resp.StatusCode, body)
	}
}

func (it *integration) requestNoContent(t *testing.T, client *http.Client, method, path string) error {
	t.Helper()
	resp, body := it.request(t, client, method, path, nil,
		map[string]string{"X-CSRF-Token": it.cookie(t, client, csrfCookieName),
			"Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusNoContent {
		return errors.New(string(body))
	}
	return nil
}

func mustUserID(t *testing.T, it *integration, realmID domain.RealmID, email string) domain.UserID {
	t.Helper()
	user, err := it.users.GetByEmail(context.Background(), realmID, email)
	if err != nil {
		t.Fatalf("read user %s: %v", email, err)
	}
	return user.ID
}
