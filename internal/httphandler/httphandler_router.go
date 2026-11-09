package httphandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/caching"
	"github.com/ppa/itprj-0007-sso/internal/infra/config"
	"github.com/ppa/itprj-0007-sso/internal/infra/database"
	"github.com/ppa/itprj-0007-sso/internal/infra/tracing"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
	"github.com/ppa/itprj-0007-sso/internal/webui"
)

// Deps is the wiring passed from cmd/server. Handlers see domain ports only.
type Deps struct {
	Config      *config.Config
	DB          *database.DB
	Redis       *redis.Client
	Logger      *slog.Logger
	Version     domain.VersionInfo
	Realms      domain.RealmRepository
	Users       domain.UserRepository
	Credentials domain.CredentialRepository
	Sessions    domain.SessionRepository
	Roles       domain.RoleRepository
	EmailTokens domain.EmailTokenRepository
	Audit       domain.AuditRepository
	CSRFStore   csrf.Store
	RateLimit   domain.RateLimitRepository

	MFA           domain.MFARepository
	MFAChallenges domain.MFAChallengeRepository
	AdminUsers    domain.AdminUserRepository
	Idempotency   domain.IdempotencyRepository
	Keyring       *pkg.Keyring

	Clients       domain.ClientRepository
	SigningKeys   domain.SigningKeyRepository
	Tokens        domain.TokenIssuer
	AuthRequests  domain.AuthRequestRepository
	AuthCodes     domain.AuthCodeRepository
	RefreshTokens domain.RefreshTokenRepository
	TokenState    domain.TokenStateRepository
	Outbox        domain.OutboxRepository

	AdminClients  domain.AdminClientRepository
	AdminRoles    domain.AdminRoleRepository
	Groups        domain.GroupRepository
	AdminSessions domain.AdminSessionRepository
	AdminAudit    domain.AdminAuditRepository
	RealmSettings domain.RealmSettingsRepository
}

type handlers struct {
	deps Deps
}

