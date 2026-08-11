package cwmp

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRecorder struct {
	inform Inform
}

func (f *fakeRecorder) RecordInform(_ context.Context, inform Inform, _, _ string) (Reception, error) {
	f.inform = inform
	return Reception{SessionID: "session-id", Known: false}, nil
}

func TestHandlerReceivesInform(t *testing.T) {
	repository := &fakeRecorder{}
	handler := NewHandler(repository, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodPost, "/cwmp", strings.NewReader(sampleInform))
	request.RemoteAddr = "192.0.2.10:7547"
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.inform.SerialNumber != "SN123" || !strings.Contains(response.Body.String(), "InformResponse") {
		t.Fatalf("Inform was not processed: %+v, body = %s", repository.inform, response.Body.String())
	}
	if len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].Value != "session-id" {
		t.Fatal("ACS session cookie was not set")
	}
}
