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
	EmailTokenVerify EmailTokenPurpose = "verify"
	EmailTokenReset  EmailTokenPurpose = "reset"
	EmailTokenInvite EmailTokenPurpose = "invite"
)

// Audit actor types.
const (
	ActorTypeUser   = "user"
	ActorTypeAdmin  = "admin"
	ActorTypeClient = "client"
	ActorTypeSystem = "system"
)

// Audit event types written in M2.
const (
	AuditLoginSuccess      = "auth.login.success"
	AuditLoginFailure      = "auth.login.failure"
	AuditLogout            = "auth.logout"
	AuditPasswordReset     = "auth.password.reset"
	AuditPasswordChanged   = "auth.password.changed"
	AuditInviteAccepted    = "auth.invite.accepted"
	AuditEmailVerified     = "auth.email.verified"
	AuditSessionRevoked    = "auth.session.revoked"
	AuditPasswordRequested = "auth.password.reset_requested"

	AuditMFAChallengeCreated = "auth.mfa.challenge"
	AuditMFAEnrolled         = "auth.mfa.enrolled"
	AuditMFADisabled         = "auth.mfa.disabled"
	AuditMFAVerified         = "auth.mfa.verified"
	AuditMFAFailed           = "auth.mfa.failed"
	AuditMFARecoveryUsed     = "auth.mfa.recovery_used"
	AuditMFARecoveryCreated  = "auth.mfa.recovery_created"

	AuditAdminAccessDenied    = "admin.access.denied"
	AuditAdminUserCreated     = "admin.user.created"
	AuditAdminUserUpdated     = "admin.user.updated"
	AuditAdminUserDisabled    = "admin.user.disabled"
	AuditAdminUserInvited     = "admin.user.invited"
	AuditAdminUserRoles       = "admin.user.roles_changed"
	AuditAdminPasswordRequest = "admin.user.password_reset_requested"
)

// Outbox event types written in M2.
const (
	OutboxEmailInvite = "email.invite"
	OutboxEmailVerify = "email.verify"
	OutboxEmailReset  = "email.reset"
)
