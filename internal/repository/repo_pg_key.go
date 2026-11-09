package repository

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

const signingKeyColumns = `id::text, realm_id::text, kid, algorithm,
	private_key_encrypted, active, created_at, retire_at`

// PgSigningKeyRepository stores encrypted RSA private keys. It embeds the JWT
// issuer so the active key, retired keys, and token verification share one
// source of truth.
type PgSigningKeyRepository struct {
	pool *pgxpool.Pool
	ring *pkg.Keyring
	*JWTIssuer
}

func NewSigningKeyRepository(pool *pgxpool.Pool, ring *pkg.Keyring) *PgSigningKeyRepository {
	repository := &PgSigningKeyRepository{pool: pool, ring: ring}
	repository.JWTIssuer = NewJWTIssuer(repository, ring)
	return repository
}

// GenerateSigningKey creates RSA-3072 material and encrypts the PKCS#8 key with
// the active master key. The KID is the RFC 7638 thumbprint of the public key.
func GenerateSigningKey(ring *pkg.Keyring, realmID domain.RealmID) (*domain.RealmSigningKey, error) {
	if ring == nil {
		return nil, errors.New("master keyring is required")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	encrypted, err := ring.Encrypt(der)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}
	thumbprint, err := (&jose.JSONWebKey{Key: &privateKey.PublicKey}).Thumbprint(crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("thumbprint public key: %w", err)
	}
	return &domain.RealmSigningKey{
		ID:                  domain.SigningKeyID(uuid.NewString()),
		RealmID:             realmID,
		KID:                 base64.RawURLEncoding.EncodeToString(thumbprint),
		Algorithm:           string(jose.RS256),
		PrivateKeyEncrypted: encrypted,
		Active:              true,
	}, nil
}

