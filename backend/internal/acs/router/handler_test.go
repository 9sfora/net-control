package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
)

type fakeStore struct {
	tenantID string
	name     string
}

func (f *fakeStore) ListUnknown(_ context.Context, tenantID string, _, _ int) ([]Unknown, error) {
	f.tenantID = tenantID
	return []Unknown{{ID: "01945c44-d753-7d69-9d2c-1809de612345", SerialNumber: "SN1"}}, nil
}

func (f *fakeStore) Configure(_ context.Context, tenantID, _ string, input ConfigureUnknownInput) (Router, error) {
	f.tenantID, f.name = tenantID, input.Name
	return Router{ID: "01945c44-d753-7d69-9d2c-1809de612346", Name: input.Name}, nil
}

func TestListUnknownUsesAuthenticatedTenant(t *testing.T) {
	store := &fakeStore{}
	tokens := auth.NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	token, _ := tokens.Issue(auth.Principal{UserID: "user", TenantID: "tenant", Permissions: []string{"acs.unknown_router.read"}})
	handler := NewHandler(store, tokens)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/acs/unknown-routers", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK || store.tenantID != "tenant" {
		t.Fatalf("status = %d, tenant = %q", response.Code, store.tenantID)
	}
}

func TestConfigureUnknown(t *testing.T) {
	store := &fakeStore{}
	tokens := auth.NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	token, _ := tokens.Issue(auth.Principal{UserID: "user", TenantID: "tenant", Permissions: []string{"acs.unknown_router.configure"}})
	handler := NewHandler(store, tokens)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/acs/unknown-routers/01945c44-d753-7d69-9d2c-1809de612345/configure", strings.NewReader(`{"name":"Customer router","router_model_id":"01945c44-d753-7d69-9d2c-1809de612344"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || store.name != "Customer router" {
		t.Fatalf("status = %d, name = %q, body = %s", response.Code, store.name, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
}
