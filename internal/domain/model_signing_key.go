package domain

import "time"

// RealmSigningKey is an RSA private key encrypted with the active master key.
type RealmSigningKey struct {
	ID                  SigningKeyID
	RealmID             RealmID
	KID                 string
	Algorithm           string
	PrivateKeyEncrypted []byte
	Active              bool
	CreatedAt           time.Time
	RetireAt            *time.Time
}