// NewRouter builds the global middleware chain in the order fixed by
// ARCHITECTURE §4 and mounts the M1 and M2 routes.
func NewRouter(deps Deps) http.Handler {
	router := chi.NewRouter()
	router.Use(RequestID)
	router.Use(tracing.Middleware)
	router.Use(RealIP(deps.Config.Proxies.TrustedCIDRs))
	router.Use(Recoverer)
	router.Use(SecurityHeaders(deps.Config.IsProduction()))
	router.Use(BodyLimit)
	router.Use(Logger(deps.Logger))
	router.Use(RealmResolver)

	h := &handlers{deps: deps}

	router.Get("/healthz", healthz)
	router.Get("/readyz", readyz(deps))
	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/version", version(deps.Version))

		r.Group(func(auth chi.Router) {
			auth.Use(originAllowed(deps.Config))
			auth.Get("/auth/csrf", h.csrfToken)
			auth.Get("/auth/authorize/request", h.authorizeRequestInfo)

			auth.Group(func(pre chi.Router) {
				pre.Use(preSessionCSRF(deps.CSRFStore))
				pre.Post("/auth/login", h.login)
				pre.Post("/auth/forgot-password", h.forgotPassword)
				pre.Post("/auth/reset-password", h.resetPassword)
				pre.Post("/auth/verify-email", h.verifyEmail)
				pre.Post("/auth/accept-invite", h.acceptInvite)
				pre.Post("/auth/mfa/verify", h.verifyMFA)
			})

			auth.Group(func(session chi.Router) {
				session.Use(SessionAuth(deps))
				session.Get("/auth/session", h.sessionInfo)
				session.With(sessionCSRF).Post("/auth/logout", h.logout)
				session.With(MFASatisfied(deps), sessionCSRF).Post("/auth/authorize/complete", h.completeAuthorize)
				session.With(MFASatisfied(deps), sessionCSRF).Post("/auth/consent", h.grantConsent)
				session.With(sessionCSRF).Post("/auth/mfa/enroll", h.enrollMFA)
			})
		})

		r.Group(func(me chi.Router) {
			me.Use(SessionAuth(deps))
			me.Use(MFASatisfied(deps))
			me.Get("/me", h.getMe)
			me.With(sessionCSRF).Patch("/me", h.updateMe)
			me.With(sessionCSRF).Post("/me/password", h.changePassword)
			me.Get("/me/sessions", h.listMySessions)
			me.With(sessionCSRF).Delete("/me/sessions/{id}", h.revokeMySession)
			me.With(sessionCSRF).Post("/me/mfa/totp", h.enrollMFA)
			me.With(sessionCSRF).Delete("/me/mfa/totp", h.deleteMyTOTP)
			me.With(sessionCSRF).Post("/me/mfa/recovery-codes", h.regenerateRecoveryCodes)
		})

		r.Group(func(admin chi.Router) {
			admin.Use(SessionAuth(deps))
			admin.Use(AdminOnly(deps))
			admin.Get("/users", h.listUsers)
			admin.With(sessionCSRF).Post("/users", h.createUser)
			admin.Get("/users/{id}", h.getUser)
			admin.With(sessionCSRF).Patch("/users/{id}", h.updateUser)
			admin.With(sessionCSRF).Post("/users/{id}/reset-password", h.resetUserPassword)
			admin.With(sessionCSRF).Post("/users/{id}/invite", h.resendUserInvite)
			admin.With(sessionCSRF).Post("/users/{id}/roles", h.assignUserRoles)

			admin.Get("/clients", h.listClients)
			admin.With(sessionCSRF).Post("/clients", h.createClient)
			admin.Get("/clients/{id}", h.getClient)
			admin.With(sessionCSRF).Patch("/clients/{id}", h.updateClient)
			admin.With(sessionCSRF).Delete("/clients/{id}", h.deleteClient)
			admin.With(sessionCSRF).Post("/clients/{id}/rotate-secret", h.rotateClientSecret)

			admin.Get("/roles", h.listRoles)
			admin.With(sessionCSRF).Post("/roles", h.createRole)

			admin.Get("/groups", h.listGroups)
			admin.With(sessionCSRF).Post("/groups", h.createGroup)
			admin.Get("/groups/{id}", h.getGroup)
			admin.With(sessionCSRF).Patch("/groups/{id}", h.updateGroup)
			admin.With(sessionCSRF).Delete("/groups/{id}", h.deleteGroup)
			admin.Get("/groups/{id}/members", h.listGroupMembers)
			admin.With(sessionCSRF).Post("/groups/{id}/members", h.addGroupMembers)
			admin.With(sessionCSRF).Delete("/groups/{id}/members/{user_id}", h.removeGroupMember)

			admin.Get("/sessions", h.listSessions)
			admin.With(sessionCSRF).Delete("/sessions/{id}", h.revokeSession)

			admin.Get("/audit", h.listAuditEvents)

			admin.Get("/realm/settings", h.getRealmSettings)
			admin.With(sessionCSRF).Patch("/realm/settings", h.updateRealmSettings)
		})
	})

	router.Route("/realms/{realm}", func(r chi.Router) {
		r.Get("/.well-known/openid-configuration", h.discovery)
		r.Get("/jwks", h.jwks)
		r.Get("/authorize", h.authorize)
		r.With(OIDCCORS(deps)).Post("/token", h.token)
		r.With(OIDCCORS(deps)).Options("/token", preflight)
		r.With(OIDCCORS(deps)).Get("/userinfo", h.userinfo)
		r.With(OIDCCORS(deps)).Post("/userinfo", h.userinfo)
		r.With(OIDCCORS(deps)).Options("/userinfo", preflight)
		r.With(OIDCCORS(deps)).Post("/revoke", h.revoke)
		r.With(OIDCCORS(deps)).Options("/revoke", preflight)
		r.With(OIDCCORS(deps)).Post("/introspect", h.introspect)
		r.With(OIDCCORS(deps)).Options("/introspect", preflight)
		r.Get("/logout", h.oidcLogout)
		r.Post("/logout", h.oidcLogout)
		r.Post("/logout/backchannel", h.backchannelLogout)
	})

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && !isAPIPath(r.URL.Path) {
			webui.Handler().ServeHTTP(w, r)
			return
		}
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "no route matches this request")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, http.StatusMethodNotAllowed, domain.CodeBadRequest, "method not allowed for this route")
	})

	return router
}

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/realms/")
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyz(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{
			"database":   "ok",
			"redis":      "ok",
			"migrations": "ok",
		}
		ready := true

		if err := deps.DB.Ping(ctx); err != nil {
			checks["database"] = "error"
			checks["migrations"] = "error"
			ready = false
		} else {
			version, err := deps.DB.Version(ctx)
			latest, latestErr := database.LatestVersion()
			if err != nil || latestErr != nil || version != latest {
				checks["migrations"] = "error"
				ready = false
			}
		}
		if err := caching.Ping(ctx, deps.Redis); err != nil {
			checks["redis"] = "error"
			ready = false
		}

		status := http.StatusOK
		state := "ready"
		if !ready {
			status = http.StatusServiceUnavailable
			state = "unready"
		}
		writeJSON(w, status, map[string]any{"status": state, "checks": checks})
	}
}

func version(info domain.VersionInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, info)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
