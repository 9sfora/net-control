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

	"github.com/jackc/pgx/v5/pgtype"
)

const maxRequestBody = 2 << 20

type InformRecorder interface {
	RecordInform(context.Context, Inform, string, string) (Reception, error)
}

type CommandDispatcher interface {
	Next(context.Context, string) (OutboundCommand, error)
	Complete(context.Context, string, CommandResult) error
}

type Handler struct {
	repository   InformRecorder
	dispatcher   CommandDispatcher
	logger       *slog.Logger
	cookieSecure bool
}

func NewHandler(repository InformRecorder, dispatcher CommandDispatcher, logger *slog.Logger, cookieSecure bool) *Handler {
	return &Handler{repository: repository, dispatcher: dispatcher, logger: logger, cookieSecure: cookieSecure}
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
		h.dispatch(w, r)
		return
	}

	inform, err := ParseInform(bytes.NewReader(body))
	if errors.Is(err, ErrInvalidInform) {
		h.complete(w, r, body)
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
	http.SetCookie(w, &http.Cookie{Name: "ACS_SESSION", Value: reception.SessionID, Path: "/cwmp", HttpOnly: true, Secure: h.cookieSecure})
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(InformResponse(inform.ID))
}

func (h *Handler) dispatch(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := sessionID(r)
	if !ok || h.dispatcher == nil {
		http.Error(w, "invalid ACS session", http.StatusBadRequest)
		return
	}
	command, err := h.dispatcher.Next(r.Context(), sessionID)
	if errors.Is(err, ErrNoCommand) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "CWMP command dispatch failed", "error", err, "session_id", sessionID)
		http.Error(w, "ACS unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(EncodeCommand(command))
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request, body []byte) {
	sessionID, ok := sessionID(r)
	if !ok || h.dispatcher == nil {
		http.Error(w, "invalid ACS session", http.StatusBadRequest)
		return
	}
	result, err := ParseCommandResult(body)
	if err != nil {
		http.Error(w, "unsupported CWMP message", http.StatusBadRequest)
		return
	}
	if err := h.dispatcher.Complete(r.Context(), sessionID, result); err != nil && !errors.Is(err, ErrNoCommand) {
		h.logger.ErrorContext(r.Context(), "CWMP command completion failed", "error", err, "session_id", sessionID)
		http.Error(w, "ACS unavailable", http.StatusServiceUnavailable)
		return
	}
	if result.Kind == "TRANSFER_COMPLETE" {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(TransferCompleteResponse())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func sessionID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("ACS_SESSION")
	if err != nil {
		return "", false
	}
	var id pgtype.UUID
	if err := id.Scan(cookie.Value); err != nil || !id.Valid {
		return "", false
	}
	return cookie.Value, true
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
