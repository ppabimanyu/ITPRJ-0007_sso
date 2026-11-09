package httphandler

import (
	"encoding/json"
	"net/http"

	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

type discoveryDocument struct {
	Issuer                                    string   `json:"issuer"`
	AuthorizationEndpoint                     string   `json:"authorization_endpoint"`
	TokenEndpoint                             string   `json:"token_endpoint"`
	UserinfoEndpoint                          string   `json:"userinfo_endpoint"`
	JwksURI                                   string   `json:"jwks_uri"`
	RevocationEndpoint                        string   `json:"revocation_endpoint"`
	IntrospectionEndpoint                     string   `json:"introspection_endpoint"`
	EndSessionEndpoint                        string   `json:"end_session_endpoint"`
	ResponseTypesSupported                    []string `json:"response_types_supported"`
	ResponseModesSupported                    []string `json:"response_modes_supported"`
	GrantTypesSupported                       []string `json:"grant_types_supported"`
	ScopesSupported                           []string `json:"scopes_supported"`
	ClaimsSupported                           []string `json:"claims_supported"`
	CodeChallengeMethodsSupported             []string `json:"code_challenge_methods_supported"`
	SubjectTypesSupported                     []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported          []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported         []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported    []string `json:"revocation_endpoint_auth_methods_supported"`
	IntrospectionEndpointAuthMethodsSupported []string `json:"introspection_endpoint_auth_methods_supported"`
	RequestParameterSupported                 bool     `json:"request_parameter_supported"`
	RequestURIParameterSupported              bool     `json:"request_uri_parameter_supported"`
	ClaimsParameterSupported                  bool     `json:"claims_parameter_supported"`
}

var (
	supportedScopes = []string{
		domain.ScopeOpenID, domain.ScopeProfile, domain.ScopeEmail,
		domain.ScopeOfflineAccess, domain.ScopeRoles, domain.ScopeGroups,
	}
	supportedClaims = []string{
		"iss", "sub", "aud", "exp", "iat", "auth_time", "nonce", "amr", "acr",
		"at_hash", "sid", "email", "email_verified", "name", "given_name",
		"family_name", "preferred_username", "locale", "roles", "groups",
	}
	clientAuthMethods = []string{"client_secret_basic", "client_secret_post", "none"}
)

func (h *handlers) discovery(w http.ResponseWriter, r *http.Request) {
	realm, err := h.realm(r.Context())
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "realm was not found")
		return
	}
	issuer := realmIssuer(h.deps.Config.HTTP.ExternalURL, realm)
	writeJSON(w, http.StatusOK, discoveryDocument{
		Issuer:                                    issuer,
		AuthorizationEndpoint:                     issuer + "/authorize",
		TokenEndpoint:                             issuer + "/token",
		UserinfoEndpoint:                          issuer + "/userinfo",
		JwksURI:                                   issuer + "/jwks",
		RevocationEndpoint:                        issuer + "/revoke",
		IntrospectionEndpoint:                     issuer + "/introspect",
		EndSessionEndpoint:                        issuer + "/logout",
		ResponseTypesSupported:                    []string{string(oidc.ResponseTypeCode)},
		ResponseModesSupported:                    []string{string(oidc.ResponseModeQuery)},
		GrantTypesSupported:                       []string{string(oidc.GrantTypeCode), string(oidc.GrantTypeRefreshToken), string(oidc.GrantTypeClientCredentials)},
		ScopesSupported:                           supportedScopes,
		ClaimsSupported:                           supportedClaims,
		CodeChallengeMethodsSupported:             []string{string(oidc.CodeChallengeMethodS256)},
		SubjectTypesSupported:                     []string{"public"},
		IDTokenSigningAlgValuesSupported:          []string{"RS256"},
		TokenEndpointAuthMethodsSupported:         clientAuthMethods,
		RevocationEndpointAuthMethodsSupported:    clientAuthMethods,
		IntrospectionEndpointAuthMethodsSupported: clientAuthMethods,
		RequestParameterSupported:                 false,
		RequestURIParameterSupported:              false,
		ClaimsParameterSupported:                  false,
	})
}

func (h *handlers) jwks(w http.ResponseWriter, r *http.Request) {
	realm, err := h.realm(r.Context())
	if err != nil {
		h.logInternal(r.Context(), "realm_lookup_failed", err)
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "realm was not found")
		return
	}
	if h.deps.Tokens == nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"signing keys are unavailable")
		return
	}
	set, err := h.deps.Tokens.JWKS(r.Context(), realm.ID)
	if err != nil {
		logger.FromContext(r.Context()).Error("jwks_load_failed", "error", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"signing keys are unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(set)
}
