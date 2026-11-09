package httphandler

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/config"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
	"github.com/ppa/itprj-0007-sso/internal/pkg/csrf"
)

type realmContextKey struct{}

// RealmFromContext returns the realm selected by RealmResolver.
func RealmFromContext(ctx context.Context) string {
	realm, _ := ctx.Value(realmContextKey{}).(string)
	return realm
}

type currentSessionKey struct{}

// CurrentSession is the authenticated request view used by handlers.
type CurrentSession struct {
	Session     *domain.Session
	User        *domain.User
	Roles       []domain.Role
	MFAEnrolled bool
}

// SessionFromContext returns the session placed by SessionAuth.
func SessionFromContext(ctx context.Context) *CurrentSession {
	current, _ := ctx.Value(currentSessionKey{}).(*CurrentSession)
	return current
}

func isAdmin(roles []domain.Role) bool {
	for _, role := range roles {
		if role.Name == domain.PlatformAdminRole {
			return true
		}
	}
	return false
}

func adminMFAReady(current *CurrentSession) bool {
	return current != nil && current.Session != nil &&
		current.MFAEnrolled &&
		current.Session.ACR == domain.ACRMFA
}

// MFASatisfied blocks an admin whose session is still password-only from
// reaching self-service routes. Non-admin sessions pass through unchanged.
func MFASatisfied(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := SessionFromContext(r.Context())
			if current == nil {
				WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
					"session is missing or expired")
				return
			}
			if isAdmin(current.Roles) && !adminMFAReady(current) {
				denyAdminAccess(w, r, deps, current, "mfa_enrollment_required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnly enforces the platform-admin role and a completed MFA session.
func AdminOnly(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := SessionFromContext(r.Context())
			if current == nil {
				WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
					"session is missing or expired")
				return
			}
			if !isAdmin(current.Roles) {
				denyAdminAccess(w, r, deps, current, "role_required")
				return
			}
			if !adminMFAReady(current) {
				denyAdminAccess(w, r, deps, current, "mfa_required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func denyAdminAccess(w http.ResponseWriter, r *http.Request, deps Deps, current *CurrentSession, reason string) {
	if deps.Audit != nil {
		audit := auditEvent(current.User.RealmID, domain.ActorTypeUser,
			userActor(current.User.ID), domain.AuditAdminAccessDenied,
			map[string]any{"route": r.URL.Path, "reason": reason})
		if err := deps.Audit.Insert(r.Context(), audit); err != nil {
			logger.FromContext(r.Context()).Warn("audit_insert_failed", "error", err)
		}
	}
	code := domain.CodeForbidden
	if reason == "mfa_required" || reason == "mfa_enrollment_required" {
		code = domain.CodeMFARequired
	}
	WriteProblem(w, r, http.StatusForbidden, code, "administrator access requires a completed MFA session")
}

// RequestID propagates or creates X-Request-ID and puts it in the request
// context before any logger runs.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(logger.WithRequestID(r.Context(), id)))
	})
}

func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// RealIP honors X-Forwarded-For and X-Real-IP only when the direct peer is in
// TRUSTED_PROXY_CIDRS. Without that check the headers are attacker controlled.
func RealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !peerTrusted(r.RemoteAddr, trusted) {
				next.ServeHTTP(w, r)
				return
			}
			client := forwardedClient(r)
			if client == "" {
				next.ServeHTTP(w, r)
				return
			}
			r.RemoteAddr = net.JoinHostPort(client, "0")
			next.ServeHTTP(w, r)
		})
	}
}

func peerTrusted(remoteAddr string, trusted []netip.Prefix) bool {
	if len(trusted) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func forwardedClient(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			part = strings.TrimSpace(part)
			if addr, err := netip.ParseAddr(part); err == nil {
				return addr.Unmap().String()
			}
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if addr, err := netip.ParseAddr(realIP); err == nil {
			return addr.Unmap().String()
		}
	}
	return ""
}

// Recoverer turns a panic into a 500 problem+json and logs the stack.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}
			logger.FromContext(r.Context()).Error("panic recovered",
				"error", recovered,
				"stack", string(debug.Stack()),
			)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders applies the baseline header set from ARCHITECTURE §10.
