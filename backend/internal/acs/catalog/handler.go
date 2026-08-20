package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
var ouiPattern = regexp.MustCompile(`^[0-9A-F]{6}$`)

type Store interface {
	ListManufacturers(context.Context, string, string) ([]Manufacturer, error)
	SaveManufacturer(context.Context, string, string, ManufacturerInput) (Manufacturer, error)
	DeleteManufacturer(context.Context, string, string) error
	ListModels(context.Context, string, string, string) ([]RouterModel, error)
	GetModel(context.Context, string, string) (RouterModel, error)
	SaveModel(context.Context, string, string, RouterModelInput) (RouterModel, error)
	DeleteModel(context.Context, string, string) error
	ListTR069DataModels(context.Context, string) ([]TR069DataModel, error)
	GetTR069DataModel(context.Context, string, string) (TR069DataModel, error)
	SaveTR069DataModel(context.Context, string, string, TR069DataModelInput) (TR069DataModel, error)
	DeleteTR069DataModel(context.Context, string, string) error
	SaveHardwareVersion(context.Context, string, string, string, HardwareVersionInput) (HardwareVersion, error)
	ListCWMPParameters(context.Context, string, string) ([]CWMPParameter, error)
	ReplaceCWMPParameters(context.Context, string, string, []CWMPParameterInput) ([]CWMPParameter, error)
}
type Handler struct {
	store  Store
	tokens *auth.TokenManager
}

