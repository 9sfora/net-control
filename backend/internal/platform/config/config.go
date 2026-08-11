package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddress     string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	WorkerInterval  time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:     value("APP_ENV", "development"),
		HTTPAddress:     value("HTTP_ADDRESS", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		ShutdownTimeout: duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		WorkerInterval:  duration("WORKER_INTERVAL", time.Second),
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

	return cfg, nil
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
