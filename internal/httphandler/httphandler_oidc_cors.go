package httphandler

import (
	"net/http"
	"strings"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

// OIDCCORS handles preflight and response headers for the token, userinfo,
// revoke, and introspect endpoints. Preflight has no client identity, so the
// realm's registered origins are matched as a union; each handler still checks
// the authenticated client's own origins.
func OIDCCORS(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			allowed, err := realmAllowsOrigin(r, deps, origin)
			if err != nil {
				WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
					"client origins are unavailable")
				return
			}
			if !allowed {
				WriteProblem(w, r, http.StatusForbidden, domain.CodeForbidden,
					"origin is not registered for any client in this realm")
				return
			}
			header := w.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Vary", "Origin")
			header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			header.Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func realmAllowsOrigin(r *http.Request, deps Deps, origin string) (bool, error) {
	if deps.Realms == nil || deps.Clients == nil {
		return false, nil
	}
	realm, err := deps.Realms.GetByName(r.Context(), RealmFromContext(r.Context()))
	if err != nil {
		return false, nil
	}
	clients, err := deps.Clients.ListByRealm(r.Context(), realm.ID)
	if err != nil {
		return false, err
	}
	for i := range clients {
		origins, err := deps.Clients.CORSOrigins(r.Context(), clients[i].ID)
		if err != nil {
			return false, err
		}
		for _, allowed := range origins {
			if strings.EqualFold(allowed, origin) {
				return true, nil
			}
		}
	}
	return false, nil
}

func preflight(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
