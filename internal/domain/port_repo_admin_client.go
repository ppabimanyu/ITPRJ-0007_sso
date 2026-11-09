package domain

import (
	"context"
	"time"
)

// AdminClientRepository owns the console client operations. Mutations take the
// audit event so the change and its evidence commit together.
type AdminClientRepository interface {
	ListAdmin(ctx context.Context, realmID RealmID, filter AdminClientFilter) ([]AdminClientView, string, error)
	GetAdmin(ctx context.Context, realmID RealmID, clientID ClientID) (*AdminClientView, error)
	CreateWithAudit(ctx context.Context, client *Client, secret *ClientSecret, redirects []ClientRedirectURI, origins []ClientCORSOrigin, audit *AuditEvent) error
	UpdateAdmin(ctx context.Context, realmID RealmID, clientID ClientID, update AdminClientUpdate, audit *AuditEvent) (*AdminClientView, error)
	RotateSecret(ctx context.Context, realmID RealmID, clientID ClientID, secretHash string, audit *AuditEvent) (time.Time, error)
	Delete(ctx context.Context, realmID RealmID, clientID ClientID, audit *AuditEvent) error
}
