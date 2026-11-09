package domain

import "context"

// CredentialRepository owns password material.
type CredentialRepository interface {
	Upsert(ctx context.Context, credential *Credential) error
	GetByUserID(ctx context.Context, userID UserID) (*Credential, error)
}
