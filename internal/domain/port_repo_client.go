package domain

import "context"

// ClientRepository owns OIDC clients, their child rows, and user consent.
// Create atomically writes the client, optional secret, redirect URIs, and CORS
// origins.
type ClientRepository interface {
	Create(ctx context.Context, client *Client, secret *ClientSecret, redirects []ClientRedirectURI, origins []ClientCORSOrigin) error
	GetByClientID(ctx context.Context, realmID RealmID, clientID string) (*Client, error)
	ListByRealm(ctx context.Context, realmID RealmID) ([]Client, error)
	FindRedirectURI(ctx context.Context, clientID ClientID, uri, kind string) (bool, error)
	CORSOrigins(ctx context.Context, clientID ClientID) ([]string, error)
	AuthorizeSecret(ctx context.Context, clientID ClientID, secretHash string) (bool, error)
	GetConsent(ctx context.Context, userID UserID, clientID ClientID) (*Consent, error)
	UpsertConsent(ctx context.Context, consent *Consent) error
}
