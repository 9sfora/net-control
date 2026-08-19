package firmware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Store interface {
	List(context.Context, string, string) ([]Firmware, error)
	Get(context.Context, string, string) (Firmware, error)
	Create(context.Context, string, Input) (Firmware, error)
	Update(context.Context, string, string, Input) (Firmware, error)
	Publish(context.Context, string, string) (Firmware, error)
	SetDefault(context.Context, string, string) (Firmware, error)
	Delete(context.Context, string, string) (string, error)
	Attach(context.Context, string, string, string, StoredFile) (Firmware, error)
	Exists(context.Context, string, string) error
}

type FileStorage interface {
	Save(string, io.Reader) (StoredFile, error)
	Remove(string) error
	Open(string) (*os.File, error)
}

type Handler struct {
	store   Store
	storage FileStorage
	tokens  *auth.TokenManager
	logger  *slog.Logger
}

func NewHandler(store Store, storage FileStorage, tokens *auth.TokenManager, logger *slog.Logger) *Handler {
	return &Handler{store: store, storage: storage, tokens: tokens, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	read := func(next http.Handler) http.Handler {
		return auth.Authenticate(h.tokens, auth.RequirePermission("acs.firmware.read", next))
	}
	create := func(next http.Handler) http.Handler {
		return auth.Authenticate(h.tokens, auth.RequirePermission("acs.firmware.create", next))
	}
	upload := func(next http.Handler) http.Handler {
		return auth.Authenticate(h.tokens, auth.RequirePermission("acs.firmware.upload", next))
	}
	mux.Handle("GET /api/v1/acs/firmwares", read(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/acs/firmwares/{id}", read(http.HandlerFunc(h.get)))
	mux.Handle("POST /api/v1/acs/firmwares", create(http.HandlerFunc(h.create)))
	mux.Handle("PUT /api/v1/acs/firmwares/{id}", create(http.HandlerFunc(h.update)))
	mux.Handle("POST /api/v1/acs/firmwares/{id}/publish", create(http.HandlerFunc(h.publish)))
	mux.Handle("POST /api/v1/acs/firmwares/{id}/default", create(http.HandlerFunc(h.setDefault)))
	mux.Handle("DELETE /api/v1/acs/firmwares/{id}", create(http.HandlerFunc(h.delete)))
	mux.Handle("POST /api/v1/acs/firmwares/{id}/file", upload(http.HandlerFunc(h.upload)))
	mux.Handle("GET /api/v1/acs/firmwares/{id}/file", read(http.HandlerFunc(h.download)))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	modelID := strings.TrimSpace(r.URL.Query().Get("router_model_id"))
	if modelID != "" && !uuidPattern.MatchString(modelID) {
		validation(w, r, "Invalid router_model_id")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.List(r.Context(), principal.TenantID, modelID)
	if err != nil {
		h.internal(w, r, "list firmware", err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.Get(r.Context(), principal.TenantID, id)
	if h.writeError(w, r, "get firmware", err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	h.save(w, r, "")
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	h.save(w, r, id)
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request, id string) {
	var input Input
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		validation(w, r, "Invalid request body")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Version = strings.TrimSpace(input.Version)
	input.RouterModelID = strings.TrimSpace(input.RouterModelID)
	if input.HardwareVersionID != nil {
		value := strings.TrimSpace(*input.HardwareVersionID)
		input.HardwareVersionID = &value
	}
	if input.Name == "" || len(input.Name) > 200 || input.Version == "" || len(input.Version) > 100 || !uuidPattern.MatchString(input.RouterModelID) {
		validation(w, r, "Name, version and a valid router_model_id are required")
		return
	}
	if input.HardwareVersionID != nil && !uuidPattern.MatchString(*input.HardwareVersionID) {
		validation(w, r, "Invalid hardware_version_id")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	var item Firmware
	var err error
	if id == "" {
		item, err = h.store.Create(r.Context(), principal.TenantID, input)
	} else {
		item, err = h.store.Update(r.Context(), principal.TenantID, id, input)
	}
	if h.writeError(w, r, "save firmware metadata", err) {
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	httpserver.WriteJSON(w, status, map[string]any{"data": item})
}

func (h *Handler) setDefault(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.SetDefault(r.Context(), principal.TenantID, id)
	if h.writeError(w, r, "set default firmware", err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.Publish(r.Context(), principal.TenantID, id)
	if h.writeError(w, r, "publish firmware", err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	path, err := h.store.Delete(r.Context(), principal.TenantID, id)
	if h.writeError(w, r, "delete firmware metadata", err) {
		return
	}
	if err := h.storage.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		h.internal(w, r, "remove firmware file", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.Get(r.Context(), principal.TenantID, id)
	if h.writeError(w, r, "get firmware for download", err) {
		return
	}
	if item.Status != "READY" || item.FileName == nil {
		httpserver.WriteError(w, r, http.StatusConflict, "FIRMWARE_FILE_UNAVAILABLE", "Firmware file is not available")
		return
	}
	file, err := h.storage.Open(id + ".bin")
	if errors.Is(err, os.ErrNotExist) {
		httpserver.WriteError(w, r, http.StatusNotFound, "FIRMWARE_FILE_NOT_FOUND", "Firmware file not found")
		return
	}
	if err != nil {
		h.internal(w, r, "open firmware file", err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", *item.FileName))
	if item.FileSize != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*item.FileSize, 10))
	}
	if item.ChecksumSHA256 != nil {
		w.Header().Set("X-Checksum-SHA256", *item.ChecksumSHA256)
	}
	_, _ = io.Copy(w, file)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if h.writeError(w, r, "validate firmware upload", h.store.Exists(r.Context(), principal.TenantID, id)) {
		return
	}
	reader, err := r.MultipartReader()
	if err != nil {
		validation(w, r, "Content-Type must be multipart/form-data")
		return
	}
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			validation(w, r, "Invalid multipart body")
			return
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		name := filepath.Base(strings.TrimSpace(part.FileName()))
		if name == "." || name == "" || len(name) > 255 {
			part.Close()
			validation(w, r, "A file with a valid name is required")
			return
		}
		stored, saveErr := h.storage.Save(id, part)
		part.Close()
		if errors.Is(saveErr, ErrFileTooLarge) {
			httpserver.WriteError(w, r, http.StatusRequestEntityTooLarge, "FIRMWARE_FILE_TOO_LARGE", "Firmware file exceeds the configured size limit")
			return
		}
		if saveErr != nil {
			h.logger.ErrorContext(r.Context(), "firmware request failed", "operation", "store uploaded firmware", "request_id", httpserver.RequestID(r.Context()), "error", saveErr)
			if errors.Is(saveErr, os.ErrPermission) {
				httpserver.WriteError(w, r, http.StatusServiceUnavailable, "FIRMWARE_STORAGE_NOT_WRITABLE", "Firmware storage is not writable; verify the server storage permissions")
				return
			}
			httpserver.WriteError(w, r, http.StatusServiceUnavailable, "FIRMWARE_STORAGE_UNAVAILABLE", "Firmware storage is temporarily unavailable")
			return
		}
		item, attachErr := h.store.Attach(r.Context(), principal.TenantID, id, name, stored)
		if attachErr != nil {
			_ = h.storage.Remove(stored.Path)
			h.writeError(w, r, "attach uploaded firmware", attachErr)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
		return
	}
	validation(w, r, "Multipart field file is required")
}

func validID(w http.ResponseWriter, r *http.Request, id string) bool {
	if !uuidPattern.MatchString(id) {
		validation(w, r, "Invalid ID")
		return false
	}
	return true
}
func validation(w http.ResponseWriter, r *http.Request, message string) {
	httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", message)
}
func (h *Handler) internal(w http.ResponseWriter, r *http.Request, operation string, err error) {
	h.logger.ErrorContext(r.Context(), "firmware request failed", "operation", operation, "request_id", httpserver.RequestID(r.Context()), "error", err)
	httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, operation string, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "FIRMWARE_NOT_FOUND", "Firmware not found")
	case errors.Is(err, ErrConflict):
		httpserver.WriteError(w, r, http.StatusConflict, "FIRMWARE_CONFLICT", "Firmware already exists")
	case errors.Is(err, ErrInvalidCompatibility):
		validation(w, r, "Router model or hardware version is invalid or inactive")
	case errors.Is(err, ErrImmutable):
		httpserver.WriteError(w, r, http.StatusConflict, "FIRMWARE_IMMUTABLE", "Published firmware cannot be changed")
	case errors.Is(err, ErrFileRequired):
		httpserver.WriteError(w, r, http.StatusConflict, "FIRMWARE_FILE_REQUIRED", "A firmware file is required before publishing")
	case errors.Is(err, ErrNotPublished):
		httpserver.WriteError(w, r, http.StatusConflict, "FIRMWARE_NOT_PUBLISHED", "Only published firmware can be set as default")
	default:
		h.internal(w, r, operation, err)
	}
	return true
}
