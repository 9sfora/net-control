package cwmp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

const maxRequestBody = 2 << 20

type InformRecorder interface {
	RecordInform(context.Context, Inform, string, string) (Reception, error)
}

type Handler struct {
	repository InformRecorder
	logger     *slog.Logger
}

func NewHandler(repository InformRecorder, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /cwmp", h.cwmp)
}

func (h *Handler) cwmp(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	inform, err := ParseInform(bytes.NewReader(body))
	if errors.Is(err, ErrInvalidInform) {
		http.Error(w, "invalid CWMP Inform", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "CWMP parsing failed", "error", err)
		http.Error(w, "invalid CWMP request", http.StatusBadRequest)
		return
	}
	reception, err := h.repository.RecordInform(r.Context(), inform, remoteIP(r), r.UserAgent())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "CWMP Inform persistence failed", "error", err,
			"oui", inform.OUI, "product_class", inform.ProductClass, "serial_number", inform.SerialNumber)
		http.Error(w, "ACS unavailable", http.StatusServiceUnavailable)
		return
	}

	h.logger.InfoContext(r.Context(), "CWMP Inform received", "session_id", reception.SessionID,
		"known_router", reception.Known, "oui", inform.OUI, "product_class", inform.ProductClass,
		"serial_number", inform.SerialNumber)
	http.SetCookie(w, &http.Cookie{Name: "ACS_SESSION", Value: reception.SessionID, Path: "/cwmp", HttpOnly: true, Secure: true})
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(InformResponse(inform.ID))
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
