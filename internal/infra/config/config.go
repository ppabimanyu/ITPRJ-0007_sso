// Package config loads and validates the service environment.
package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config is the fully validated runtime configuration.
type Config struct {
	AppEnv         string
	HTTP           HTTPConfig
	Log            LogConfig
	Metrics        MetricsConfig
	Database       DatabaseConfig
	Redis          RedisConfig
	MasterKeys     MasterKeyConfig
	Proxies        ProxyConfig
	AllowedOrigins []string
	Password       PasswordConfig
	RateLimit      RateLimitConfig
	Argon2         Argon2Config
	SMTP           SMTPConfig
}

type HTTPConfig struct {
	Addr        string
	ExternalURL string
}

type PasswordConfig struct {
	MinLength int
}

// RateLimitConfig carries the deployment-tunable counters.
type RateLimitConfig struct {
	LoginIPLimit      int
	LoginAccountLimit int
	AuthIPLimit       int
	APIIPLimit        int
	OIDCIPLimit       int
	TokenClientLimit  int
}

// Argon2Config bounds concurrent password hashing.
type Argon2Config struct {
	Concurrency int
}

type SMTPConfig struct {
	Host     string
	Port     string
	From     string
	Username string
	Password string
	TLS      bool
}

type LogConfig struct {
	Level string
}

