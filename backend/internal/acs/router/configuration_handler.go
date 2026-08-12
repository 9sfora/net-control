package router

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

type ConfigurationStore interface {
	SaveConfiguration(context.Context, string, string, DesiredConfiguration) (ConfigurationResult, error)
	CreateActivationCommand(context.Context, string, string, string, string) (Command, error)
	CreateFirmwareCommand(context.Context, string, string, string, string, string) (Command, error)
}

type Encryptor interface {
	Encrypt(string) ([]byte, error)
}

type ConfigurationHandler struct {
	store     ConfigurationStore
	tokens    *auth.TokenManager
	encryptor Encryptor
}

func NewConfigurationHandler(store ConfigurationStore, tokens *auth.TokenManager, encryptor Encryptor) *ConfigurationHandler {
	return &ConfigurationHandler{store: store, tokens: tokens, encryptor: encryptor}
}

func (h *ConfigurationHandler) Register(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/acs/routers/{id}/configuration",
		auth.Authenticate(h.tokens, auth.RequirePermission("acs.router.update", http.HandlerFunc(h.save))))
	mux.Handle("POST /api/v1/acs/routers/{id}/commands",
		auth.Authenticate(h.tokens, http.HandlerFunc(h.createCommand)))
}

func (h *ConfigurationHandler) save(w http.ResponseWriter, r *http.Request) {
	routerID := r.PathValue("id")
	if !uuidPattern.MatchString(routerID) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid router ID")
		return
	}
	var configuration DesiredConfiguration
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configuration); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if message := validateConfiguration(&configuration); message != "" {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}
	for index := range configuration.WiFi {
		encrypted, err := h.encryptor.Encrypt(configuration.WiFi[index].Password)
		if err != nil {
			httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			return
		}
		configuration.WiFi[index].EncryptedPassword = encrypted
		configuration.WiFi[index].Password = ""
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := h.store.SaveConfiguration(r.Context(), principal.TenantID, routerID, configuration)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, r, http.StatusNotFound, "ACS_ROUTER_NOT_FOUND", "Router not found")
		return
	}
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *ConfigurationHandler) createCommand(w http.ResponseWriter, r *http.Request) {
	routerID := r.PathValue("id")
	if !uuidPattern.MatchString(routerID) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid router ID")
		return
	}
	var input struct {
		Type       string `json:"type"`
		FirmwareID string `json:"firmware_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	input.FirmwareID = strings.TrimSpace(input.FirmwareID)
	if input.Type != "ACTIVATE" && input.Type != "UPDATE_FIRMWARE" {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Command type must be ACTIVATE or UPDATE_FIRMWARE")
		return
	}
	if input.Type == "UPDATE_FIRMWARE" && !uuidPattern.MatchString(input.FirmwareID) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "A valid firmware_id is required")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) > 200 {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Idempotency-Key must contain at most 200 characters")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	permission := "acs.router.command.activate"
	if input.Type == "UPDATE_FIRMWARE" {
		permission = "acs.router.command.update_firmware"
	}
	if !hasPermission(principal.Permissions, permission) {
		httpserver.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Permission denied")
		return
	}
	var command Command
	var err error
	if input.Type == "ACTIVATE" {
		command, err = h.store.CreateActivationCommand(r.Context(), principal.TenantID, routerID, principal.UserID, idempotencyKey)
	} else {
		command, err = h.store.CreateFirmwareCommand(r.Context(), principal.TenantID, routerID, principal.UserID, input.FirmwareID, idempotencyKey)
	}
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, r, http.StatusNotFound, "ACS_ROUTER_NOT_FOUND", "Router not found")
		return
	}
	if errors.Is(err, ErrConfigurationRequired) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_ROUTER_CONFIGURATION_REQUIRED", "Router configuration is required before activation")
		return
	}
	if errors.Is(err, ErrFirmwareUnavailable) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_FIRMWARE_UNAVAILABLE", "Firmware is not ready or is incompatible with the router")
		return
	}
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusAccepted, map[string]any{"data": command})
}

func hasPermission(permissions []string, expected string) bool {
	for _, permission := range permissions {
		if permission == expected {
			return true
		}
	}
	return false
}

func validateConfiguration(configuration *DesiredConfiguration) string {
	if len(configuration.WiFi) == 0 || len(configuration.WiFi) > 2 {
		return "One or two Wi-Fi radios are required"
	}
	bands := make(map[string]bool)
	for index := range configuration.WiFi {
		radio := &configuration.WiFi[index]
		radio.Band = strings.ToUpper(strings.TrimSpace(radio.Band))
		radio.SSID = strings.TrimSpace(radio.SSID)
		radio.SecurityMode = strings.ToUpper(strings.TrimSpace(radio.SecurityMode))
		radio.ChannelMode = strings.ToUpper(strings.TrimSpace(radio.ChannelMode))
		if (radio.Band != "2.4GHZ" && radio.Band != "5GHZ") || bands[radio.Band] {
			return "Wi-Fi bands must be unique and use 2.4GHZ or 5GHZ"
		}
		bands[radio.Band] = true
		if radio.SSID == "" || len([]byte(radio.SSID)) > 32 {
			return "SSID is required and must contain at most 32 bytes"
		}
		if len(radio.Password) < 8 || len(radio.Password) > 63 {
			return "Wi-Fi password must contain between 8 and 63 characters"
		}
		if radio.SecurityMode == "" {
			radio.SecurityMode = "WPA2_PSK"
		}
		if radio.SecurityMode != "WPA2_PSK" && radio.SecurityMode != "WPA3_SAE" && radio.SecurityMode != "WPA2_WPA3" {
			return "Unsupported Wi-Fi security mode"
		}
		if radio.ChannelMode == "" {
			radio.ChannelMode = "AUTO"
		}
		if radio.ChannelMode != "AUTO" && radio.ChannelMode != "MANUAL" {
			return "Channel mode must be AUTO or MANUAL"
		}
		if radio.ChannelMode == "MANUAL" && (radio.Channel == nil || *radio.Channel < 1 || *radio.Channel > 196) {
			return "A valid channel is required in MANUAL mode"
		}
		if radio.ChannelMode == "AUTO" {
			radio.Channel = nil
		}
	}
	network := configuration.Network
	if network.DHCPEnabled && (!validIP(network.DHCPStart) || !validIP(network.DHCPEnd) || network.DHCPLeaseSeconds == nil || *network.DHCPLeaseSeconds <= 0) {
		return "DHCP start, end and a positive lease are required when DHCP is enabled"
	}
	if !optionalIP(network.DNSPrimary) || !optionalIP(network.DNSSecondary) {
		return "DNS addresses must be valid IP addresses"
	}
	return ""
}

func validIP(value *string) bool    { return value != nil && net.ParseIP(*value) != nil }
func optionalIP(value *string) bool { return value == nil || net.ParseIP(*value) != nil }
