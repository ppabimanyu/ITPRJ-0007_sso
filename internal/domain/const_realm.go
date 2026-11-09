package domain

// Realm and protocol constants shared by handlers, repositories, and the CLI.
const (
	DefaultRealmName  = "default"
	PlatformAdminRole = "platform-admin"
	MemberRole        = "member"
)

// OIDC scopes.
const (
	ScopeOpenID        = "openid"
	ScopeProfile       = "profile"
	ScopeEmail         = "email"
	ScopeOfflineAccess = "offline_access"
	ScopeRoles         = "roles"
	ScopeGroups        = "groups"
)

// Authentication context and method values.
const (
	ACRPassword = "urn:itprj:sso:acr:pwd"
	ACRMFA      = "urn:itprj:sso:acr:mfa"

	AMRPassword = "pwd"
	AMROTP      = "otp"
)

// Client and redirect kinds.
const (
	ClientTypePublic       = "public"
	ClientTypeConfidential = "confidential"

	RedirectKindAuthorize  = "authorize"
	RedirectKindPostLogout = "post_logout"
)

// Email token purposes.
const (
	EmailTokenVerify = "verify"
	EmailTokenReset  = "reset"
	EmailTokenInvite = "invite"
)

// Audit actor types and outbox states.
const (
	ActorTypeUser   = "user"
	ActorTypeAdmin  = "admin"
	ActorTypeClient = "client"
	ActorTypeSystem = "system"

	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusSent       = "sent"
	OutboxStatusFailed     = "failed"
)