type MetricsConfig struct {
	Addr string
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type MasterKeyConfig struct {
	Keys  map[string][]byte
	KeyID string
}

type ProxyConfig struct {
	TrustedCIDRs []netip.Prefix
}

// Load reads configuration from the process environment. A local .env file is
// loaded first for developer convenience; existing environment variables win.
func Load() (*Config, error) {
	loadDotEnv(".env")
	return Parse(os.Getenv)
}

// loadDotEnv reads simple KEY=value lines. It never overrides a variable that
// is already set, and it silently ignores a missing file.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

// Parse builds a Config from a lookup function so tests can avoid the process
// environment. Every required value is validated here, before any listener or
// database pool opens.
func Parse(getenv func(string) string) (*Config, error) {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	appEnv := valueOr(getenv, "APP_ENV", "dev")
	switch appEnv {
	case "dev", "staging", "production":
	default:
		fail("APP_ENV must be dev, staging, or production, got %q", appEnv)
	}

	httpAddr := valueOr(getenv, "HTTP_ADDR", ":8080")
	if err := validateAddr(httpAddr); err != nil {
		fail("HTTP_ADDR: %v", err)
	}

	metricsAddr := valueOr(getenv, "METRICS_ADDR", ":9090")
	if err := validateAddr(metricsAddr); err != nil {
		fail("METRICS_ADDR: %v", err)
	}

	logLevel := strings.ToLower(valueOr(getenv, "LOG_LEVEL", "info"))
	switch logLevel {
	case "debug", "info", "warn", "error":
	default:
		fail("LOG_LEVEL must be debug, info, warn, or error, got %q", logLevel)
	}

	externalURL := strings.TrimRight(getenv("EXTERNAL_URL"), "/")
	if externalURL == "" {
		fail("EXTERNAL_URL is required")
	} else if err := validateExternalURL(externalURL, appEnv); err != nil {
		fail("EXTERNAL_URL: %v", err)
	}

	databaseURL := getenv("DATABASE_URL")
	if databaseURL == "" {
		fail("DATABASE_URL is required")
	} else if err := validateURL(databaseURL, "postgres", "postgresql"); err != nil {
		fail("DATABASE_URL: %v", err)
	}

	redisURL := getenv("REDIS_URL")
	if redisURL == "" {
		fail("REDIS_URL is required")
	} else if err := validateURL(redisURL, "redis", "rediss"); err != nil {
		fail("REDIS_URL: %v", err)
	}

	keys, keyErr := parseMasterKeys(getenv("SSO_MASTER_KEYS"))
	if keyErr != nil {
		fail("SSO_MASTER_KEYS: %v", keyErr)
	}
	keyID := getenv("SSO_MASTER_KEY_ID")
	if keyID == "" {
		fail("SSO_MASTER_KEY_ID is required")
	} else if _, ok := keys[keyID]; !ok {
		fail("SSO_MASTER_KEY_ID %q is not present in SSO_MASTER_KEYS", keyID)
	}

	proxies, proxyErr := parseCIDRs(getenv("TRUSTED_PROXY_CIDRS"))
	if proxyErr != nil {
		fail("TRUSTED_PROXY_CIDRS: %v", proxyErr)
	}

	origins, originErr := parseOrigins(getenv("ALLOWED_ORIGINS"), externalURL, appEnv)
	if originErr != nil {
		fail("ALLOWED_ORIGINS: %v", originErr)
	}

	passwordMin, passwordErr := parsePasswordMin(getenv("PASSWORD_MIN_LENGTH"))
	if passwordErr != nil {
		fail("PASSWORD_MIN_LENGTH: %v", passwordErr)
	}

	loginIPLimit, err := parseLimit(getenv, "AUTH_LOGIN_IP_LIMIT", 10)
	if err != nil {
		fail("%v", err)
	}
	loginAccountLimit, err := parseLimit(getenv, "AUTH_LOGIN_ACCOUNT_LIMIT", 5)
	if err != nil {
		fail("%v", err)
	}
	authIPLimit, err := parseLimit(getenv, "AUTH_IP_LIMIT", 120)
	if err != nil {
		fail("%v", err)
	}
	apiIPLimit, err := parseLimit(getenv, "API_IP_LIMIT", 300)
	if err != nil {
		fail("%v", err)
	}
	oidcIPLimit, err := parseLimit(getenv, "OIDC_IP_LIMIT", 120)
	if err != nil {
		fail("%v", err)
	}
	tokenClientLimit, err := parseLimit(getenv, "TOKEN_CLIENT_LIMIT", 60)
	if err != nil {
		fail("%v", err)
	}

	argon2Concurrency := 2
	if raw := strings.TrimSpace(getenv("ARGON2_CONCURRENCY")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 64 {
			fail("ARGON2_CONCURRENCY must be between 1 and 64")
		} else {
			argon2Concurrency = parsed
		}
	}

	smtpHost := valueOr(getenv, "SMTP_HOST", "localhost")
	smtpPort := valueOr(getenv, "SMTP_PORT", "1025")
	if err := validatePort(smtpPort); err != nil {
		fail("SMTP_PORT: %v", err)
	}
	smtpFrom := valueOr(getenv, "SMTP_FROM", "no-reply@example.com")
	if _, err := mail.ParseAddress(smtpFrom); err != nil {
		fail("SMTP_FROM: %v", err)
	}
	smtpTLS, err := parseBool(getenv("SMTP_TLS"))
	if err != nil {
		fail("SMTP_TLS: %v", err)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &Config{
		AppEnv:  appEnv,
		HTTP:    HTTPConfig{Addr: httpAddr, ExternalURL: externalURL},
		Log:     LogConfig{Level: logLevel},
		Metrics: MetricsConfig{Addr: metricsAddr},
		Database: DatabaseConfig{
			URL: databaseURL,
		},
		Redis: RedisConfig{URL: redisURL},
		MasterKeys: MasterKeyConfig{
			Keys:  keys,
			KeyID: keyID,
		},
		Proxies:        ProxyConfig{TrustedCIDRs: proxies},
		AllowedOrigins: origins,
		Password:       PasswordConfig{MinLength: passwordMin},
		RateLimit: RateLimitConfig{
			LoginIPLimit:      loginIPLimit,
			LoginAccountLimit: loginAccountLimit,
			AuthIPLimit:       authIPLimit,
			APIIPLimit:        apiIPLimit,
			OIDCIPLimit:       oidcIPLimit,
			TokenClientLimit:  tokenClientLimit,
		},
		Argon2: Argon2Config{Concurrency: argon2Concurrency},
		SMTP: SMTPConfig{
			Host:     smtpHost,
			Port:     smtpPort,
			From:     smtpFrom,
			Username: getenv("SMTP_USERNAME"),
			Password: getenv("SMTP_PASSWORD"),
			TLS:      smtpTLS,
		},
	}, nil
}

func (c *Config) IsProduction() bool { return c.AppEnv == "production" }
func (c *Config) IsDev() bool        { return c.AppEnv == "dev" }

// OriginAllowed reports an exact scheme and host match.
func (c *Config) OriginAllowed(origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return false
	}
	for _, allowed := range c.AllowedOrigins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}
	return false
}

