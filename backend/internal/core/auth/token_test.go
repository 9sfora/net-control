package auth

import (
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	manager := NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	token, err := manager.Issue(Principal{UserID: "user", TenantID: "tenant", Permissions: []string{"acs.router.read"}})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	principal, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if principal.UserID != "user" || principal.ExpiresAt != 1060 {
		t.Fatalf("unexpected principal: %+v", principal)
	}
}

func TestTokenRejectsTamperingAndExpiration(t *testing.T) {
	manager := NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	now := time.Unix(1000, 0)
	manager.now = func() time.Time { return now }
	token, _ := manager.Issue(Principal{UserID: "user", TenantID: "tenant"})

	if _, err := manager.Parse(token + "x"); err == nil {
		t.Fatal("Parse() accepted a tampered token")
	}
	now = now.Add(2 * time.Minute)
	if _, err := manager.Parse(token); err == nil {
		t.Fatal("Parse() accepted an expired token")
	}
}
