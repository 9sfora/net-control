package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment          string
	HTTPAddress          string
	DatabaseURL          string
	ShutdownTimeout      time.Duration
	WorkerInterval       time.Duration
	AuthTokenSecret      string
	AuthTokenTTL         time.Duration
	ApplicationFQDN      string
	ACSFQDN              string
	ACSDefaultTenantSlug string
	WiFiEncryptionKey    string
}

func Load() (Config, error) {
	cfg := Config{
		Environment:          value("APP_ENV", "development"),
		HTTPAddress:          value("HTTP_ADDRESS", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		ShutdownTimeout:      duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		WorkerInterval:       duration("WORKER_INTERVAL", time.Second),
		AuthTokenSecret:      os.Getenv("AUTH_TOKEN_SECRET"),
		AuthTokenTTL:         duration("AUTH_TOKEN_TTL", 15*time.Minute),
		ApplicationFQDN:      strings.ToLower(strings.TrimSpace(os.Getenv("APPLICATION_FQDN"))),
		ACSFQDN:              strings.ToLower(strings.TrimSpace(os.Getenv("ACS_FQDN"))),
		ACSDefaultTenantSlug: strings.ToLower(strings.TrimSpace(os.Getenv("ACS_DEFAULT_TENANT_SLUG"))),
		WiFiEncryptionKey:    os.Getenv("WIFI_ENCRYPTION_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, errors.New("SHUTDOWN_TIMEOUT must be positive")
	}
	if cfg.WorkerInterval <= 0 {
		return Config{}, errors.New("WORKER_INTERVAL must be positive")
	}
	if len(cfg.AuthTokenSecret) < 32 {
		return Config{}, errors.New("AUTH_TOKEN_SECRET must contain at least 32 characters")
	}
	if cfg.AuthTokenTTL <= 0 {
		return Config{}, errors.New("AUTH_TOKEN_TTL must be positive")
	}
	if !validFQDN(cfg.ApplicationFQDN) {
		return Config{}, errors.New("APPLICATION_FQDN must be a valid hostname without scheme or path")
	}
	if !validFQDN(cfg.ACSFQDN) {
		return Config{}, errors.New("ACS_FQDN must be a valid hostname without scheme or path")
	}
	if cfg.ApplicationFQDN == cfg.ACSFQDN {
		return Config{}, errors.New("APPLICATION_FQDN and ACS_FQDN must be different")
	}
	if !validSlug(cfg.ACSDefaultTenantSlug) {
		return Config{}, errors.New("ACS_DEFAULT_TENANT_SLUG must contain lowercase letters, numbers and hyphens")
	}
	if key, err := base64.StdEncoding.DecodeString(cfg.WiFiEncryptionKey); err != nil || len(key) != 32 {
		return Config{}, errors.New("WIFI_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}

	return cfg, nil
}

func validSlug(value string) bool {
	if value == "" || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

func validFQDN(host string) bool {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/:@ ") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(raw); err == nil {
		return parsed
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return -1
	}
	return time.Duration(seconds) * time.Second
}

func (c Config) String() string {
	return fmt.Sprintf("environment=%s http_address=%s", c.Environment, c.HTTPAddress)
}