// SessionCookieName uses the __Host- prefix outside dev.
func (c *Config) SessionCookieName() string {
	if c.IsDev() {
		return "sso_session"
	}
	return "__Host-sso_session"
}

// SessionCookieSecure is false only for the HTTP dev stack.
func (c *Config) SessionCookieSecure() bool { return !c.IsDev() }

func valueOr(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLimit(getenv func(string) string, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 1 || parsed > 1_000_000 {
		return 0, fmt.Errorf("%s must be between 1 and 1000000", name)
	}
	return parsed, nil
}

func parseOrigins(raw, externalURL, appEnv string) ([]string, error) {
	candidates := []string{}
	if strings.TrimSpace(raw) != "" {
		candidates = append(candidates, strings.Split(raw, ",")...)
	} else {
		candidates = append(candidates, externalURL)
		if appEnv == "dev" {
			candidates = append(candidates, "http://localhost:5173")
		}
	}
	origins := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		origin, err := normalizeOrigin(candidate)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return nil, errors.New("at least one origin is required")
	}
	return origins, nil
}

func normalizeOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid origin %q: %w", raw, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("origin %q must be an absolute http or https URL", raw)
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("origin %q must not contain a path", raw)
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

func parsePasswordMin(raw string) (int, error) {
	if raw == "" {
		return 12, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("must be an integer: %w", err)
	}
	if n < 12 || n > 64 {
		return 0, fmt.Errorf("must be between 12 and 64, got %d", n)
	}
	return n, nil
}

func validatePort(raw string) error {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("invalid port %q", raw)
	}
	return nil
}

func parseBool(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

func validateAddr(addr string) error {
	if addr == "" {
		return errors.New("must not be empty")
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid host:port: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("invalid port %q", port)
	}
	return nil
}

func validateExternalURL(raw, appEnv string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("host must not be empty")
	}
	if appEnv == "production" && u.Scheme != "https" {
		return errors.New("production requires https")
	}
	return nil
}

func validateURL(raw string, schemes ...string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	allowed := false
	for _, s := range schemes {
		if u.Scheme == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("scheme must be one of %v, got %q", schemes, u.Scheme)
	}
	if u.Host == "" {
		return errors.New("host must not be empty")
	}
	return nil
}

func parseMasterKeys(raw string) (map[string][]byte, error) {
	if raw == "" {
		return nil, errors.New("is required")
	}
	var encoded map[string]string
	if err := json.Unmarshal([]byte(raw), &encoded); err != nil {
		return nil, fmt.Errorf("must be a JSON object of id to base64 key: %w", err)
	}
	if len(encoded) == 0 {
		return nil, errors.New("must contain at least one key")
	}
	keys := make(map[string][]byte, len(encoded))
	for id, value := range encoded {
		if id == "" {
			return nil, errors.New("key id must not be empty")
		}
		key, err := decodeBase64Key(value)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", id, err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("key %q: must decode to 32 bytes, got %d", id, len(key))
		}
		keys[id] = key
	}
	return keys, nil
}

func decodeBase64Key(value string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return key, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}

func parseCIDRs(raw string) ([]netip.Prefix, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", part, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
