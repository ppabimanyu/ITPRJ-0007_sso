package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

type memoryKeyStore struct {
	keys []domain.RealmSigningKey
}

func (m *memoryKeyStore) Create(_ context.Context, key *domain.RealmSigningKey) error {
	m.keys = append(m.keys, *key)
	return nil
}

func (m *memoryKeyStore) GetActive(_ context.Context, realmID domain.RealmID) (*domain.RealmSigningKey, error) {
	for i := range m.keys {
		if m.keys[i].RealmID == realmID && m.keys[i].Active {
			return &m.keys[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *memoryKeyStore) ListActive(_ context.Context, realmID domain.RealmID) ([]domain.RealmSigningKey, error) {
	var out []domain.RealmSigningKey
	for _, key := range m.keys {
		if key.RealmID == realmID && key.Active {
			out = append(out, key)
		}
	}
	return out, nil
}

func (m *memoryKeyStore) ListPublished(_ context.Context, realmID domain.RealmID) ([]domain.RealmSigningKey, error) {
	now := time.Now().UTC()
	var out []domain.RealmSigningKey
	for _, key := range m.keys {
		if key.RealmID != realmID {
			continue
		}
		if key.Active || (key.RetireAt != nil && key.RetireAt.After(now)) {
			out = append(out, key)
		}
	}
	return out, nil
}

func (m *memoryKeyStore) Rotate(_ context.Context, realmID domain.RealmID, next *domain.RealmSigningKey, retireAt time.Time) error {
	for i := range m.keys {
		if m.keys[i].RealmID == realmID && m.keys[i].Active {
			m.keys[i].Active = false
			m.keys[i].RetireAt = &retireAt
		}
	}
	m.keys = append(m.keys, *next)
	return nil
}

func testKeyring(t *testing.T) *pkg.Keyring {
	t.Helper()
	ring, err := pkg.NewKeyring(map[string][]byte{"test": make([]byte, 32)}, "test")
	if err != nil {
		t.Fatal(err)
	}
	return ring
}

func TestKeyRotationKeepsOldTokensVerifying(t *testing.T) {
	ctx := context.Background()
	ring := testKeyring(t)
	store := &memoryKeyStore{}
	issuer := NewJWTIssuer(store, ring)
	realmID := domain.RealmID("realm-1")

	first, err := GenerateSigningKey(ring, realmID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	oldToken, err := issuer.SignAccessToken(ctx, realmID, domain.AccessTokenClaims{
		Issuer: "issuer", Subject: "user", Audience: "client",
		ExpiresAt: time.Now().Add(time.Hour).Unix(), IssuedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := GenerateSigningKey(ring, realmID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Rotate(ctx, realmID, second, time.Now().UTC().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.SignAccessToken(ctx, realmID, domain.AccessTokenClaims{
		Issuer: "issuer", Subject: "user", Audience: "client",
		ExpiresAt: time.Now().Add(time.Hour).Unix(), IssuedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := issuer.VerifyAccessToken(ctx, realmID, oldToken); err != nil {
		t.Fatalf("old token stopped verifying inside the retire window: %v", err)
	}
	set, err := issuer.JWKS(ctx, realmID)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("JWKS key count = %d, want 2", len(set.Keys))
	}
	foundOld := false
	for _, key := range set.Keys {
		if key.Kid == first.KID {
			foundOld = true
		}
	}
	if !foundOld {
		t.Fatal("retired key is missing from JWKS inside the retire window")
	}

	past := time.Now().UTC().Add(-time.Hour)
	store.keys[0].RetireAt = &past
	if _, err := issuer.VerifyAccessToken(ctx, realmID, oldToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("old token after retire error = %v, want ErrTokenInvalid", err)
	}
}

func TestClassifyRefreshToken(t *testing.T) {
	now := time.Now().UTC()
	live := func() *domain.RefreshToken {
		return &domain.RefreshToken{
			ExpiresAt:         now.Add(time.Hour),
			AbsoluteExpiresAt: now.Add(24 * time.Hour),
		}
	}
	cases := []struct {
		name string
		edit func(*domain.RefreshToken)
		want refreshTokenState
	}{
		{name: "valid", edit: func(*domain.RefreshToken) {}, want: refreshTokenValid},
		{name: "reused", edit: func(token *domain.RefreshToken) { used := now.Add(-time.Minute); token.UsedAt = &used }, want: refreshTokenReused},
		{name: "expired", edit: func(token *domain.RefreshToken) { token.ExpiresAt = now.Add(-time.Second) }, want: refreshTokenExpired},
		{name: "revoked", edit: func(token *domain.RefreshToken) { revoked := now.Add(-time.Minute); token.RevokedAt = &revoked }, want: refreshTokenRevoked},
	}
	for _, tc := range cases {
		token := live()
		tc.edit(token)
		if got := classifyRefreshToken(token, now); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