func SecurityHeaders(production bool) func(http.Handler) http.Handler {
	const csp = "default-src 'self'; script-src 'self'; style-src 'self'; " +
		"img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; " +
		"base-uri 'none'; form-action 'self'"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Set("Content-Security-Policy", csp)
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("X-Frame-Options", "DENY")
			header.Set("Referrer-Policy", "no-referrer")
			if production {
				header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimit allows 64 KiB for form posts and 1 MiB for JSON and everything
// else.
func BodyLimit(next http.Handler) http.Handler {
	const (
		jsonLimit = 1 << 20
		formLimit = 64 << 10
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(jsonLimit)
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			limit = formLimit
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// Logger writes one JSON access line per request. The request state holder lets
// SessionAuth add user_id after this middleware starts the timer.
func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			state := logger.NewRequestState()
			ctx := logger.WithLogger(r.Context(), base)
			ctx = logger.WithRequestState(ctx, state)
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r.WithContext(ctx))

			route := r.URL.Path
			logger.FromContext(ctx).Info("http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("route", route),
				slog.Int("status", recorder.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// SessionAuth reads the session cookie, validates it against Postgres, and
// populates the user_id used by the access log. It never writes the user_id
// for a rejected session.
func SessionAuth(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if deps.Sessions == nil {
				WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
					"session storage is unavailable")
				return
			}
			cookie, err := r.Cookie(deps.Config.SessionCookieName())
			if err != nil || cookie.Value == "" {
				WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
					"session is missing or expired")
				return
			}
			current, err := deps.Sessions.GetByTokenHash(r.Context(), pkg.SHA256Hex(cookie.Value))
			if errors.Is(err, domain.ErrNotFound) {
				WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
					"session is missing or expired")
				return
			}
			if err != nil {
				logger.FromContext(r.Context()).Error("session_lookup_failed", "error", err)
				WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
					"the server could not complete the request")
				return
			}

			now := time.Now().UTC()
			session := current.Session
			if session.RevokedAt != nil ||
				current.User.DisabledAt != nil ||
				!session.IdleExpiresAt.After(now) ||
				!session.AbsoluteExpiresAt.After(now) {
				WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
					"session is missing or expired")
				return
			}

			admin := isAdmin(current.Roles)
			idleTTL := session.IdleExpiresAt.Sub(session.LastSeenAt)
			if idleTTL <= 0 {
				idleTTL = sessionIdleTTL(admin, session.RememberMe)
			}
			idleExpiresAt := now.Add(idleTTL)
			if idleExpiresAt.After(session.AbsoluteExpiresAt) {
				idleExpiresAt = session.AbsoluteExpiresAt
			}
			if err := deps.Sessions.TouchLastSeen(r.Context(), session.ID, now, idleExpiresAt); err != nil {
				logger.FromContext(r.Context()).Warn("session_touch_failed", "error", err)
			}

			ctx := context.WithValue(r.Context(), currentSessionKey{}, &CurrentSession{
				Session:     session,
				User:        current.User,
				Roles:       current.Roles,
				MFAEnrolled: current.MFAEnrolled,
			})
			logger.SetUserID(ctx, string(current.User.ID))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// preSessionCSRF verifies the double-submit token and its Redis digest.
func preSessionCSRF(store csrf.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
					"csrf storage is unavailable")
				return
			}
			header := r.Header.Get("X-CSRF-Token")
			cookie, err := r.Cookie(csrfCookieName)
			if err != nil || !csrf.Equal(header, cookie.Value) {
				WriteProblem(w, r, http.StatusForbidden, domain.CodeCSRFInvalid,
					"csrf token is missing or invalid")
				return
			}
			ok, err := csrf.Verify(r.Context(), store, header)
			if err != nil {
				logger.FromContext(r.Context()).Error("csrf_verify_failed", "error", err)
				WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
					"csrf storage is unavailable")
				return
			}
			if !ok {
				WriteProblem(w, r, http.StatusForbidden, domain.CodeCSRFInvalid,
					"csrf token is missing or invalid")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// sessionCSRF binds the header token to the current session row.
func sessionCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := SessionFromContext(r.Context())
		if current == nil || current.Session == nil {
			WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
				"session is missing or expired")
			return
		}
		header := r.Header.Get("X-CSRF-Token")
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || !csrf.Equal(header, cookie.Value) ||
			!csrf.MatchesHash(header, current.Session.CSRFTokenHash) {
			WriteProblem(w, r, http.StatusForbidden, domain.CodeCSRFInvalid,
				"csrf token is missing or invalid")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed rejects state-changing auth requests whose Origin/Referer is
// absent or not configured.
func originAllowed(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			origin := requestOrigin(r)
			if origin == "" {
				WriteProblem(w, r, http.StatusForbidden, domain.CodeForbidden,
					"origin or referer is required")
				return
			}
			if !cfg.OriginAllowed(origin) {
				WriteProblem(w, r, http.StatusForbidden, domain.CodeForbidden,
					"origin is not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestOrigin(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return origin
	}
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// RealmResolver extracts {realm} from /realms/{realm}/... and falls back to the
// default realm for /api/v1 routes.
func RealmResolver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realm := domain.DefaultRealmName
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) > 1 && parts[0] == "realms" && parts[1] != "" {
			realm = parts[1]
		}
		ctx := context.WithValue(r.Context(), realmContextKey{}, realm)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
