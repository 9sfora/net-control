package firmware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeliveryHandler struct {
	pool       *pgxpool.Pool
	tenantSlug string
	storage    *Storage
}

func NewDeliveryHandler(pool *pgxpool.Pool, tenantSlug string, storage *Storage) *DeliveryHandler {
	return &DeliveryHandler{pool: pool, tenantSlug: tenantSlug, storage: storage}
}
func (h *DeliveryHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /firmwares/{id}/download", h.download)
}
func (h *DeliveryHandler) download(w http.ResponseWriter, r *http.Request) {
	id, token := r.PathValue("id"), r.URL.Query().Get("token")
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(token) {
		http.NotFound(w, r)
		return
	}
	var path, name, checksum string
	var size int64
	err := h.resolve(r.Context(), id, token, &path, &name, &checksum, &size)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "ACS unavailable", http.StatusServiceUnavailable)
		return
	}
	file, err := h.storage.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "ACS unavailable", http.StatusServiceUnavailable)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("X-Checksum-SHA256", checksum)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, file)
}
func (h *DeliveryHandler) resolve(ctx context.Context, id, token string, path, name, checksum *string, size *int64) error {
	var tenantID string
	if err := h.pool.QueryRow(ctx, `SELECT id FROM public.tenants WHERE slug=$1 AND active`, h.tenantSlug).Scan(&tenantID); err != nil {
		return err
	}
	return database.InTenantTransaction(ctx, h.pool, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT f.file_path,f.file_name,f.checksum_sha256,f.file_size FROM acs.firmwares f JOIN acs.router_commands c ON c.tenant_id=f.tenant_id AND c.parameters->>'firmware_id'=f.id::text JOIN acs.router_command_attempts a ON a.router_command_id=c.id AND a.tenant_id=c.tenant_id WHERE f.tenant_id=$1 AND f.id=$2 AND a.id=$3 AND f.active AND f.status='READY' AND c.type='UPDATE_FIRMWARE' AND c.status='PROCESSING' AND a.status IN('PROCESSING','COMPLETED')`, tenantID, id, token).Scan(path, name, checksum, size)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
}
