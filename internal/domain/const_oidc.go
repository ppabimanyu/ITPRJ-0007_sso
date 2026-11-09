package domain

// OAuth grant and response values accepted by the provider.
const (
	GrantTypeAuthorizationCode = "authorization_code"
	GrantTypeRefreshToken      = "refresh_token"
	GrantTypeClientCredentials = "client_credentials"

	ResponseTypeCode  = "code"
	ResponseModeQuery = "query"

	CodeChallengeMethodS256 = "S256"

	TokenTypeBearer = "Bearer"
)

// OAuth error codes used by the OIDC endpoints.
const (
	OAuthInvalidRequest       = "invalid_request"
	OAuthInvalidClient        = "invalid_client"
	OAuthInvalidGrant         = "invalid_grant"
	OAuthUnauthorizedClient   = "unauthorized_client"
	OAuthUnsupportedGrantType = "unsupported_grant_type"
	OAuthUnsupportedResponse  = "unsupported_response_type"
	OAuthInvalidScope         = "invalid_scope"
	OAuthAccessDenied         = "access_denied"
	OAuthLoginRequired        = "login_required"
	OAuthInteractionRequired  = "interaction_required"
	OAuthInvalidToken         = "invalid_token"
	OAuthServerError          = "server_error"
)

// Backchannel logout event identifier from RFC 8055.
const BackchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"

// ACR, AMR, scope and audit names used by the OIDC handlers.
const (
	AuditAuthorizeDenied = "oidc.authorize.denied"
	AuditAuthCodeIssued  = "oidc.auth_code.issued"
	AuditTokenIssued     = "oidc.token.issued"
	AuditRefreshReused   = "oidc.refresh.reused"
	AuditRefreshRevoked  = "oidc.refresh.revoked"
	AuditOIDCLogout      = "oidc.logout"
	AuditKeyRotated      = "oidc.key.rotated"
)

// Outbox event type for backchannel logout.
const OutboxLogoutBackchannel = "logout.backchannel"
