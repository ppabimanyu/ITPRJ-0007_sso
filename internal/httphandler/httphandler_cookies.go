package httphandler

import (
	"net/http"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/infra/config"
)

const csrfCookieName = "sso_csrf"

func setSessionCookie(w http.ResponseWriter, cfg *config.Config, raw string, remember bool, absolute time.Time) {
	cookie := &http.Cookie{
		Name:     cfg.SessionCookieName(),
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.SessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	}
	if remember {
		cookie.Expires = absolute
		cookie.MaxAge = int(time.Until(absolute).Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.SessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
}

func setCSRFCookie(w http.ResponseWriter, cfg *config.Config, raw string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     csrfCookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: false,
		Secure:   cfg.SessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	}
	if !expires.IsZero() {
		cookie.Expires = expires
		cookie.MaxAge = int(time.Until(expires).Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearCSRFCookie(w http.ResponseWriter, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   cfg.SessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
}
