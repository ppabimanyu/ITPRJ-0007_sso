package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/repository"
)

func (it *integration) adminSession(t *testing.T, email, password string) (*http.Client, map[string]string) {
	t.Helper()
	client := it.client(t)
	it.loginAndEnrollAdmin(t, client, email, password)
	headers := map[string]string{
		"X-CSRF-Token": it.cookie(t, client, csrfCookieName),
		"Origin":       "http://localhost:5173",
	}
	return client, headers
}

func TestIntegrationAdminClientCRUDAndSecretRotation(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	adminEmail := "admin-clients-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, adminEmail, "correct horse battery staple")
	admin, headers := it.adminSession(t, adminEmail, "correct horse battery staple")
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]

	create := domain.CreateClientRequest{
		Name:              "Test SPA " + suffix,
		Type:              domain.ClientTypePublic,
		RedirectURIs:      []string{"http://localhost:5173/callback-" + suffix},
		CORSOrigins:       []string{"http://localhost:5173"},
		GrantTypes:        []string{domain.GrantTypeAuthorizationCode, domain.GrantTypeRefreshToken},
		AllowedScopes:     []string{domain.ScopeOpenID, domain.ScopeProfile, domain.ScopeOfflineAccess},
		ThirdParty:        false,
		IDTokenTTLSeconds: intPtr(300),
	}
	key := uuid.NewString()
	headers["Idempotency-Key"] = key
	resp, body := it.request(t, admin, http.MethodPost, "/api/v1/clients", create, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create client = %d, body = %s", resp.StatusCode, body)
	}
	var created domain.ClientWithSecret
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("create body: %v (%s)", err, body)
	}
	if created.ID == "" || created.ClientID == "" || created.Name != create.Name {
		t.Fatalf("created client = %+v", created)
	}
	if created.ClientSecret != nil {
		t.Fatal("a public client was issued a secret")
	}
	if len(created.RedirectURIs) != 1 || created.RedirectURIs[0] != create.RedirectURIs[0] {
		t.Fatalf("redirect uris = %v", created.RedirectURIs)
	}

	resp, replay := it.request(t, admin, http.MethodPost, "/api/v1/clients", create, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("replay = %d, body = %s", resp.StatusCode, replay)
	}
	var replayed domain.ClientWithSecret
	if err := json.Unmarshal(replay, &replayed); err != nil {
		t.Fatalf("replay body: %v (%s)", err, replay)
	}
	if replayed.ID != created.ID {
		t.Fatalf("replay id = %s, want %s", replayed.ID, created.ID)
	}
	create.Name = "Different name"
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/clients", create, headers)
	if resp.StatusCode != http.StatusConflict || problemCode(t, body) != domain.CodeIdempotencyMismatch {
		t.Fatalf("idempotency mismatch = %d, body = %s", resp.StatusCode, body)
	}
	create.Name = "Test SPA " + suffix

	resp, body = it.request(t, admin, http.MethodGet, "/api/v1/clients?limit=100", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list clients = %d, body = %s", resp.StatusCode, body)
	}
	var list domain.AdminClientList
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("list body: %v (%s)", err, body)
	}
	found := false
	for _, item := range list.Items {
		if item.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created client %s missing from list", created.ID)
	}

	name := "Renamed SPA " + suffix
	updatedRedirects := []string{"http://localhost:5173/callback2-" + suffix}
	disabled := true
	resp, body = it.request(t, admin, http.MethodPatch, "/api/v1/clients/"+created.ID,
		domain.UpdateClientRequest{
			Name:         &name,
			RedirectURIs: &updatedRedirects,
			Disabled:     &disabled,
		}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch client = %d, body = %s", resp.StatusCode, body)
	}
	var patched domain.AdminClientView
	if err := json.Unmarshal(body, &patched); err != nil {
		t.Fatalf("patch body: %v (%s)", err, body)
	}
	if patched.Name != name || !patched.Disabled || len(patched.RedirectURIs) != 1 ||
		patched.RedirectURIs[0] != updatedRedirects[0] {
		t.Fatalf("patched client = %+v", patched)
	}

	confidential := domain.CreateClientRequest{
		Name:          "Test Backend " + suffix,
		Type:          domain.ClientTypeConfidential,
		RedirectURIs:  []string{"http://localhost:8080/callback-" + suffix},
		GrantTypes:    []string{domain.GrantTypeClientCredentials},
		AllowedScopes: []string{domain.ScopeOpenID},
	}
	delete(headers, "Idempotency-Key")
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/clients", confidential, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create confidential = %d, body = %s", resp.StatusCode, body)
	}
	var confidentialClient domain.ClientWithSecret
	if err := json.Unmarshal(body, &confidentialClient); err != nil {
		t.Fatalf("confidential body: %v (%s)", err, body)
	}
	if confidentialClient.ClientSecret == nil || *confidentialClient.ClientSecret == "" {
		t.Fatal("confidential client secret was not returned once")
	}
	initialSecret := *confidentialClient.ClientSecret

	resp, body = it.request(t, admin, http.MethodGet,
		"/api/v1/clients/"+confidentialClient.ID, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get confidential = %d, body = %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), initialSecret) || strings.Contains(string(body), "client_secret") {
		t.Fatalf("GET leaked the client secret: %s", body)
	}

	resp, body = it.request(t, admin, http.MethodPost,
		"/api/v1/clients/"+confidentialClient.ID+"/rotate-secret", nil, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate secret = %d, body = %s", resp.StatusCode, body)
	}
	var rotated domain.ClientSecretResult
	if err := json.Unmarshal(body, &rotated); err != nil {
		t.Fatalf("rotate body: %v (%s)", err, body)
	}
	if rotated.ClientSecret == "" || rotated.ClientSecret == initialSecret {
		t.Fatal("rotate did not return a new secret")
	}
	if rotated.ClientID != confidentialClient.ClientID {
		t.Fatalf("rotated client_id = %q, want %q", rotated.ClientID, confidentialClient.ClientID)
	}
	resp, body = it.request(t, admin, http.MethodGet,
		"/api/v1/clients/"+confidentialClient.ID, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get rotated = %d, body = %s", resp.StatusCode, body)
	}
	var afterRotate domain.AdminClientView
	if err := json.Unmarshal(body, &afterRotate); err != nil {
		t.Fatalf("rotated view: %v (%s)", err, body)
	}
	if afterRotate.SecretRotatedAt == nil {
		t.Fatal("secret_rotated_at is empty after rotation")
	}

	user := it.createUser(t, realm, "consent-"+uuid.NewString()+"@example.com", "", true)
	if _, err := it.db.Pool.Exec(context.Background(), `
		INSERT INTO consent (id, user_id, client_id, scope)
		VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, 'openid')`,
		uuid.NewString(), string(user.ID), confidentialClient.ID); err != nil {
		t.Fatalf("insert consent: %v", err)
	}
	resp, body = it.request(t, admin, http.MethodDelete,
		"/api/v1/clients/"+confidentialClient.ID, nil, headers)
	if resp.StatusCode != http.StatusConflict || problemCode(t, body) != domain.CodeConflict {
		t.Fatalf("delete guarded = %d, body = %s", resp.StatusCode, body)
	}
	if _, err := it.db.Pool.Exec(context.Background(),
		`DELETE FROM consent WHERE client_id = $1::text::uuid`, confidentialClient.ID); err != nil {
		t.Fatalf("delete consent: %v", err)
	}
	resp, body = it.request(t, admin, http.MethodDelete,
		"/api/v1/clients/"+confidentialClient.ID, nil, headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete client = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, admin, http.MethodGet,
		"/api/v1/clients/"+confidentialClient.ID, nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted client get = %d, body = %s", resp.StatusCode, body)
	}

	bad := domain.CreateClientRequest{
		Name:         "Bad " + suffix,
		Type:         domain.ClientTypePublic,
		RedirectURIs: []string{"http://evil.example/callback"},
	}
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/clients", bad, headers)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("non-local http redirect = %d, body = %s", resp.StatusCode, body)
	}
	bad.RedirectURIs = []string{"https://*.example.com/callback"}
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/clients", bad, headers)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("wildcard redirect = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationAdminRolesGroupsAndMembers(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	adminEmail := "admin-groups-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, adminEmail, "correct horse battery staple")
	admin, headers := it.adminSession(t, adminEmail, "correct horse battery staple")
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]

	roleName := "role-" + suffix
	roleRequest := domain.CreateRoleRequest{Name: roleName}
	key := uuid.NewString()
	headers["Idempotency-Key"] = key
	resp, body := it.request(t, admin, http.MethodPost, "/api/v1/roles", roleRequest, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create role = %d, body = %s", resp.StatusCode, body)
	}
	var role domain.RoleView
	if err := json.Unmarshal(body, &role); err != nil {
		t.Fatalf("role body: %v (%s)", err, body)
	}
	if role.Name != roleName || role.IsSystem {
		t.Fatalf("role = %+v", role)
	}
	resp, replay := it.request(t, admin, http.MethodPost, "/api/v1/roles", roleRequest, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("role replay = %d, body = %s", resp.StatusCode, replay)
	}
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/roles", roleRequest, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("role repeated create replay = %d, body = %s", resp.StatusCode, body)
	}
	delete(headers, "Idempotency-Key")
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/roles", roleRequest, headers)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate role = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, admin, http.MethodGet, "/api/v1/roles", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list roles = %d, body = %s", resp.StatusCode, body)
	}
	var roles domain.RoleList
	if err := json.Unmarshal(body, &roles); err != nil {
		t.Fatalf("roles body: %v (%s)", err, body)
	}
	foundRole := false
	for _, item := range roles.Items {
		if item.ID == role.ID {
			foundRole = true
		}
	}
	if !foundRole {
		t.Fatalf("role %s missing from list", role.ID)
	}

	groupName := "group-" + suffix
	groupRequest := domain.CreateGroupRequest{Name: groupName, Description: stringPtr("Support group")}
	headers["Idempotency-Key"] = uuid.NewString()
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/groups", groupRequest, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group = %d, body = %s", resp.StatusCode, body)
	}
	var group domain.GroupView
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatalf("group body: %v (%s)", err, body)
	}
	if group.Name != groupName || group.MemberCount != 0 {
		t.Fatalf("group = %+v", group)
	}
	delete(headers, "Idempotency-Key")
	renamed := "group-renamed-" + suffix
	clearedDescription := ""
	resp, body = it.request(t, admin, http.MethodPatch, "/api/v1/groups/"+group.ID,
		domain.UpdateGroupRequest{
			Name:        &renamed,
			Description: domain.Optional[string]{Set: true, Value: &clearedDescription},
		}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch group = %d, body = %s", resp.StatusCode, body)
	}
	var patched domain.GroupView
	if err := json.Unmarshal(body, &patched); err != nil {
		t.Fatalf("patched group body: %v (%s)", err, body)
	}
	if patched.Name != renamed || patched.Description != nil {
		t.Fatalf("patched group = %+v", patched)
	}

	first := it.createUser(t, realm, "member1-"+uuid.NewString()+"@example.com", "", true)
	second := it.createUser(t, realm, "member2-"+uuid.NewString()+"@example.com", "", true)
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/groups/"+group.ID+"/members",
		domain.AddGroupMembersRequest{UserIDs: []string{string(first.ID), string(second.ID)}}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add members = %d, body = %s", resp.StatusCode, body)
	}
	var members domain.GroupMemberList
	if err := json.Unmarshal(body, &members); err != nil {
		t.Fatalf("members body: %v (%s)", err, body)
	}
	if len(members.Items) != 2 {
		t.Fatalf("member count = %d, want 2", len(members.Items))
	}
	emails := map[string]bool{}
	for _, member := range members.Items {
		emails[member.Email] = true
	}
	if !emails[first.Email] || !emails[second.Email] {
		t.Fatalf("members = %+v", members.Items)
	}
	resp, body = it.request(t, admin, http.MethodDelete,
		"/api/v1/groups/"+group.ID+"/members/"+string(first.ID), nil, headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("remove member = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, admin, http.MethodGet, "/api/v1/groups/"+group.ID+"/members", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list members = %d, body = %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &members); err != nil {
		t.Fatalf("members after remove: %v (%s)", err, body)
	}
	if len(members.Items) != 1 || members.Items[0].Email != second.Email {
		t.Fatalf("members after remove = %+v", members.Items)
	}
	resp, body = it.request(t, admin, http.MethodDelete, "/api/v1/groups/"+group.ID, nil, headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete group = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, admin, http.MethodGet, "/api/v1/groups/"+group.ID, nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted group get = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationAdminSessionsAuditAndRevoke(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	adminEmail := "admin-sessions-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, adminEmail, "correct horse battery staple")
	admin, headers := it.adminSession(t, adminEmail, "correct horse battery staple")

	targetEmail := "session-target-" + uuid.NewString() + "@example.com"
	target := it.createUser(t, realm, targetEmail, "correct horse battery staple", true)
	first := it.client(t)
	if resp, body := it.login(t, first, it.csrfToken(t, first), targetEmail, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("first target login = %d, body = %s", resp.StatusCode, body)
	}
	second := it.client(t)
	if resp, body := it.login(t, second, it.csrfToken(t, second), targetEmail, "correct horse battery staple"); resp.StatusCode != http.StatusOK {
		t.Fatalf("second target login = %d, body = %s", resp.StatusCode, body)
	}

	resp, body := it.request(t, admin, http.MethodGet,
		"/api/v1/sessions?user_id="+string(target.ID)+"&status=active&limit=100", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list sessions = %d, body = %s", resp.StatusCode, body)
	}
	var sessions domain.AdminSessionList
	if err := json.Unmarshal(body, &sessions); err != nil {
		t.Fatalf("sessions body: %v (%s)", err, body)
	}
	if len(sessions.Items) != 2 {
		t.Fatalf("session count = %d, want 2", len(sessions.Items))
	}
	if sessions.Items[0].UserEmail != targetEmail {
		t.Fatalf("session email = %q, want %q", sessions.Items[0].UserEmail, targetEmail)
	}
	revokeID := sessions.Items[0].ID

	resp, body = it.request(t, admin, http.MethodDelete, "/api/v1/sessions/"+revokeID, nil, headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke session = %d, body = %s", resp.StatusCode, body)
	}
	resp, _ = it.request(t, first, http.MethodGet, "/api/v1/auth/session", nil, nil)
	revokedFirst := resp.StatusCode == http.StatusUnauthorized
	resp, _ = it.request(t, second, http.MethodGet, "/api/v1/auth/session", nil, nil)
	revokedSecond := resp.StatusCode == http.StatusUnauthorized
	if revokedFirst == revokedSecond {
		t.Fatalf("exactly one target session should be revoked, first=%v second=%v", revokedFirst, revokedSecond)
	}
	resp, body = it.request(t, admin, http.MethodGet,
		"/api/v1/sessions?user_id="+string(target.ID)+"&status=revoked&limit=100", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list revoked sessions = %d, body = %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &sessions); err != nil {
		t.Fatalf("revoked sessions body: %v (%s)", err, body)
	}
	if len(sessions.Items) != 1 || sessions.Items[0].ID != revokeID {
		t.Fatalf("revoked sessions = %+v", sessions.Items)
	}

	roleName := "audit-role-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	resp, body = it.request(t, admin, http.MethodPost, "/api/v1/roles",
		domain.CreateRoleRequest{Name: roleName}, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create audit role = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, admin, http.MethodGet,
		"/api/v1/audit?type="+domain.AuditAdminRoleCreated, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit type filter = %d, body = %s", resp.StatusCode, body)
	}
	var events domain.AuditList
	if err := json.Unmarshal(body, &events); err != nil {
		t.Fatalf("audit body: %v (%s)", err, body)
	}
	foundEvent := false
	for _, event := range events.Items {
		if event.Type != domain.AuditAdminRoleCreated {
			t.Fatalf("audit type = %q", event.Type)
		}
		if !strings.Contains(string(event.Metadata), roleName) {
			continue
		}
		if strings.Contains(string(event.Metadata), adminEmail) ||
			strings.Contains(string(event.Metadata), targetEmail) ||
			strings.Contains(string(event.Metadata), "password") {
			t.Fatalf("audit metadata leaked a secret or email: %s", event.Metadata)
		}
		if event.ActorEmail == nil || *event.ActorEmail != adminEmail {
			t.Fatalf("audit actor email = %v, want %q", event.ActorEmail, adminEmail)
		}
		foundEvent = true
	}
	if !foundEvent {
		t.Fatal("role audit event missing")
	}
	now := time.Now().UTC()
	resp, body = it.request(t, admin, http.MethodGet,
		"/api/v1/audit?from="+now.Add(-time.Hour).Format(time.RFC3339)+
			"&to="+now.Add(time.Hour).Format(time.RFC3339), nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit time filter = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = it.request(t, admin, http.MethodGet, "/api/v1/audit?from=not-a-time", nil, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid from = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationRealmSettingsSMTPPasswordIsWriteOnly(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	t.Cleanup(func() {
		_, _ = it.db.Pool.Exec(context.Background(),
			`UPDATE realm SET settings = '{}'::jsonb WHERE id = $1::text::uuid`, string(realm.ID))
	})
	adminEmail := "admin-settings-" + uuid.NewString() + "@example.com"
	it.createAdmin(t, realm, adminEmail, "correct horse battery staple")
	admin, headers := it.adminSession(t, adminEmail, "correct horse battery staple")

	resp, body := it.request(t, admin, http.MethodGet, "/api/v1/realm/settings", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get settings = %d, body = %s", resp.StatusCode, body)
	}
	var current domain.RealmSettings
	if err := json.Unmarshal(body, &current); err != nil {
		t.Fatalf("settings body: %v (%s)", err, body)
	}
	if current.Realm != realm.Name || current.PasswordPolicy.MinLength < 12 {
		t.Fatalf("default settings = %+v", current)
	}
	if current.SMTP.PasswordSet {
		t.Fatal("default settings report a password")
	}

	smtpPassword := "realm-smtp-password"
	username := "mailer"
	minLength := 14
	idle := 20
	absolute := 8
	adminIdle := 10
	remember := 14
	useTLS := false
	resp, body = it.request(t, admin, http.MethodPatch, "/api/v1/realm/settings",
		domain.UpdateRealmSettingsRequest{
			PasswordPolicy: &domain.UpdatePasswordPolicy{MinLength: &minLength},
			SessionPolicy: &domain.UpdateSessionPolicy{
				IdleMinutes:      &idle,
				AbsoluteHours:    &absolute,
				AdminIdleMinutes: &adminIdle,
				RememberMeDays:   &remember,
			},
			SMTP: &domain.UpdateSMTPSettings{
				Host:        stringPtr("localhost"),
				Port:        intPtr(1025),
				Username:    domain.Optional[string]{Set: true, Value: &username},
				FromAddress: stringPtr("no-reply@example.com"),
				UseTLS:      &useTLS,
				Password:    &smtpPassword,
			},
		}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch settings = %d, body = %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), smtpPassword) || strings.Contains(string(body), `"password"`) {
		t.Fatalf("settings response leaked the SMTP password: %s", body)
	}
	var updated domain.RealmSettings
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("updated settings body: %v (%s)", err, body)
	}
	if !updated.SMTP.PasswordSet || updated.SMTP.Host != "localhost" || updated.SMTP.Port != 1025 ||
		updated.SessionPolicy.IdleMinutes != 20 || updated.PasswordPolicy.MinLength != 14 {
		t.Fatalf("updated settings = %+v", updated)
	}

	var rawSettings string
	if err := it.db.Pool.QueryRow(context.Background(),
		`SELECT settings::text FROM realm WHERE id = $1::text::uuid`, string(realm.ID)).Scan(&rawSettings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rawSettings, "password_encrypted") || strings.Contains(rawSettings, smtpPassword) {
		t.Fatalf("stored settings = %s", rawSettings)
	}
	resp, body = it.request(t, admin, http.MethodGet, "/api/v1/realm/settings", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get updated settings = %d, body = %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), smtpPassword) || !strings.Contains(string(body), `"password_set":true`) {
		t.Fatalf("GET settings did not hide the password: %s", body)
	}

	emptyPassword := ""
	resp, body = it.request(t, admin, http.MethodPatch, "/api/v1/realm/settings",
		domain.UpdateRealmSettingsRequest{
			SMTP: &domain.UpdateSMTPSettings{Password: &emptyPassword},
		}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear SMTP password = %d, body = %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("cleared settings body: %v (%s)", err, body)
	}
	if updated.SMTP.PasswordSet {
		t.Fatal("password_set is still true after clearing the password")
	}
	if err := it.db.Pool.QueryRow(context.Background(),
		`SELECT settings::text FROM realm WHERE id = $1::text::uuid`, string(realm.ID)).Scan(&rawSettings); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rawSettings, "password_encrypted") {
		t.Fatalf("cleared settings still store a password: %s", rawSettings)
	}

	tooShort := 11
	resp, body = it.request(t, admin, http.MethodPatch, "/api/v1/realm/settings",
		domain.UpdateRealmSettingsRequest{
			PasswordPolicy: &domain.UpdatePasswordPolicy{MinLength: &tooShort},
		}, headers)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("short password policy = %d, body = %s", resp.StatusCode, body)
	}
}

