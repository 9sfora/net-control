package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/netcontrol")
	t.Setenv("HTTP_ADDRESS", ":9090")
	t.Setenv("WORKER_INTERVAL", "2s")
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret-with-at-least-32-characters")
	t.Setenv("APPLICATION_FQDN", "net-control.example.com")
	t.Setenv("ACS_FQDN", "acs.example.com")
	t.Setenv("ACS_DEFAULT_TENANT_SLUG", "netcontrol")
	t.Setenv("ACS_COOKIE_SECURE", "false")
	t.Setenv("WIFI_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddress != ":9090" || cfg.WorkerInterval != 2*time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.ACSCookieSecure {
		t.Fatal("ACS cookie should not be secure in this test")
	}
}

func TestLoadRejectsInvalidACSCookieSecure(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/netcontrol")
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret-with-at-least-32-characters")
	t.Setenv("APPLICATION_FQDN", "net-control.example.com")
	t.Setenv("ACS_FQDN", "acs.example.com")
	t.Setenv("ACS_DEFAULT_TENANT_SLUG", "netcontrol")
	t.Setenv("ACS_COOKIE_SECURE", "sometimes")
	t.Setenv("WIFI_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret-with-at-least-32-characters")
	t.Setenv("APPLICATION_FQDN", "net-control.example.com")
	t.Setenv("ACS_FQDN", "acs.example.com")
	t.Setenv("ACS_DEFAULT_TENANT_SLUG", "netcontrol")
	t.Setenv("WIFI_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}
