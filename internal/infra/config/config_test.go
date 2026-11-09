package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":             "dev",
		"HTTP_ADDR":           ":8080",
		"EXTERNAL_URL":        "http://localhost:8080",
		"DATABASE_URL":        "postgres://sso:secret@localhost:5432/sso?sslmode=disable",
		"REDIS_URL":           "redis://localhost:6379/0",
		"SSO_MASTER_KEYS":     `{"dev":"` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `"}`,
		"SSO_MASTER_KEY_ID":   "dev",
		"TRUSTED_PROXY_CIDRS": "10.0.0.0/8, 192.168.0.0/16",
		"LOG_LEVEL":           "info",
		"METRICS_ADDR":        ":9090",
	}
}

func lookup(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestParseValid(t *testing.T) {
	cfg, err := Parse(lookup(validEnv()))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.MasterKeys.KeyID != "dev" || len(cfg.MasterKeys.Keys["dev"]) != 32 {
		t.Errorf("master keyring not loaded: %+v", cfg.MasterKeys)
	}
	if len(cfg.Proxies.TrustedCIDRs) != 2 {
		t.Errorf("trusted CIDRs = %v", cfg.Proxies.TrustedCIDRs)
	}
	if !cfg.IsDev() || cfg.IsProduction() {
		t.Errorf("environment helpers wrong for %q", cfg.AppEnv)
	}
}

func TestParseDefaults(t *testing.T) {
	env := validEnv()
	delete(env, "HTTP_ADDR")
	delete(env, "LOG_LEVEL")
	delete(env, "METRICS_ADDR")
	delete(env, "APP_ENV")
	delete(env, "TRUSTED_PROXY_CIDRS")

	cfg, err := Parse(lookup(env))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.HTTP.Addr != ":8080" || cfg.Log.Level != "info" || cfg.Metrics.Addr != ":9090" {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	if cfg.AppEnv != "dev" {
		t.Errorf("AppEnv = %q", cfg.AppEnv)
	}
	if cfg.Proxies.TrustedCIDRs != nil {
		t.Errorf("TrustedCIDRs = %v, want nil", cfg.Proxies.TrustedCIDRs)
	}
}

func TestParseMissingRequired(t *testing.T) {
	for _, key := range []string{
		"EXTERNAL_URL",
		"DATABASE_URL",
		"REDIS_URL",
		"SSO_MASTER_KEYS",
		"SSO_MASTER_KEY_ID",
	} {
		t.Run(key, func(t *testing.T) {
			env := validEnv()
			delete(env, key)
			_, err := Parse(lookup(env))
			if err == nil {
				t.Fatalf("Parse() with missing %s succeeded", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error %q does not name %s", err, key)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	tests := map[string]struct {
		key   string
		value string
		want  string
	}{
		"app env":          {"APP_ENV", "qa", "APP_ENV"},
		"http addr":        {"HTTP_ADDR", "8080", "HTTP_ADDR"},
		"metrics addr":     {"METRICS_ADDR", "nope", "METRICS_ADDR"},
		"log level":        {"LOG_LEVEL", "verbose", "LOG_LEVEL"},
		"external url":     {"EXTERNAL_URL", "://bad", "EXTERNAL_URL"},
		"external scheme":  {"EXTERNAL_URL", "ftp://sso.example.com", "EXTERNAL_URL"},
		"database scheme":  {"DATABASE_URL", "mysql://localhost/sso", "DATABASE_URL"},
		"redis scheme":     {"REDIS_URL", "http://localhost:6379", "REDIS_URL"},
		"proxy cidr":       {"TRUSTED_PROXY_CIDRS", "10.0.0.0", "TRUSTED_PROXY_CIDRS"},
		"master keys json": {"SSO_MASTER_KEYS", "{not-json", "SSO_MASTER_KEYS"},
		"master key size":  {"SSO_MASTER_KEYS", `{"dev":"` + base64.StdEncoding.EncodeToString([]byte("short")) + `"}`, "32 bytes"},
		"unknown key id":   {"SSO_MASTER_KEY_ID", "other", "SSO_MASTER_KEY_ID"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env[tt.key] = tt.value
			_, err := Parse(lookup(env))
			if err == nil {
				t.Fatalf("Parse() with invalid %s succeeded", tt.key)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestParseAuthDefaults(t *testing.T) {
	cfg, err := Parse(lookup(validEnv()))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Password.MinLength != 12 {
		t.Errorf("Password.MinLength = %d, want 12", cfg.Password.MinLength)
	}
	if !cfg.OriginAllowed("http://localhost:8080") {
		t.Errorf("EXTERNAL_URL origin is not allowed: %v", cfg.AllowedOrigins)
	}
	if !cfg.OriginAllowed("http://localhost:5173") {
		t.Errorf("dev SPA origin is not allowed: %v", cfg.AllowedOrigins)
	}
	if cfg.SessionCookieName() != "sso_session" {
		t.Errorf("SessionCookieName() = %q", cfg.SessionCookieName())
	}
	if cfg.SMTP.Host != "localhost" || cfg.SMTP.Port != "1025" ||
		cfg.SMTP.From != "no-reply@example.com" || cfg.SMTP.TLS {
		t.Errorf("SMTP defaults = %+v", cfg.SMTP)
	}
}

func TestSessionCookieProductionPrefix(t *testing.T) {
	cfg := &Config{AppEnv: "production"}
	if got := cfg.SessionCookieName(); got != "__Host-sso_session" {
		t.Fatalf("SessionCookieName() = %q, want __Host-sso_session", got)
	}
	if !cfg.SessionCookieSecure() {
		t.Fatal("production session cookie must be Secure")
	}
}

func TestParseAuthOverrides(t *testing.T) {
	env := validEnv()
	env["ALLOWED_ORIGINS"] = "https://app.example.com, https://admin.example.com"
	env["PASSWORD_MIN_LENGTH"] = "16"
	env["SMTP_HOST"] = "smtp.example.com"
	env["SMTP_PORT"] = "587"
	env["SMTP_FROM"] = "sso@example.com"
	env["SMTP_USERNAME"] = "sso"
	env["SMTP_PASSWORD"] = "not-a-real-secret"
	env["SMTP_TLS"] = "true"

	cfg, err := Parse(lookup(env))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(cfg.AllowedOrigins) != 2 || !cfg.OriginAllowed("https://app.example.com") {
		t.Errorf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
	if cfg.OriginAllowed("http://localhost:5173") {
		t.Error("explicit ALLOWED_ORIGINS still allowed the dev SPA origin")
	}
	if cfg.Password.MinLength != 16 {
		t.Errorf("Password.MinLength = %d", cfg.Password.MinLength)
	}
	if cfg.SMTP.Host != "smtp.example.com" || cfg.SMTP.Port != "587" ||
		cfg.SMTP.From != "sso@example.com" || !cfg.SMTP.TLS || cfg.SMTP.Username != "sso" {
		t.Errorf("SMTP = %+v", cfg.SMTP)
	}
}

func TestParseAuthInvalid(t *testing.T) {
	tests := map[string]struct {
		key   string
		value string
		want  string
	}{
		"origin path":   {"ALLOWED_ORIGINS", "https://app.example.com/callback", "ALLOWED_ORIGINS"},
		"origin scheme": {"ALLOWED_ORIGINS", "ftp://app.example.com", "ALLOWED_ORIGINS"},
		"password min":  {"PASSWORD_MIN_LENGTH", "8", "PASSWORD_MIN_LENGTH"},
		"smtp port":     {"SMTP_PORT", "0", "SMTP_PORT"},
		"smtp from":     {"SMTP_FROM", "not-an-address", "SMTP_FROM"},
		"smtp tls":      {"SMTP_TLS", "sometimes", "SMTP_TLS"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env[tt.key] = tt.value
			_, err := Parse(lookup(env))
			if err == nil {
				t.Fatalf("Parse() with invalid %s succeeded", tt.key)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestProductionRequiresHTTPS(t *testing.T) {
	env := validEnv()
	env["APP_ENV"] = "production"
	env["EXTERNAL_URL"] = "http://sso.example.com"
	if _, err := Parse(lookup(env)); err == nil {
		t.Fatal("production with http EXTERNAL_URL succeeded")
	}
}
