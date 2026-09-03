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
	CreateActivationCommand(context.Context, string, string, string, string, string, ActivationOverrides) (Command, error)
	CreateFirmwareCommand(context.Context, string, string, string, string, string) (Command, error)
	CreateSimpleCommand(context.Context, string, string, string, string) (Command, error)
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
		Type           string  `json:"type"`
		FirmwareID     string  `json:"firmware_id"`
		Scope          string  `json:"scope"`
		DNSPrimary     *string `json:"dns_primary"`
		DNSSecondary   *string `json:"dns_secondary"`
		DHCPGateway    *string `json:"dhcp_gateway"`
		DHCPSubnetMask *string `json:"dhcp_subnet_mask"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	input.FirmwareID = strings.TrimSpace(input.FirmwareID)
	input.Scope = strings.ToUpper(strings.TrimSpace(input.Scope))
	if input.Type != "ACTIVATE" && input.Type != "UPDATE_FIRMWARE" && input.Type != "SYNC" && input.Type != "REBOOT" && input.Type != "FACTORY_RESET" {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Unsupported command type")
		return
	}
	if input.Type == "UPDATE_FIRMWARE" && !uuidPattern.MatchString(input.FirmwareID) {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "A valid firmware_id is required")
		return
	}
	if input.Type == "ACTIVATE" {
		if input.Scope == "" {
			input.Scope = "ALL"
		}
		for _, value := range []*string{input.DNSPrimary, input.DNSSecondary} {
			if value != nil {
				trimmed := strings.TrimSpace(*value)
				*value = trimmed
			}
		}
		if input.Scope != "DNS" && (input.DNSPrimary != nil || input.DNSSecondary != nil) {
			httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "DNS overrides are only allowed with scope DNS")
			return
		}
		if !optionalIP(input.DNSPrimary) || !optionalIP(input.DNSSecondary) {
			httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "DNS addresses must be valid IP addresses")
			return
		}
		for _, value := range []*string{input.DHCPGateway, input.DHCPSubnetMask} {
			if value != nil {
				trimmed := strings.TrimSpace(*value)
				*value = trimmed
			}
		}
		if input.Scope != "DHCP" && (input.DHCPGateway != nil || input.DHCPSubnetMask != nil) {
			httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "DHCP overrides are only allowed with scope DHCP")
			return
		}
		if !optionalIP(input.DHCPGateway) || !optionalIPv4Mask(input.DHCPSubnetMask) {
			httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "DHCP gateway and subnet mask must be valid IPv4 values")
			return
		}
		if input.Scope != "ALL" && input.Scope != "WIFI_24" && input.Scope != "WIFI_5" && input.Scope != "DHCP" && input.Scope != "DNS" {
			httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Activation scope must be ALL, WIFI_24, WIFI_5, DHCP or DNS")
			return
		}
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
	} else if input.Type == "REBOOT" || input.Type == "FACTORY_RESET" {
		permission = "acs.router.command.reboot"
	}
	if !hasPermission(principal.Permissions, permission) {
		httpserver.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Permission denied")
		return
	}
	var command Command
	var err error
	if input.Type == "ACTIVATE" {
		overrides := ActivationOverrides{DNSPrimary: input.DNSPrimary, DNSSecondary: input.DNSSecondary, DHCPGateway: input.DHCPGateway, DHCPSubnetMask: input.DHCPSubnetMask}
		command, err = h.store.CreateActivationCommand(r.Context(), principal.TenantID, routerID, principal.UserID, idempotencyKey, input.Scope, overrides)
	} else if input.Type == "UPDATE_FIRMWARE" {
		command, err = h.store.CreateFirmwareCommand(r.Context(), principal.TenantID, routerID, principal.UserID, input.FirmwareID, idempotencyKey)
	} else {
		command, err = h.store.CreateSimpleCommand(r.Context(), principal.TenantID, routerID, principal.UserID, input.Type)
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
	if errors.Is(err, ErrInvalidModel) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_ROUTER_MODEL_INVALID", "Router model has no active TR-069 data model commands")
		return
	}
	if errors.Is(err, ErrScopeConfigurationRequired) {
		httpserver.WriteError(w, r, http.StatusConflict, "ACS_SCOPE_CONFIGURATION_REQUIRED", "The selected activation scope has no desired configuration")
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
	if len(configuration.WiFi) > 2 || (len(configuration.WiFi) == 0 && configuration.Network == nil && configuration.BandSteeringEnabled == nil) {
		return "One or two Wi-Fi radios or a network configuration are required"
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
	if configuration.Network == nil {
		return ""
	}
	network := *configuration.Network
	if network.DHCPEnabled && (!validIP(network.DHCPStart) || !validIP(network.DHCPEnd) || network.DHCPLeaseSeconds == nil || *network.DHCPLeaseSeconds <= 0) {
		return "DHCP start, end and a positive lease are required when DHCP is enabled"
	}
	if !optionalIP(network.DNSPrimary) || !optionalIP(network.DNSSecondary) {
		return "DNS addresses must be valid IP addresses"
	}
	if !optionalIP(network.DHCPGateway) || !optionalIPv4Mask(network.DHCPSubnetMask) {
		return "DHCP gateway and subnet mask must be valid IPv4 values"
	}
	return ""
}

func validIP(value *string) bool    { return value != nil && net.ParseIP(*value) != nil }
func optionalIP(value *string) bool { return value == nil || net.ParseIP(*value) != nil }
func optionalIPv4Mask(value *string) bool {
	if value == nil {
		return true
	}
	ip := net.ParseIP(*value)
	if ip == nil || ip.To4() == nil {
		return false
	}
	ones, bits := net.IPMask(ip.To4()).Size()
	return bits == 32 && ones > 0
}
