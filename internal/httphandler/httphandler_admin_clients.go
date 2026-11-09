package httphandler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

const (
	adminClientEndpoint = "POST /api/v1/clients"
	defaultClientGrants = "authorization_code"
)

func (h *handlers) listClients(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	limit, ok := queryLimit(w, r)
	if !ok {
		return
	}
	if h.deps.AdminClients == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	views, next, err := h.deps.AdminClients.ListAdmin(r.Context(), current.User.RealmID,
		domain.AdminClientFilter{After: strings.TrimSpace(r.URL.Query().Get("after")), Limit: limit})
	if err != nil {
		h.logInternal(r.Context(), "admin_client_list_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	var nextAfter *string
	if next != "" {
		nextAfter = &next
	}
	writeJSON(w, http.StatusOK, domain.AdminClientList{Items: views, NextAfter: nextAfter})
}

func (h *handlers) getClient(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	clientID, ok := adminClientPathID(w, r)
	if !ok {
		return
	}
	if h.deps.AdminClients == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	view, err := h.deps.AdminClients.GetAdmin(r.Context(), current.User.RealmID, clientID)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "client not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_client_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *handlers) createClient(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	var request domain.CreateClientRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	name, fields := validClientName(request.Name)
	if !validClientType(request.Type) {
		fields = append(fields, FieldError{Field: "type", Message: "must be public or confidential"})
	}
	redirects, redirectFields := validRedirectURIs(request.RedirectURIs, h.deps.Config.IsDev(), true)
	fields = append(fields, redirectFields...)
	postLogout, postFields := validRedirectURIs(request.PostLogoutRedirectURIs, h.deps.Config.IsDev(), false)
	fields = append(fields, postFields...)
	origins, originFields := validCORSOrigins(request.CORSOrigins, h.deps.Config.IsDev())
	fields = append(fields, originFields...)
	backchannel, backchannelErr := validBackchannelURI(request.BackchannelLogoutURI, h.deps.Config.IsDev())
	if backchannelErr != "" {
		fields = append(fields, FieldError{Field: "backchannel_logout_uri", Message: backchannelErr})
	}
	grants, grantFields := validGrantTypes(request.GrantTypes, true)
	fields = append(fields, grantFields...)
	scopes, scopeFields := validClientScopes(request.AllowedScopes, true)
	fields = append(fields, scopeFields...)
	if err := validTTL("access_token_ttl_seconds", request.AccessTokenTTLSeconds); err != nil {
		fields = append(fields, *err)
	}
	if err := validTTL("id_token_ttl_seconds", request.IDTokenTTLSeconds); err != nil {
		fields = append(fields, *err)
	}
	if len(fields) > 0 {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"client does not meet the constraints", fields)
		return
	}
	if grants == nil {
		grants = []string{defaultClientGrants}
	}
	if scopes == nil {
		scopes = []string{domain.ScopeOpenID}
	}

	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	requestHash, proceed := h.checkIdempotency(w, r, current.User.RealmID,
		adminClientEndpoint, key, request)
	if !proceed {
		return
	}

	publicID, err := newClientIdentifier(name)
	if err != nil {
		h.logInternal(r.Context(), "client_id_generate_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	client := &domain.Client{
		ID:                    domain.ClientID(uuid.NewString()),
		RealmID:               current.User.RealmID,
		ClientID:              publicID,
		Name:                  name,
		Type:                  request.Type,
		GrantTypes:            grants,
		AllowedScopes:         scopes,
		ThirdParty:            request.ThirdParty,
		AccessTokenTTLSeconds: request.AccessTokenTTLSeconds,
		IDTokenTTLSeconds:     request.IDTokenTTLSeconds,
		BackchannelLogoutURI:  backchannel,
	}
	var secret *domain.ClientSecret
	var rawSecret *string
	if request.Type == domain.ClientTypeConfidential {
		generated, err := pkg.RandomToken(32)
		if err != nil {
			h.logInternal(r.Context(), "client_secret_generate_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		rawSecret = &generated
		secret = &domain.ClientSecret{ClientID: client.ID, SecretHash: pkg.SHA256Hex(generated)}
	}
	redirectModels := make([]domain.ClientRedirectURI, 0, len(redirects)+len(postLogout))
	for _, uri := range redirects {
		redirectModels = append(redirectModels, domain.ClientRedirectURI{
			ClientID: client.ID, URI: uri, Kind: domain.RedirectKindAuthorize,
		})
	}
	for _, uri := range postLogout {
		redirectModels = append(redirectModels, domain.ClientRedirectURI{
			ClientID: client.ID, URI: uri, Kind: domain.RedirectKindPostLogout,
		})
	}
	originModels := make([]domain.ClientCORSOrigin, 0, len(origins))
	for _, origin := range origins {
		originModels = append(originModels, domain.ClientCORSOrigin{ClientID: client.ID, Origin: origin})
	}
	audit := adminAudit(current, domain.AuditAdminClientCreated, map[string]any{
		"client_id": client.ClientID,
		"type":      client.Type,
	})
	if err := h.deps.AdminClients.CreateWithAudit(r.Context(), client, secret,
		redirectModels, originModels, audit); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"a client with this identifier already exists")
			return
		}
		h.logInternal(r.Context(), "admin_client_create_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	view, err := h.deps.AdminClients.GetAdmin(r.Context(), client.RealmID, client.ID)
	if err != nil {
		h.logInternal(r.Context(), "admin_client_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	body, err := json.Marshal(domain.ClientWithSecret{AdminClientView: *view, ClientSecret: rawSecret})
	if err != nil {
		h.logInternal(r.Context(), "admin_client_encode_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	h.saveIdempotency(r.Context(), current.User.RealmID, adminClientEndpoint, key,
		requestHash, http.StatusCreated, body)
	writeRawJSON(w, http.StatusCreated, body)
}

func (h *handlers) updateClient(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	clientID, ok := adminClientPathID(w, r)
	if !ok {
		return
	}
	var request domain.UpdateClientRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	var fields []FieldError
	update := domain.AdminClientUpdate{
		RedirectURIs:           request.RedirectURIs,
		PostLogoutRedirectURIs: request.PostLogoutRedirectURIs,
		CORSOrigins:            request.CORSOrigins,
		GrantTypes:             request.GrantTypes,
		AllowedScopes:          request.AllowedScopes,
		ThirdParty:             request.ThirdParty,
		AccessTokenTTLSeconds:  request.AccessTokenTTLSeconds,
		IDTokenTTLSeconds:      request.IDTokenTTLSeconds,
		Disabled:               request.Disabled,
		BackchannelLogoutURI:   request.BackchannelLogoutURI,
	}
	if request.Name != nil {
		name, nameFields := validClientName(*request.Name)
		fields = append(fields, nameFields...)
		update.Name = &name
	}
	if request.RedirectURIs != nil {
		uriModels, uriFields := validRedirectURIs(*request.RedirectURIs, h.deps.Config.IsDev(), false)
		fields = append(fields, uriFields...)
		update.RedirectURIs = &uriModels
	}
	if request.PostLogoutRedirectURIs != nil {
		uriModels, uriFields := validRedirectURIs(*request.PostLogoutRedirectURIs, h.deps.Config.IsDev(), false)
		fields = append(fields, uriFields...)
		update.PostLogoutRedirectURIs = &uriModels
	}
	if request.CORSOrigins != nil {
		origins, originFields := validCORSOrigins(*request.CORSOrigins, h.deps.Config.IsDev())
		fields = append(fields, originFields...)
		update.CORSOrigins = &origins
	}
	if request.GrantTypes != nil {
		grants, grantFields := validGrantTypes(*request.GrantTypes, false)
		fields = append(fields, grantFields...)
		update.GrantTypes = &grants
	}
	if request.AllowedScopes != nil {
		scopes, scopeFields := validClientScopes(*request.AllowedScopes, false)
		fields = append(fields, scopeFields...)
		update.AllowedScopes = &scopes
	}
	if request.BackchannelLogoutURI.Set && request.BackchannelLogoutURI.Value != nil {
		value, message := validBackchannelURI(request.BackchannelLogoutURI.Value, h.deps.Config.IsDev())
		if message != "" {
			fields = append(fields, FieldError{Field: "backchannel_logout_uri", Message: message})
		}
		update.BackchannelLogoutURI = domain.Optional[string]{Set: true, Value: value}
	}
	if request.AccessTokenTTLSeconds.Set {
		if err := validOptionalTTL("access_token_ttl_seconds", request.AccessTokenTTLSeconds); err != nil {
			fields = append(fields, *err)
		}
	}
	if request.IDTokenTTLSeconds.Set {
		if err := validOptionalTTL("id_token_ttl_seconds", request.IDTokenTTLSeconds); err != nil {
			fields = append(fields, *err)
		}
	}
	if len(fields) > 0 {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"client does not meet the constraints", fields)
		return
	}
	if h.deps.AdminClients == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	eventType := domain.AuditAdminClientUpdated
	if request.Disabled != nil && *request.Disabled {
		eventType = domain.AuditAdminClientDisabled
	}
	audit := adminAudit(current, eventType, map[string]any{"client_key": string(clientID)})
	view, err := h.deps.AdminClients.UpdateAdmin(r.Context(), current.User.RealmID, clientID, update, audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "client not found")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_client_update_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *handlers) rotateClientSecret(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	clientID, ok := adminClientPathID(w, r)
	if !ok {
		return
	}
	if h.deps.AdminClients == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	secret, err := pkg.RandomToken(32)
	if err != nil {
		h.logInternal(r.Context(), "client_secret_generate_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	audit := adminAudit(current, domain.AuditAdminClientSecretRotated,
		map[string]any{"client_key": string(clientID)})
	rotatedAt, err := h.deps.AdminClients.RotateSecret(r.Context(), current.User.RealmID,
		clientID, pkg.SHA256Hex(secret), audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "client not found")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"a public client does not have a secret to rotate")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_client_rotate_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	publicClientID := string(clientID)
	if view, lookupErr := h.deps.AdminClients.GetAdmin(r.Context(), current.User.RealmID, clientID); lookupErr == nil {
		publicClientID = view.ClientID
	}
	writeJSON(w, http.StatusOK, domain.ClientSecretResult{
		ClientID:     publicClientID,
		ClientSecret: secret,
		RotatedAt:    rotatedAt,
	})
}

func (h *handlers) deleteClient(w http.ResponseWriter, r *http.Request) {
	current := SessionFromContext(r.Context())
	if current == nil {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	clientID, ok := adminClientPathID(w, r)
	if !ok {
		return
	}
	if h.deps.AdminClients == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"admin storage is unavailable")
		return
	}
	audit := adminAudit(current, domain.AuditAdminClientDeleted,
		map[string]any{"client_key": string(clientID)})
	err := h.deps.AdminClients.Delete(r.Context(), current.User.RealmID, clientID, audit)
	if errors.Is(err, domain.ErrNotFound) {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "client not found")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"client has an active session or consent; disable it instead")
		return
	}
	if err != nil {
		h.logInternal(r.Context(), "admin_client_delete_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	writeNoContent(w)
}

func adminClientPathID(w http.ResponseWriter, r *http.Request) (domain.ClientID, bool) {
	raw := chi.URLParam(r, "id")
	if _, err := uuid.Parse(raw); err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "client not found")
		return "", false
	}
	return domain.ClientID(raw), true
}

func queryLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			WriteProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
				"limit must be between 1 and 100")
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}

