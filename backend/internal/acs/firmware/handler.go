package firmware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Store interface {
	List(context.Context, string, string) ([]Firmware, error)
	Get(context.Context, string, string) (Firmware, error)
	Create(context.Context, string, Input) (Firmware, error)
	Attach(context.Context, string, string, string, StoredFile) (Firmware, error)
	Exists(context.Context, string, string) error
}

type FileStorage interface {
	Save(string, io.Reader) (StoredFile, error)
	Remove(string) error
}

type Handler struct {
	store   Store
	storage FileStorage
	tokens  *auth.TokenManager
}

func NewHandler(store Store, storage FileStorage, tokens *auth.TokenManager) *Handler {
	return &Handler{store: store, storage: storage, tokens: tokens}
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
	mux.Handle("POST /api/v1/acs/firmwares/{id}/file", upload(http.HandlerFunc(h.upload)))
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
		internal(w, r)
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
	if writeError(w, r, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
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
	item, err := h.store.Create(r.Context(), principal.TenantID, input)
	if writeError(w, r, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"data": item})
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if writeError(w, r, h.store.Exists(r.Context(), principal.TenantID, id)) {
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
			internal(w, r)
			return
		}
		item, attachErr := h.store.Attach(r.Context(), principal.TenantID, id, name, stored)
		if attachErr != nil {
			_ = h.storage.Remove(stored.Path)
			writeError(w, r, attachErr)
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
func internal(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
}
func writeError(w http.ResponseWriter, r *http.Request, err error) bool {
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
	default:
		internal(w, r)
	}
	return true
}
