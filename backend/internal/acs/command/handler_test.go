package command

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
)

type fakeStore struct {
	tenantID string
	canceled bool
}

func (f *fakeStore) List(_ context.Context, tenantID string, _ Filter) ([]Item, error) {
	f.tenantID = tenantID
	return []Item{{ID: "command"}}, nil
}
func (f *fakeStore) Get(context.Context, string, string) (Detail, error) { return Detail{}, nil }
func (f *fakeStore) Cancel(_ context.Context, tenantID, _, _ string) (Item, error) {
	f.tenantID = tenantID
	f.canceled = true
	return Item{Status: "CANCELED"}, nil
}

func TestListUsesTenant(t *testing.T) {
	store := &fakeStore{}
	tokens := auth.NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	token, _ := tokens.Issue(auth.Principal{UserID: "user", TenantID: "tenant", Permissions: []string{"acs.command.read"}})
	handler := NewHandler(store, tokens)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/acs/commands?status=PENDING", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.tenantID != "tenant" {
		t.Fatalf("status=%d tenant=%q", response.Code, store.tenantID)
	}
}

func TestCancelRejectsInvalidID(t *testing.T) {
	store := &fakeStore{}
	tokens := auth.NewTokenManager("test-secret-with-at-least-32-characters", time.Minute)
	token, _ := tokens.Issue(auth.Principal{UserID: "user", TenantID: "tenant", Permissions: []string{"acs.command.cancel"}})
	handler := NewHandler(store, tokens)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/acs/commands/not-a-uuid/cancel", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || store.canceled {
		t.Fatalf("status=%d canceled=%v", response.Code, store.canceled)
	}
}
