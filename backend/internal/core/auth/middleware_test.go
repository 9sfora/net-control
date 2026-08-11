package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticationAndPermission(t *testing.T) {
	tokens := NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	token, _ := tokens.Issue(Principal{UserID: "user", TenantID: "tenant", Permissions: []string{"acs.router.read"}})
	called := false
	handler := Authenticate(tokens, RequirePermission("acs.router.read", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodGet, "/routers", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("status = %d, called = %v", response.Code, called)
	}
}

func TestPermissionDenied(t *testing.T) {
	tokens := NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	token, _ := tokens.Issue(Principal{UserID: "user", TenantID: "tenant"})
	handler := Authenticate(tokens, RequirePermission("system.users.manage", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	request := httptest.NewRequest(http.MethodGet, "/users", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
