package router

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Store interface {
	ListUnknown(context.Context, string, int, int) ([]Unknown, error)
	Configure(context.Context, string, string, ConfigureUnknownInput) (Router, error)
}

type Handler struct {
	store  Store
	tokens *auth.TokenManager
}

func NewHandler(store Store, tokens *auth.TokenManager) *Handler {
	return &Handler{store: store, tokens: tokens}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/acs/unknown-routers",
		auth.Authenticate(h.tokens, auth.RequirePermission("acs.unknown_router.read", http.HandlerFunc(h.listUnknown))))
	mux.Handle("POST /api/v1/acs/unknown-routers/{id}/configure",
		auth.Authenticate(h.tokens, auth.RequirePermission("acs.unknown_router.configure", http.HandlerFunc(h.configure))))
}

func (h *Handler) listUnknown(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	limit, ok := queryInteger(r, "limit", 50, 1, 100)
	if !ok {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Limit must be between 1 and 100")
		return
	}
	offset, ok := queryInteger(r, "offset", 0, 0, 1_000_000)
	if !ok {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Offset must be a non-negative integer")
		return
	}
	items, err := h.store.ListUnknown(r.Context(), principal.TenantID, limit, offset)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"data": items, "pagination": map[string]int{"limit": limit, "offset": offset, "count": len(items)},
	})
}

func (h *Handler) configure(w http.ResponseWriter, r *http.Request) {
	unknownID := r.PathValue("id")
	if !uuidPattern.MatchString(unknownID) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid unknown router ID")
		return
	}
	var input struct {
		Name              string  `json:"name"`
		RouterModelID     string  `json:"router_model_id"`
		HardwareVersionID *string `json:"hardware_version_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Name) == "" || len(strings.TrimSpace(input.Name)) > 200 {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Name is required and must contain at most 200 characters")
		return
	}
	input.RouterModelID = strings.TrimSpace(input.RouterModelID)
	if !uuidPattern.MatchString(input.RouterModelID) || (input.HardwareVersionID != nil && !uuidPattern.MatchString(*input.HardwareVersionID)) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Valid router_model_id and hardware_version_id are required")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	configured, err := h.store.Configure(r.Context(), principal.TenantID, unknownID, ConfigureUnknownInput{Name: strings.TrimSpace(input.Name), RouterModelID: input.RouterModelID, HardwareVersionID: input.HardwareVersionID, CreatedBy: principal.UserID})
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, r, http.StatusNotFound, "ACS_UNKNOWN_ROUTER_NOT_FOUND", "Unknown router not found")
		return
	}
	if errors.Is(err, ErrInvalidModel) || errors.Is(err, ErrInvalidHardware) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "ACS_CATALOG_REFERENCE_INVALID", "Router model or hardware version is invalid")
		return
	}
	if errors.Is(err, ErrIncompatibleModel) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_ROUTER_MODEL_INCOMPATIBLE", "Router model does not match the reported OUI or Product Class")
		return
	}
	if errors.Is(err, ErrIncompatibleHardware) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_ROUTER_HARDWARE_INCOMPATIBLE", "Hardware version does not match the version reported by the router")
		return
	}
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"data": configured})
}

func queryInteger(r *http.Request, key string, fallback, minimum, maximum int) (int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value >= minimum && value <= maximum
}
