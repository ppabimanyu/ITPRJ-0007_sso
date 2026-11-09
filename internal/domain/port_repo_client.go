package domain

import "context"

// ClientRepository owns OIDC clients and their child rows. Create atomically
// writes the client, optional secret, redirect URIs, and CORS origins.
type ClientRepository interface {
	Create(ctx context.Context, client *Client, secret *ClientSecret, redirects []ClientRedirectURI, origins []ClientCORSOrigin) error
	GetByClientID(ctx context.Context, realmID RealmID, clientID string) (*Client, error)
	ListByRealm(ctx context.Context, realmID RealmID) ([]Client, error)
}
