package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

type Store interface {
	Snapshot(context.Context, string, time.Time, time.Duration, time.Duration) (Snapshot, error)
}
type Handler struct {
	store                         Store
	tokens                        *auth.TokenManager
	onlineTimeout, offlineTimeout time.Duration
	now                           func() time.Time
}

func NewHandler(store Store, tokens *auth.TokenManager, onlineTimeout, offlineTimeout time.Duration) *Handler {
	return &Handler{store: store, tokens: tokens, onlineTimeout: onlineTimeout, offlineTimeout: offlineTimeout, now: time.Now}
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/acs/dashboard", auth.Authenticate(h.tokens, auth.RequirePermission("acs.dashboard.read", http.HandlerFunc(h.get))))
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	snapshot, err := h.store.Snapshot(r.Context(), principal.TenantID, h.now(), h.onlineTimeout, h.offlineTimeout)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": snapshot})
}