func NewHandler(store Store, tokens *auth.TokenManager) *Handler {
	return &Handler{store: store, tokens: tokens}
}
func (h *Handler) Register(mux *http.ServeMux) {
	read := func(next http.Handler) http.Handler {
		return auth.Authenticate(h.tokens, auth.RequirePermission("acs.catalog.read", next))
	}
	manage := func(next http.Handler) http.Handler {
		return auth.Authenticate(h.tokens, auth.RequirePermission("acs.catalog.manage", next))
	}
	mux.Handle("GET /api/v1/acs/manufacturers", read(http.HandlerFunc(h.listManufacturers)))
	mux.Handle("POST /api/v1/acs/manufacturers", manage(http.HandlerFunc(h.createManufacturer)))
	mux.Handle("PUT /api/v1/acs/manufacturers/{id}", manage(http.HandlerFunc(h.updateManufacturer)))
	mux.Handle("DELETE /api/v1/acs/manufacturers/{id}", manage(http.HandlerFunc(h.deleteManufacturer)))
	mux.Handle("GET /api/v1/acs/router-models", read(http.HandlerFunc(h.listModels)))
	mux.Handle("GET /api/v1/acs/router-models/{id}", read(http.HandlerFunc(h.getModel)))
	mux.Handle("POST /api/v1/acs/router-models", manage(http.HandlerFunc(h.createModel)))
	mux.Handle("PUT /api/v1/acs/router-models/{id}", manage(http.HandlerFunc(h.updateModel)))
	mux.Handle("DELETE /api/v1/acs/router-models/{id}", manage(http.HandlerFunc(h.deleteModel)))
	mux.Handle("GET /api/v1/acs/tr069-data-models", read(http.HandlerFunc(h.listTR069DataModels)))
	mux.Handle("GET /api/v1/acs/tr069-data-models/{id}", read(http.HandlerFunc(h.getTR069DataModel)))
	mux.Handle("POST /api/v1/acs/tr069-data-models", manage(http.HandlerFunc(h.createTR069DataModel)))
	mux.Handle("PUT /api/v1/acs/tr069-data-models/{id}", manage(http.HandlerFunc(h.updateTR069DataModel)))
	mux.Handle("DELETE /api/v1/acs/tr069-data-models/{id}", manage(http.HandlerFunc(h.deleteTR069DataModel)))
	mux.Handle("POST /api/v1/acs/router-models/{id}/hardware-versions", manage(http.HandlerFunc(h.createHardware)))
	mux.Handle("PUT /api/v1/acs/router-models/{id}/hardware-versions/{version_id}", manage(http.HandlerFunc(h.updateHardware)))
	mux.Handle("GET /api/v1/acs/router-models/{id}/cwmp-parameters", read(http.HandlerFunc(h.listCWMPParameters)))
	mux.Handle("PUT /api/v1/acs/router-models/{id}/cwmp-parameters", manage(http.HandlerFunc(h.replaceCWMPParameters)))
}
func (h *Handler) listCWMPParameters(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if !validID(w, r, modelID) {
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.ListCWMPParameters(r.Context(), p.TenantID, modelID)
	if writeCatalogError(w, r, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}
func (h *Handler) replaceCWMPParameters(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if !validID(w, r, modelID) {
		return
	}
	var input CWMPParametersInput
	if !decode(w, r, &input) {
		return
	}
	seen := make(map[string]bool, len(input.Parameters))
	validTypes := map[string]bool{"string": true, "boolean": true, "int": true, "unsignedInt": true}
	for index := range input.Parameters {
		item := &input.Parameters[index]
		item.ParameterKey = strings.ToLower(strings.TrimSpace(item.ParameterKey))
		item.ParameterName = strings.TrimSpace(item.ParameterName)
		item.XSDType = strings.TrimSpace(item.XSDType)
		if item.ParameterKey == "" || len(item.ParameterKey) > 100 || item.ParameterName == "" || len(item.ParameterName) > 500 || !validTypes[item.XSDType] || seen[item.ParameterKey] {
			validation(w, r, "Parameters require unique parameter_key, parameter_name and a valid xsd_type")
			return
		}
		seen[item.ParameterKey] = true
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.ReplaceCWMPParameters(r.Context(), p.TenantID, modelID, input.Parameters)
	if writeCatalogError(w, r, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}
func (h *Handler) listManufacturers(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.ListManufacturers(r.Context(), p.TenantID, strings.TrimSpace(r.URL.Query().Get("search")))
	if err != nil {
		internal(w, r)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}
func (h *Handler) createManufacturer(w http.ResponseWriter, r *http.Request) {
	h.saveManufacturer(w, r, "")
}
func (h *Handler) updateManufacturer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	h.saveManufacturer(w, r, id)
}
func (h *Handler) saveManufacturer(w http.ResponseWriter, r *http.Request, id string) {
	var input ManufacturerInput
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 200 {
		validation(w, r, "Name is required and must contain at most 200 characters")
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.SaveManufacturer(r.Context(), p.TenantID, id, input)
	if writeCatalogError(w, r, err) {
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	httpserver.WriteJSON(w, status, map[string]any{"data": item})
}
func (h *Handler) deleteManufacturer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	if writeCatalogError(w, r, h.store.DeleteManufacturer(r.Context(), p.TenantID, id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	manufacturerID := strings.TrimSpace(r.URL.Query().Get("manufacturer_id"))
	if manufacturerID != "" && !uuidPattern.MatchString(manufacturerID) {
		validation(w, r, "Invalid manufacturer ID")
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.ListModels(r.Context(), p.TenantID, strings.TrimSpace(r.URL.Query().Get("search")), manufacturerID)
	if err != nil {
		internal(w, r)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}
func (h *Handler) getModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.GetModel(r.Context(), p.TenantID, id)
	if writeCatalogError(w, r, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}
func (h *Handler) createModel(w http.ResponseWriter, r *http.Request) { h.saveModel(w, r, "") }
func (h *Handler) updateModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	h.saveModel(w, r, id)
}
func (h *Handler) saveModel(w http.ResponseWriter, r *http.Request, id string) {
	var input RouterModelInput
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.ManufacturerID = strings.TrimSpace(input.ManufacturerID)
	if input.TR069DataModelID != nil {
		value := strings.TrimSpace(*input.TR069DataModelID)
		input.TR069DataModelID = &value
	}
	input.Protocol = strings.ToUpper(strings.TrimSpace(input.Protocol))
	if input.Protocol == "" {
		input.Protocol = "TR-069"
	}
	if input.Name == "" || len(input.Name) > 200 || !uuidPattern.MatchString(input.ManufacturerID) {
		validation(w, r, "Name and a valid manufacturer_id are required")
		return
	}
	if input.TR069DataModelID == nil || !uuidPattern.MatchString(*input.TR069DataModelID) {
		validation(w, r, "A valid tr069_data_model_id is required")
		return
	}
	if input.Protocol != "TR-069" {
		validation(w, r, "Protocol must be TR-069")
		return
	}
	if input.OUI != nil {
		normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(*input.OUI), ":", ""), "-", ""))
		if !ouiPattern.MatchString(normalized) {
			validation(w, r, "OUI must contain 6 hexadecimal characters")
			return
		}
		input.OUI = &normalized
	}
	seen := map[string]bool{}
	for index, mode := range input.SupportedAuthModes {
		mode = strings.ToUpper(strings.TrimSpace(mode))
		if mode != "WPA2_PSK" && mode != "WPA3_SAE" && mode != "WPA2_WPA3" {
			validation(w, r, "Unsupported authentication mode")
			return
		}
		if seen[mode] {
			validation(w, r, "Authentication modes must be unique")
			return
		}
		seen[mode] = true
		input.SupportedAuthModes[index] = mode
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.SaveModel(r.Context(), p.TenantID, id, input)
	if writeCatalogError(w, r, err) {
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	httpserver.WriteJSON(w, status, map[string]any{"data": item})
}

func (h *Handler) listTR069DataModels(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.store.ListTR069DataModels(r.Context(), p.TenantID)
	if writeCatalogError(w, r, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) getTR069DataModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.GetTR069DataModel(r.Context(), p.TenantID, id)
	if writeCatalogError(w, r, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *Handler) createTR069DataModel(w http.ResponseWriter, r *http.Request) {
	h.saveTR069DataModel(w, r, "")
}

func (h *Handler) updateTR069DataModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	h.saveTR069DataModel(w, r, id)
}

func (h *Handler) saveTR069DataModel(w http.ResponseWriter, r *http.Request, id string) {
	var input TR069DataModelInput
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 100 {
		validation(w, r, "Name is required and must contain at most 100 characters")
		return
	}
	validTypes := map[string]bool{"string": true, "boolean": true, "int": true, "unsignedInt": true, "dateTime": true, "base64": true, "hexBinary": true}
	seen := make(map[string]bool, len(input.Commands))
	for index := range input.Commands {
		command := &input.Commands[index]
		command.ParameterPath = strings.TrimSpace(command.ParameterPath)
		command.ValueType = strings.TrimSpace(command.ValueType)
		key := strings.ToLower(command.ParameterPath)
		if command.ParameterPath == "" || len(command.ParameterPath) > 500 || !validTypes[command.ValueType] || seen[key] {
			validation(w, r, "Commands require a unique parameter_path and a valid value_type")
			return
		}
		seen[key] = true
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.SaveTR069DataModel(r.Context(), p.TenantID, id, input)
	if writeCatalogError(w, r, err) {
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	httpserver.WriteJSON(w, status, map[string]any{"data": item})
}

func (h *Handler) deleteTR069DataModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	if writeCatalogError(w, r, h.store.DeleteTR069DataModel(r.Context(), p.TenantID, id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) deleteModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(w, r, id) {
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	if writeCatalogError(w, r, h.store.DeleteModel(r.Context(), p.TenantID, id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) createHardware(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if !validID(w, r, modelID) {
		return
	}
	h.saveHardware(w, r, modelID, "")
}
func (h *Handler) updateHardware(w http.ResponseWriter, r *http.Request) {
	modelID, id := r.PathValue("id"), r.PathValue("version_id")
	if !validID(w, r, modelID) || !validID(w, r, id) {
		return
	}
	h.saveHardware(w, r, modelID, id)
}
func (h *Handler) saveHardware(w http.ResponseWriter, r *http.Request, modelID, id string) {
	var input HardwareVersionInput
	if !decode(w, r, &input) {
		return
	}
	input.Version = strings.TrimSpace(input.Version)
	if input.Version == "" || len(input.Version) > 100 {
		validation(w, r, "Version is required and must contain at most 100 characters")
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.store.SaveHardwareVersion(r.Context(), p.TenantID, modelID, id, input)
	if writeCatalogError(w, r, err) {
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	httpserver.WriteJSON(w, status, map[string]any{"data": item})
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		validation(w, r, "Invalid request body")
		return false
	}
	return true
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
func writeCatalogError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, r, http.StatusNotFound, "ACS_CATALOG_NOT_FOUND", "Catalog item not found")
		return true
	}
	if errors.Is(err, ErrConflict) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_CATALOG_CONFLICT", "Catalog item already exists")
		return true
	}
	if errors.Is(err, ErrInvalidReference) {
		validation(w, r, "Referenced catalog item does not exist")
		return true
	}
	if errors.Is(err, ErrInUse) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_CATALOG_IN_USE", "Catalog item is in use and cannot be deleted")
		return true
	}
	internal(w, r)
	return true
}