func validClientName(raw string) (string, []FieldError) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > 200 {
		return name, []FieldError{{Field: "name", Message: "is required and must be at most 200 characters"}}
	}
	return name, nil
}

func validClientType(value string) bool {
	return value == domain.ClientTypePublic || value == domain.ClientTypeConfidential
}

func validRedirectURIs(values []string, dev, required bool) ([]string, []FieldError) {
	if required && len(values) == 0 {
		return nil, []FieldError{{Field: "redirect_uris", Message: "must contain at least one URI"}}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	var fields []FieldError
	for _, raw := range values {
		if err := validateRedirectURI(raw, dev); err != nil {
			fields = append(fields, FieldError{Field: "redirect_uris", Message: err.Error()})
			continue
		}
		if _, exists := seen[raw]; exists {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out, fields
}

func validateRedirectURI(raw string, dev bool) error {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.Contains(raw, "*") {
		return errors.New("must be an exact URI without wildcards or surrounding spaces")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil {
		return errors.New("must be an absolute URI with a host and no userinfo")
	}
	if parsed.Fragment != "" {
		return errors.New("must not contain a fragment")
	}
	if parsed.Scheme != "https" && !loopbackHTTPAllowed(parsed.Scheme, parsed.Hostname(), dev) {
		return errors.New("must use https unless it targets localhost in dev")
	}
	return nil
}

func validCORSOrigins(values []string, dev bool) ([]string, []FieldError) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	var fields []FieldError
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || trimmed != raw || strings.Contains(raw, "*") {
			fields = append(fields, FieldError{Field: "cors_origins", Message: "must be an exact origin without wildcards"})
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" || parsed.User != nil ||
			(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			fields = append(fields, FieldError{Field: "cors_origins", Message: "must be a scheme and host with no path"})
			continue
		}
		if parsed.Scheme != "https" && !loopbackHTTPAllowed(parsed.Scheme, parsed.Hostname(), dev) {
			fields = append(fields, FieldError{Field: "cors_origins", Message: "must use https unless it targets localhost in dev"})
			continue
		}
		if _, exists := seen[raw]; exists {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out, fields
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loopbackHTTPAllowed(scheme, host string, dev bool) bool {
	return dev && scheme == "http" && isLoopbackHost(host)
}

func validBackchannelURI(raw *string, dev bool) (*string, string) {
	if raw == nil {
		return nil, ""
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, ""
	}
	if value != *raw || strings.Contains(value, "*") {
		return nil, "must be an exact URI without wildcards or surrounding spaces"
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil {
		return nil, "must be an absolute URI with a host and no userinfo"
	}
	if parsed.Fragment != "" {
		return nil, "must not contain a fragment"
	}
	host := parsed.Hostname()
	if parsed.Scheme != "https" && !loopbackHTTPAllowed(parsed.Scheme, host, dev) {
		return nil, "must use https unless it targets localhost in dev"
	}
	if !dev {
		if strings.EqualFold(host, "localhost") {
			return nil, "must not target loopback in non-dev"
		}
		if ip := net.ParseIP(host); ip != nil &&
			(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
			return nil, "must not target a private or loopback address in non-dev"
		}
	}
	return &value, ""
}

func validGrantTypes(values []string, defaultWhenEmpty bool) ([]string, []FieldError) {
	if len(values) == 0 {
		if defaultWhenEmpty {
			return []string{defaultClientGrants}, nil
		}
		return nil, []FieldError{{Field: "grant_types", Message: "must contain at least one grant type"}}
	}
	allowed := map[string]struct{}{
		domain.GrantTypeAuthorizationCode: {},
		domain.GrantTypeRefreshToken:      {},
		domain.GrantTypeClientCredentials: {},
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	var fields []FieldError
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			fields = append(fields, FieldError{Field: "grant_types", Message: "contains an unsupported grant type"})
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, fields
}

func validClientScopes(values []string, defaultWhenEmpty bool) ([]string, []FieldError) {
	if len(values) == 0 {
		if defaultWhenEmpty {
			return []string{domain.ScopeOpenID}, nil
		}
		return nil, []FieldError{{Field: "allowed_scopes", Message: "must contain at least one scope"}}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	var fields []FieldError
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || trimmed != value || len(value) > 100 {
			fields = append(fields, FieldError{Field: "allowed_scopes", Message: "contains an invalid scope"})
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, fields
}

func validTTL(field string, value *int) *FieldError {
	if value == nil {
		return nil
	}
	if *value < 60 || *value > 3600 {
		return &FieldError{Field: field, Message: "must be between 60 and 3600 seconds"}
	}
	return nil
}

func validOptionalTTL(field string, value domain.Optional[int]) *FieldError {
	if value.Value == nil {
		return nil
	}
	return validTTL(field, value.Value)
}

func newClientIdentifier(name string) (string, error) {
	suffix, err := pkg.RandomToken(6)
	if err != nil {
		return "", err
	}
	suffix = strings.ToLower(strings.NewReplacer("_", "x", "-", "x").Replace(suffix))
	slug := slugify(name)
	if slug == "" {
		slug = "client"
	}
	return slug + "-" + suffix, nil
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
		if builder.Len() >= 40 {
			break
		}
	}
	return strings.Trim(builder.String(), "-")
}
