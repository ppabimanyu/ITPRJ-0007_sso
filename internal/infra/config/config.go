// Package config loads and validates the service environment.
package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config is the fully validated runtime configuration.
type Config struct {
	AppEnv     string
	HTTP       HTTPConfig
	Log        LogConfig
	Metrics    MetricsConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	MasterKeys MasterKeyConfig
	Proxies    ProxyConfig
}

type HTTPConfig struct {
	Addr        string
	ExternalURL string
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

// Load reads configuration from the process environment.
func Load() (*Config, error) {
	return Parse(os.Getenv)
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
		Proxies: ProxyConfig{TrustedCIDRs: proxies},
	}, nil
}

func (c *Config) IsProduction() bool { return c.AppEnv == "production" }
func (c *Config) IsDev() bool        { return c.AppEnv == "dev" }

func valueOr(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
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