func TestIntegrationAuditRetentionDeletesOldEvents(t *testing.T) {
	it := newIntegration(t)
	realm := it.createRealm(t)
	ctx := context.Background()
	audits := repository.NewAuditRepository(it.db.Pool)

	oldID := uuid.NewString()
	recentID := uuid.NewString()
	now := time.Now().UTC()
	if err := audits.Insert(ctx, &domain.AuditEvent{
		ID:        domain.AuditEventID(oldID),
		RealmID:   realm.ID,
		ActorType: domain.ActorTypeSystem,
		Type:      "test.retention.old",
		Metadata:  []byte("{}"),
		CreatedAt: now.Add(-91 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("insert old event: %v", err)
	}
	if err := audits.Insert(ctx, &domain.AuditEvent{
		ID:        domain.AuditEventID(recentID),
		RealmID:   realm.ID,
		ActorType: domain.ActorTypeSystem,
		Type:      "test.retention.recent",
		Metadata:  []byte("{}"),
		CreatedAt: now.Add(-89 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("insert recent event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = it.db.Pool.Exec(context.Background(),
			`DELETE FROM audit_event WHERE id = $1::text::uuid`, recentID)
	})

	if _, err := audits.DeleteBefore(ctx, now.Add(-90*24*time.Hour)); err != nil {
		t.Fatalf("delete before cutoff: %v", err)
	}

	var oldCount, recentCount int
	if err := it.db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_event WHERE id = $1::text::uuid`, oldID).Scan(&oldCount); err != nil {
		t.Fatalf("count old event: %v", err)
	}
	if oldCount != 0 {
		t.Fatalf("old event survived retention: %d", oldCount)
	}
	if err := it.db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_event WHERE id = $1::text::uuid`, recentID).Scan(&recentCount); err != nil {
		t.Fatalf("count recent event: %v", err)
	}
	if recentCount != 1 {
		t.Fatalf("recent event count = %d, want 1", recentCount)
	}
}

func intPtr(value int) *int { return &value }
