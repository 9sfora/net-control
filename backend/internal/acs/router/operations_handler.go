package router

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

type OperationsStore interface {
	ListRouters(context.Context, string, ListFilter) ([]Summary, error)
	GetRouter(context.Context, string, string) (Detail, error)
}

type OperationsHandler struct {
	store                         OperationsStore
	tokens                        *auth.TokenManager
	onlineTimeout, offlineTimeout time.Duration
	now                           func() time.Time
}

func NewOperationsHandler(store OperationsStore, tokens *auth.TokenManager, onlineTimeout, offlineTimeout time.Duration) *OperationsHandler {
	return &OperationsHandler{store: store, tokens: tokens, onlineTimeout: onlineTimeout, offlineTimeout: offlineTimeout, now: time.Now}
}

func (h *OperationsHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/acs/routers", auth.Authenticate(h.tokens, auth.RequirePermission("acs.router.read", http.HandlerFunc(h.list))))
	mux.Handle("GET /api/v1/acs/routers/{id}", auth.Authenticate(h.tokens, auth.RequirePermission("acs.router.read", http.HandlerFunc(h.get))))
}

func (h *OperationsHandler) list(w http.ResponseWriter, r *http.Request) {
	connectivity := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("connectivity_status")))
	if connectivity != "" && connectivity != "ONLINE" && connectivity != "STALE" && connectivity != "OFFLINE" && connectivity != "NEVER_CONNECTED" {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid connectivity status")
		return
	}
	desired := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("desired_status")))
	if desired != "" && desired != "ACTIVE" && desired != "INACTIVE" && desired != "BLOCKED" {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid desired status")
		return
	}
	limit, ok := routerInteger(r, "limit", 50, 1, 100)
	if !ok {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Limit must be between 1 and 100")
		return
	}
	offset, ok := routerInteger(r, "offset", 0, 0, 1_000_000)
	if !ok {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Offset must be non-negative")
		return
	}
	now := h.now()
	principal, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.ListRouters(r.Context(), principal.TenantID, ListFilter{Search: strings.TrimSpace(r.URL.Query().Get("search")), ConnectivityStatus: connectivity, DesiredStatus: desired, OnlineCutoff: now.Add(-h.onlineTimeout), OfflineCutoff: now.Add(-h.offlineTimeout), Limit: limit, Offset: offset})
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	for index := range items {
		items[index].ConnectivityStatus = ConnectivityStatus(items[index].LastSeenAt, now, h.onlineTimeout, h.offlineTimeout)
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items, "pagination": map[string]int{"limit": limit, "offset": offset, "count": len(items)}})
}

func (h *OperationsHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid router ID")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	detail, err := h.store.GetRouter(r.Context(), principal.TenantID, id)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, r, http.StatusNotFound, "ACS_ROUTER_NOT_FOUND", "Router not found")
		return
	}
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	detail.ConnectivityStatus = ConnectivityStatus(detail.LastSeenAt, h.now(), h.onlineTimeout, h.offlineTimeout)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func routerInteger(r *http.Request, key string, fallback, minimum, maximum int) (int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value >= minimum && value <= maximum
}
