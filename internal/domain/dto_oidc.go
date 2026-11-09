package domain

// AuthRequestInfo is the body of GET /api/v1/auth/authorize/request. The SPA
// uses it to show the client and the scopes before login or consent.
type AuthRequestInfo struct {
	RequestID       string   `json:"request_id"`
	ClientID        string   `json:"client_id"`
	ClientName      string   `json:"client_name"`
	Scopes          []string `json:"scopes"`
	Prompt          *string  `json:"prompt"`
	LoginHint       string   `json:"login_hint,omitempty"`
	LoginRequired   bool     `json:"login_required"`
	ConsentRequired bool     `json:"consent_required"`
}

// CompleteAuthorizeRequest finishes an authorization request after login.
type CompleteAuthorizeRequest struct {
	RequestID string `json:"request_id"`
}

// ConsentRequest grants scopes to a third-party client.
type ConsentRequest struct {
	RequestID string   `json:"request_id"`
	Scopes    []string `json:"scopes"`
}

// RedirectResponse tells the SPA where to send the browser next.
type RedirectResponse struct {
	RedirectTo string `json:"redirect_to"`
}

// TokenResponse is the token endpoint body from RFC 6749 §5.1.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// IntrospectionResponse is the RFC 7662 body.
type IntrospectionResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	Subject   string `json:"sub,omitempty"`
	Audience  string `json:"aud,omitempty"`
	Issuer    string `json:"iss,omitempty"`
	JTI       string `json:"jti,omitempty"`
	SessionID string `json:"sid,omitempty"`
}

// UserInfo is the OIDC userinfo body. Claims appear only when scope allows.
type UserInfo map[string]any
