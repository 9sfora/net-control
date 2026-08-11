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
	Environment              string
	HTTPAddress              string
	DatabaseURL              string
	ShutdownTimeout          time.Duration
	WorkerInterval           time.Duration
	AuthTokenSecret          string
	AuthTokenTTL             time.Duration
	ApplicationFQDN          string
	ACSFQDN                  string
	ACSDefaultTenantSlug     string
	WiFiEncryptionKey        string
	CommandProcessingTimeout time.Duration
	CommandRetryIntervals    []time.Duration
	ACSOnlineTimeout         time.Duration
	ACSOfflineTimeout        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:              value("APP_ENV", "development"),
		HTTPAddress:              value("HTTP_ADDRESS", ":8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		ShutdownTimeout:          duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		WorkerInterval:           duration("WORKER_INTERVAL", time.Second),
		AuthTokenSecret:          os.Getenv("AUTH_TOKEN_SECRET"),
		AuthTokenTTL:             duration("AUTH_TOKEN_TTL", 15*time.Minute),
		ApplicationFQDN:          strings.ToLower(strings.TrimSpace(os.Getenv("APPLICATION_FQDN"))),
		ACSFQDN:                  strings.ToLower(strings.TrimSpace(os.Getenv("ACS_FQDN"))),
		ACSDefaultTenantSlug:     strings.ToLower(strings.TrimSpace(os.Getenv("ACS_DEFAULT_TENANT_SLUG"))),
		WiFiEncryptionKey:        os.Getenv("WIFI_ENCRYPTION_KEY"),
		CommandProcessingTimeout: duration("COMMAND_PROCESSING_TIMEOUT", 5*time.Minute),
		ACSOnlineTimeout:         duration("ACS_ONLINE_TIMEOUT", 10*time.Minute),
		ACSOfflineTimeout:        duration("ACS_OFFLINE_TIMEOUT", 24*time.Hour),
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
	if cfg.CommandProcessingTimeout <= 0 {
		return Config{}, errors.New("COMMAND_PROCESSING_TIMEOUT must be positive")
	}
	retryIntervals, err := durations("COMMAND_RETRY_INTERVALS", "30s,2m,10m")
	if err != nil {
		return Config{}, err
	}
	cfg.CommandRetryIntervals = retryIntervals
	if cfg.ACSOnlineTimeout <= 0 || cfg.ACSOfflineTimeout <= cfg.ACSOnlineTimeout {
		return Config{}, errors.New("ACS timeouts must be positive and ACS_OFFLINE_TIMEOUT must exceed ACS_ONLINE_TIMEOUT")
	}

	return cfg, nil
}

func durations(key, fallback string) ([]time.Duration, error) {
	raw := value(key, fallback)
	parts := strings.Split(raw, ",")
	result := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		parsed, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("%s must contain positive comma-separated durations", key)
		}
		result = append(result, parsed)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s must not be empty", key)
	}
	return result, nil
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
