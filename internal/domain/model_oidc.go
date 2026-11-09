package domain

import (
	"strings"
	"time"
)

// AuthRequest is the Redis-only record of one authorization request. The
// browser cookie is bound by BrowserHash, so the request ID in the URL is not
// enough to complete the flow.
type AuthRequest struct {
	ID                  string
	RealmID             RealmID
	ClientID            ClientID
	ClientIDString      string
	RedirectURI         string
	Scopes              []string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
	Prompt              string
	MaxAgeSeconds       *int
	LoginHint           string
	BrowserHash         string
	SessionID           *SessionID
	UserID              *UserID
	AuthTime            *time.Time
	CreatedAt           time.Time
	ExpiresAt           time.Time
}

// HasScope reports whether the request asked for one exact scope.
func (a *AuthRequest) HasScope(scope string) bool {
	for _, candidate := range a.Scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

// ScopeString renders the space-delimited scope value used on the wire.
func (a *AuthRequest) ScopeString() string {
	return JoinScopes(a.Scopes)
}

// AuthCode is the single-use Redis record exchanged at the token endpoint.
type AuthCode struct {
	RealmID        RealmID
	ClientID       ClientID
	ClientIDString string
	RedirectURI    string
	Scopes         []string
	Nonce          string
	CodeChallenge  string
	SessionID      SessionID
	UserID         UserID
	AuthTime       time.Time
	AMR            []string
	ACR            string
	ExpiresAt      time.Time
}

// TokenSet is one token endpoint response.
type TokenSet struct {
	AccessToken  string
	IDToken      string
	RefreshToken string
	Scope        string
	ExpiresIn    int
	TokenType    string
}

// AccessTokenClaims is the signed access token payload from ARCHITECTURE §8.
type AccessTokenClaims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Audience  string   `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
	JTI       string   `json:"jti"`
	Scope     string   `json:"scope,omitempty"`
	SessionID string   `json:"sid,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	Groups    []string `json:"groups,omitempty"`
}

// IDTokenClaims is the signed ID token payload from PRD FR-OID-15.
type IDTokenClaims struct {
	Issuer            string   `json:"iss"`
	Subject           string   `json:"sub"`
	Audience          string   `json:"aud"`
	ExpiresAt         int64    `json:"exp"`
	IssuedAt          int64    `json:"iat"`
	AuthTime          int64    `json:"auth_time"`
	Nonce             string   `json:"nonce,omitempty"`
	AMR               []string `json:"amr,omitempty"`
	ACR               string   `json:"acr,omitempty"`
	AccessTokenHash   string   `json:"at_hash,omitempty"`
	SessionID         string   `json:"sid,omitempty"`
	Email             string   `json:"email,omitempty"`
	EmailVerified     *bool    `json:"email_verified,omitempty"`
	Name              string   `json:"name,omitempty"`
	GivenName         string   `json:"given_name,omitempty"`
	FamilyName        string   `json:"family_name,omitempty"`
	PreferredUsername string   `json:"preferred_username,omitempty"`
	Locale            string   `json:"locale,omitempty"`
	Roles             []string `json:"roles,omitempty"`
	Groups            []string `json:"groups,omitempty"`
}

// LogoutToken is the signed backchannel logout payload from RFC 8055.
type LogoutToken struct {
	Issuer    string         `json:"iss"`
	Subject   string         `json:"sub,omitempty"`
	Audience  string         `json:"aud"`
	IssuedAt  int64          `json:"iat"`
	JTI       string         `json:"jti"`
	Events    map[string]any `json:"events"`
	SessionID string         `json:"sid,omitempty"`
}

// JSONWebKey is one public RSA key published at the JWKS endpoint.
type JSONWebKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JSONWebKeySet is the JWKS document.
type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

// JoinScopes renders a scope list as the space-delimited wire value.
func JoinScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}

// SplitScopes parses a space-delimited scope value and drops duplicates.
func SplitScopes(raw string) []string {
	seen := map[string]struct{}{}
	scopes := make([]string, 0, 4)
	for _, field := range strings.Fields(raw) {
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		scopes = append(scopes, field)
	}
	return scopes
}
