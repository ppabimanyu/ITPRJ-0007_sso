package domain

import "time"

// AdminClientView is the console shape for a registered OIDC client.
type AdminClientView struct {
	ID                     string     `json:"id"`
	ClientID               string     `json:"client_id"`
	Name                   string     `json:"name"`
	Type                   string     `json:"type"`
	RedirectURIs           []string   `json:"redirect_uris"`
	PostLogoutRedirectURIs []string   `json:"post_logout_redirect_uris"`
	BackchannelLogoutURI   *string    `json:"backchannel_logout_uri"`
	CORSOrigins            []string   `json:"cors_origins"`
	GrantTypes             []string   `json:"grant_types"`
	AllowedScopes          []string   `json:"allowed_scopes"`
	ThirdParty             bool       `json:"third_party"`
	AccessTokenTTLSeconds  *int       `json:"access_token_ttl_seconds"`
	IDTokenTTLSeconds      *int       `json:"id_token_ttl_seconds"`
	Disabled               bool       `json:"disabled"`
	SecretRotatedAt        *time.Time `json:"secret_rotated_at"`
	CreatedAt              time.Time  `json:"created_at"`
}

// AdminClientList is the keyset-paginated admin client page.
type AdminClientList struct {
	Items     []AdminClientView `json:"items"`
	NextAfter *string           `json:"next_after"`
}

// CreateClientRequest creates a client registration. The server generates the
// public client_id.
type CreateClientRequest struct {
	Name                   string   `json:"name"`
	Type                   string   `json:"type"`
	RedirectURIs           []string `json:"redirect_uris"`
	PostLogoutRedirectURIs []string `json:"post_logout_redirect_uris,omitempty"`
	BackchannelLogoutURI   *string  `json:"backchannel_logout_uri,omitempty"`
	CORSOrigins            []string `json:"cors_origins,omitempty"`
	GrantTypes             []string `json:"grant_types,omitempty"`
	AllowedScopes          []string `json:"allowed_scopes,omitempty"`
	ThirdParty             bool     `json:"third_party,omitempty"`
	AccessTokenTTLSeconds  *int     `json:"access_token_ttl_seconds,omitempty"`
	IDTokenTTLSeconds      *int     `json:"id_token_ttl_seconds,omitempty"`
}

// UpdateClientRequest is the PATCH /clients/{id} body. Optional fields whose
// Value is nil explicitly clear the column; an unset Optional keeps it.
type UpdateClientRequest struct {
	Name                   *string          `json:"name,omitempty"`
	RedirectURIs           *[]string        `json:"redirect_uris,omitempty"`
	PostLogoutRedirectURIs *[]string        `json:"post_logout_redirect_uris,omitempty"`
	BackchannelLogoutURI   Optional[string] `json:"backchannel_logout_uri,omitzero"`
	CORSOrigins            *[]string        `json:"cors_origins,omitempty"`
	GrantTypes             *[]string        `json:"grant_types,omitempty"`
	AllowedScopes          *[]string        `json:"allowed_scopes,omitempty"`
	ThirdParty             *bool            `json:"third_party,omitempty"`
	AccessTokenTTLSeconds  Optional[int]    `json:"access_token_ttl_seconds,omitzero"`
	IDTokenTTLSeconds      Optional[int]    `json:"id_token_ttl_seconds,omitzero"`
	Disabled               *bool            `json:"disabled,omitempty"`
}

// AdminClientUpdate is the repository-level partial update.
type AdminClientUpdate struct {
	Name                   *string
	RedirectURIs           *[]string
	PostLogoutRedirectURIs *[]string
	BackchannelLogoutURI   Optional[string]
	CORSOrigins            *[]string
	GrantTypes             *[]string
	AllowedScopes          *[]string
	ThirdParty             *bool
	AccessTokenTTLSeconds  Optional[int]
	IDTokenTTLSeconds      Optional[int]
	Disabled               *bool
}

// AdminClientFilter drives the admin client keyset query.
type AdminClientFilter struct {
	After string
	Limit int
}

// ClientWithSecret is the create response. The secret is present only for a
// confidential client and only in this response.
type ClientWithSecret struct {
	AdminClientView
	ClientSecret *string `json:"client_secret"`
}

// ClientSecretResult is the one-time rotate-secret response.
type ClientSecretResult struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	RotatedAt    time.Time `json:"rotated_at"`
}
