package httphandler

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

type realmContextKey struct{}

// RealmFromContext returns the realm selected by RealmResolver.
func RealmFromContext(ctx context.Context) string {
	realm, _ := ctx.Value(realmContextKey{}).(string)
	return realm
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

// Logger writes one JSON access line per request. Query strings are never
// logged.
func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := logger.WithLogger(r.Context(), base)
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

// RateLimit is the M1 seam. M2 replaces it with the Redis counter.
func RateLimit(next http.Handler) http.Handler {
	return next
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
