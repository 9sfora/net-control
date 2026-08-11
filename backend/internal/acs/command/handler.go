package command

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

var validStatuses = map[string]bool{
	"PENDING": true, "WAITING_DEVICE": true, "PROCESSING": true, "RETRY_SCHEDULED": true,
	"COMPLETED": true, "FAILED": true, "CANCELED": true, "EXPIRED": true,
}

type Store interface {
	List(context.Context, string, Filter) ([]Item, error)
	Get(context.Context, string, string) (Detail, error)
	Cancel(context.Context, string, string, string) (Item, error)
}

type Handler struct {
	store  Store
	tokens *auth.TokenManager
}

func NewHandler(store Store, tokens *auth.TokenManager) *Handler {
	return &Handler{store: store, tokens: tokens}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/acs/commands", auth.Authenticate(h.tokens, auth.RequirePermission("acs.command.read", http.HandlerFunc(h.list))))
	mux.Handle("GET /api/v1/acs/commands/{id}", auth.Authenticate(h.tokens, auth.RequirePermission("acs.command.read", http.HandlerFunc(h.get))))
	mux.Handle("POST /api/v1/acs/commands/{id}/cancel", auth.Authenticate(h.tokens, auth.RequirePermission("acs.command.cancel", http.HandlerFunc(h.cancel))))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validStatuses[status] {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid command status")
		return
	}
	routerID := strings.TrimSpace(r.URL.Query().Get("router_id"))
	if routerID != "" && !uuidPattern.MatchString(routerID) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid router ID")
		return
	}
	limit, ok := integer(r, "limit", 50, 1, 100)
	if !ok {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Limit must be between 1 and 100")
		return
	}
	offset, ok := integer(r, "offset", 0, 0, 1_000_000)
	if !ok {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Offset must be non-negative")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.List(r.Context(), principal.TenantID, Filter{Status: status, RouterID: routerID, Limit: limit, Offset: offset})
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items, "pagination": map[string]int{"limit": limit, "offset": offset, "count": len(items)}})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid command ID")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	detail, err := h.store.Get(r.Context(), principal.TenantID, id)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, r, http.StatusNotFound, "ACS_COMMAND_NOT_FOUND", "Command not found")
		return
	}
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid command ID")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.Cancel(r.Context(), principal.TenantID, id, principal.UserID)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, r, http.StatusNotFound, "ACS_COMMAND_NOT_FOUND", "Command not found")
		return
	}
	if errors.Is(err, ErrCannotCancel) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_COMMAND_CANNOT_BE_CANCELED", "Command has already started or reached a final state")
		return
	}
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}

func integer(r *http.Request, key string, fallback, minimum, maximum int) (int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value >= minimum && value <= maximum
}