func (r *PgSigningKeyRepository) Create(ctx context.Context, key *domain.RealmSigningKey) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO realm_signing_key (id, realm_id, kid, algorithm, private_key_encrypted, active, retire_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6, $7)
		RETURNING created_at`,
		string(key.ID), string(key.RealmID), key.KID, key.Algorithm,
		key.PrivateKeyEncrypted, key.Active, key.RetireAt,
	).Scan(&key.CreatedAt)
}

func (r *PgSigningKeyRepository) GetActive(ctx context.Context, realmID domain.RealmID) (*domain.RealmSigningKey, error) {
	return scanSigningKey(r.pool.QueryRow(ctx, `
		SELECT `+signingKeyColumns+`
		FROM realm_signing_key
		WHERE realm_id = $1::text::uuid AND active
		LIMIT 1`, string(realmID)))
}

func (r *PgSigningKeyRepository) ListActive(ctx context.Context, realmID domain.RealmID) ([]domain.RealmSigningKey, error) {
	return r.list(ctx, `
		SELECT `+signingKeyColumns+`
		FROM realm_signing_key
		WHERE realm_id = $1::text::uuid AND active
		ORDER BY created_at DESC`, realmID)
}

// ListPublished includes the active key and retired keys still inside their
// retire window, so a rotation does not invalidate tokens signed just before.
func (r *PgSigningKeyRepository) ListPublished(ctx context.Context, realmID domain.RealmID) ([]domain.RealmSigningKey, error) {
	return r.list(ctx, `
		SELECT `+signingKeyColumns+`
		FROM realm_signing_key
		WHERE realm_id = $1::text::uuid
			AND (active OR (retire_at IS NOT NULL AND retire_at > now()))
		ORDER BY active DESC, created_at DESC`, realmID)
}

func (r *PgSigningKeyRepository) list(ctx context.Context, query string, realmID domain.RealmID) ([]domain.RealmSigningKey, error) {
	rows, err := r.pool.Query(ctx, query, string(realmID))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var keys []domain.RealmSigningKey
	for rows.Next() {
		key, err := scanSigningKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, *key)
	}
	return keys, mapError(rows.Err())
}

func (r *PgSigningKeyRepository) Rotate(ctx context.Context, realmID domain.RealmID, next *domain.RealmSigningKey, retireAt time.Time) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE realm_signing_key
			SET active = false, retire_at = $2
			WHERE realm_id = $1::text::uuid AND active`,
			string(realmID), retireAt); err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO realm_signing_key (id, realm_id, kid, algorithm, private_key_encrypted, active, retire_at)
			VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6, $7)`,
			string(next.ID), string(next.RealmID), next.KID, next.Algorithm,
			next.PrivateKeyEncrypted, next.Active, next.RetireAt); err != nil {
			return mapError(err)
		}
		return nil
	})
}

func (r *PgSigningKeyRepository) RotateKey(ctx context.Context, realmID domain.RealmID, retireAfter time.Duration) (*domain.RealmSigningKey, error) {
	next, err := GenerateSigningKey(r.ring, realmID)
	if err != nil {
		return nil, err
	}
	if err := r.Rotate(ctx, realmID, next, time.Now().UTC().Add(retireAfter)); err != nil {
		return nil, err
	}
	return next, nil
}

// JWTIssuer signs and verifies realm tokens. It reads the key material through
// the SigningKeyRepository so a unit test can supply an in-memory store.
type JWTIssuer struct {
	keys domain.SigningKeyRepository
	ring *pkg.Keyring
}

func NewJWTIssuer(keys domain.SigningKeyRepository, ring *pkg.Keyring) *JWTIssuer {
	return &JWTIssuer{keys: keys, ring: ring}
}

func (j *JWTIssuer) SignAccessToken(ctx context.Context, realmID domain.RealmID, claims domain.AccessTokenClaims) (string, error) {
	return j.sign(ctx, realmID, "JWT", claims)
}

func (j *JWTIssuer) SignIDToken(ctx context.Context, realmID domain.RealmID, claims domain.IDTokenClaims) (string, error) {
	return j.sign(ctx, realmID, "JWT", claims)
}

func (j *JWTIssuer) SignLogoutToken(ctx context.Context, realmID domain.RealmID, token domain.LogoutToken) (string, error) {
	return j.sign(ctx, realmID, "logout+jwt", token)
}

func (j *JWTIssuer) sign(ctx context.Context, realmID domain.RealmID, typ string, payload any) (string, error) {
	key, err := j.keys.GetActive(ctx, realmID)
	if err != nil {
		return "", err
	}
	privateKey, err := j.privateKey(key)
	if err != nil {
		return "", err
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key: jose.JSONWebKey{
				Key:       privateKey,
				KeyID:     key.KID,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
		(&jose.SignerOptions{}).WithType(jose.ContentType(typ)),
	)
	if err != nil {
		return "", fmt.Errorf("create signer: %w", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	signed, err := signer.Sign(body)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed.CompactSerialize()
}

func (j *JWTIssuer) VerifyAccessToken(ctx context.Context, realmID domain.RealmID, raw string) (*domain.AccessTokenClaims, error) {
	body, _, err := j.verify(ctx, realmID, raw)
	if err != nil {
		return nil, err
	}
	var claims domain.AccessTokenClaims
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, fmt.Errorf("decode access token: %w", err)
	}
	return &claims, nil
}

func (j *JWTIssuer) VerifyIDToken(ctx context.Context, realmID domain.RealmID, raw string) (*domain.IDTokenClaims, error) {
	body, _, err := j.verify(ctx, realmID, raw)
	if err != nil {
		return nil, err
	}
	var claims domain.IDTokenClaims
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, fmt.Errorf("decode id token: %w", err)
	}
	return &claims, nil
}

func (j *JWTIssuer) VerifyLogoutToken(ctx context.Context, realmID domain.RealmID, raw string) (*domain.LogoutToken, error) {
	body, header, err := j.verify(ctx, realmID, raw)
	if err != nil {
		return nil, err
	}
	if typ, _ := header.ExtraHeaders[jose.HeaderType].(string); typ != "logout+jwt" {
		return nil, domain.ErrTokenInvalid
	}
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("decode logout token: %w", err)
	}
	if _, ok := fields["nonce"]; ok {
		return nil, domain.ErrTokenInvalid
	}
	events, ok := fields["events"].(map[string]any)
	if !ok {
		return nil, domain.ErrTokenInvalid
	}
	if _, ok := events[domain.BackchannelLogoutEvent]; !ok {
		return nil, domain.ErrTokenInvalid
	}
	var token domain.LogoutToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("decode logout claims: %w", err)
	}
	return &token, nil
}

func (j *JWTIssuer) JWKS(ctx context.Context, realmID domain.RealmID) (*domain.JSONWebKeySet, error) {
	keys, err := j.keys.ListPublished(ctx, realmID)
	if err != nil {
		return nil, err
	}
	set := &domain.JSONWebKeySet{Keys: make([]domain.JSONWebKey, 0, len(keys))}
	for i := range keys {
		privateKey, err := j.privateKey(&keys[i])
		if err != nil {
			return nil, err
		}
		raw, err := (jose.JSONWebKey{
			Key:       &privateKey.PublicKey,
			KeyID:     keys[i].KID,
			Algorithm: keys[i].Algorithm,
			Use:       "sig",
		}).MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("marshal jwk: %w", err)
		}
		var key domain.JSONWebKey
		if err := json.Unmarshal(raw, &key); err != nil {
			return nil, fmt.Errorf("decode jwk: %w", err)
		}
		set.Keys = append(set.Keys, key)
	}
	return set, nil
}

func (j *JWTIssuer) verify(ctx context.Context, realmID domain.RealmID, raw string) ([]byte, jose.Header, error) {
	signed, err := jose.ParseSignedCompact(raw, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil || len(signed.Signatures) == 0 {
		return nil, jose.Header{}, domain.ErrTokenInvalid
	}
	header := signed.Signatures[0].Header
	if header.KeyID == "" {
		return nil, jose.Header{}, domain.ErrTokenInvalid
	}
	key, err := j.publishedKey(ctx, realmID, header.KeyID)
	if err != nil {
		return nil, jose.Header{}, err
	}
	privateKey, err := j.privateKey(key)
	if err != nil {
		return nil, jose.Header{}, err
	}
	body, err := signed.Verify(&privateKey.PublicKey)
	if err != nil {
		return nil, jose.Header{}, domain.ErrTokenInvalid
	}
	return body, header, nil
}

func (j *JWTIssuer) publishedKey(ctx context.Context, realmID domain.RealmID, kid string) (*domain.RealmSigningKey, error) {
	keys, err := j.keys.ListPublished(ctx, realmID)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].KID == kid {
			return &keys[i], nil
		}
	}
	return nil, domain.ErrTokenInvalid
}

func (j *JWTIssuer) privateKey(key *domain.RealmSigningKey) (*rsa.PrivateKey, error) {
	if j.ring == nil {
		return nil, errors.New("master keyring is not configured")
	}
	der, err := j.ring.Decrypt(key.PrivateKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt signing key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse signing key: %w", err)
	}
	privateKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("signing key is not RSA")
	}
	return privateKey, nil
}

func scanSigningKey(row rowScanner) (*domain.RealmSigningKey, error) {
	var key domain.RealmSigningKey
	err := row.Scan(&key.ID, &key.RealmID, &key.KID, &key.Algorithm,
		&key.PrivateKeyEncrypted, &key.Active, &key.CreatedAt, &key.RetireAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &key, nil
}
