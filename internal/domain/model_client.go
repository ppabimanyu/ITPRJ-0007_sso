package domain

import "time"

// Client is an OAuth/OIDC relying party registered in a realm.
type Client struct {
	ID                    ClientID
	RealmID               RealmID
	ClientID              string
	Type                  string
	GrantTypes            []string
	AllowedScopes         []string
	ThirdParty            bool
	AccessTokenTTLSeconds *int
	IDTokenTTLSeconds     *int
	BackchannelLogoutURI  *string
	DisabledAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// ClientSecret holds a hashed confidential-client secret.
type ClientSecret struct {
	ID         string
	ClientID   ClientID
	SecretHash string
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

// ClientRedirectURI is an exact-match redirect or post-logout URI.
type ClientRedirectURI struct {
	ClientID  ClientID
	URI       string
	Kind      string
	CreatedAt time.Time
}

// ClientCORSOrigin is an origin allowed to call OIDC endpoints for a client.
type ClientCORSOrigin struct {
	ClientID  ClientID
	Origin    string
	CreatedAt time.Time
}
