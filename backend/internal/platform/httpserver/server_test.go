package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeDatabase struct{ err error }

var testHosts = VirtualHosts{Application: "net-control.example.com", ACS: "acs.example.com"}

func (db fakeDatabase) Ping(context.Context) error { return db.err }

func TestHealth(t *testing.T) {
	server := New(":0", testHosts, fakeDatabase{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Host = testHosts.Application
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestReadyWhenDatabaseIsUnavailable(t *testing.T) {
	server := New(":0", testHosts, fakeDatabase{err: errors.New("unavailable")}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	request.Host = testHosts.ACS
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(response.Body.String(), `"code":"DEPENDENCY_ERROR"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestRoutesAreIsolatedByHost(t *testing.T) {
	registerApplication := func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api/v1/private", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	}
	server := New(":0", testHosts, fakeDatabase{}, slog.New(slog.NewTextHandler(io.Discard, nil)), registerApplication, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/private", nil)
	request.Host = testHosts.ACS
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("ACS host status = %d, want %d", response.Code, http.StatusNotFound)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/private", nil)
	request.Host = "unknown.example.com"
	response = httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest {
		t.Fatalf("unknown host status = %d, want %d", response.Code, http.StatusMisdirectedRequest)
	}
}
