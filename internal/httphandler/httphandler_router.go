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
	"github.com/ppa/itprj-0007-sso/internal/webui"
)

// Deps is the wiring passed from cmd/server. Handlers only see domain ports in
// later milestones; M1 needs the health dependencies directly.
type Deps struct {
	Config  *config.Config
	DB      *database.DB
	Redis   *redis.Client
	Logger  *slog.Logger
	Version domain.VersionInfo
}

// NewRouter builds the global middleware chain in the order fixed by
// ARCHITECTURE §4 and mounts the M1 routes.
func NewRouter(deps Deps) http.Handler {
	router := chi.NewRouter()
	router.Use(RequestID)
	router.Use(tracing.Middleware)
	router.Use(RealIP(deps.Config.Proxies.TrustedCIDRs))
	router.Use(Recoverer)
	router.Use(SecurityHeaders(deps.Config.IsProduction()))
	router.Use(BodyLimit)
	router.Use(Logger(deps.Logger))
	router.Use(RateLimit)
	router.Use(RealmResolver)

	router.Get("/healthz", healthz)
	router.Get("/readyz", readyz(deps))
	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/version", version(deps.Version))
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
