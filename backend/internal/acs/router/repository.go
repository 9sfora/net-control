package router

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound                   = errors.New("router not found")
	ErrConfigurationRequired      = errors.New("router configuration is required")
	ErrInvalidModel               = errors.New("router model is invalid")
	ErrInvalidHardware            = errors.New("hardware version is invalid")
	ErrIncompatibleModel          = errors.New("router model is incompatible")
	ErrIncompatibleHardware       = errors.New("hardware version is incompatible")
	ErrFirmwareUnavailable        = errors.New("firmware is unavailable")
	ErrScopeConfigurationRequired = errors.New("activation scope configuration is required")
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) ListUnknown(ctx context.Context, tenantID string, limit, offset int) ([]Unknown, error) {
	items := make([]Unknown, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, manufacturer, oui, product_class, serial_number, software_version,
			       hardware_version, first_seen_at, last_seen_at, host(last_ip_address),
			       last_user_agent, inform_count, last_events, last_parameters
			FROM acs.unknown_routers
			WHERE tenant_id = $1
			ORDER BY last_seen_at DESC, id
			LIMIT $2 OFFSET $3`, tenantID, limit, offset)
		if err != nil {
			return fmt.Errorf("query unknown routers: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item Unknown
			var events, parameters []byte
			if err := rows.Scan(&item.ID, &item.Manufacturer, &item.OUI, &item.ProductClass, &item.SerialNumber,
				&item.SoftwareVersion, &item.HardwareVersion, &item.FirstSeenAt, &item.LastSeenAt,
				&item.LastIPAddress, &item.LastUserAgent, &item.InformCount, &events, &parameters); err != nil {
				return fmt.Errorf("scan unknown router: %w", err)
			}
			if err := json.Unmarshal(events, &item.LastEvents); err != nil {
				return fmt.Errorf("decode router events: %w", err)
			}
			if err := json.Unmarshal(parameters, &item.LastParameters); err != nil {
				return fmt.Errorf("decode router parameters: %w", err)
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) CreateFirmwareCommand(ctx context.Context, tenantID, routerID, userID, firmwareID, idempotencyKey string) (Command, error) {
	var command Command
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var compatible, routerExists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM acs.routers r
				JOIN acs.firmwares f ON f.tenant_id=r.tenant_id AND f.router_model_id=r.router_model_id
				WHERE r.tenant_id=$1 AND r.id=$2 AND f.id=$3 AND f.active AND f.status='READY'
				  AND (f.hardware_version_id IS NULL OR f.hardware_version_id=r.hardware_version_id)
			), EXISTS(SELECT 1 FROM acs.routers WHERE tenant_id=$1 AND id=$2)`, tenantID, routerID, firmwareID).Scan(&compatible, &routerExists)
		if err != nil {
			return fmt.Errorf("validate firmware command: %w", err)
		}
		if !routerExists {
			return ErrNotFound
		}
		if !compatible {
			return ErrFirmwareUnavailable
		}
		parameters, _ := json.Marshal(map[string]string{"firmware_id": firmwareID})
		var key any
		if idempotencyKey != "" {
			key = idempotencyKey
		}
		var encoded []byte
		err = tx.QueryRow(ctx, `
			INSERT INTO acs.router_commands(tenant_id,router_id,created_by,type,parameters,idempotency_key,priority)
			VALUES($1,$2,$3,'UPDATE_FIRMWARE',$4,$5,'HIGH')
			ON CONFLICT (tenant_id,idempotency_key) WHERE idempotency_key IS NOT NULL
			DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key
			RETURNING id,router_id,type,status,priority,parameters,idempotency_key`, tenantID, routerID, userID, parameters, key).Scan(
			&command.ID, &command.RouterID, &command.Type, &command.Status, &command.Priority, &encoded, &command.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("create firmware command: %w", err)
		}
		if err := json.Unmarshal(encoded, &command.Parameters); err != nil {
			return err
		}
		return nil
	})
	return command, err
}

func (r *Repository) CreateSimpleCommand(ctx context.Context, tenantID, routerID, userID, commandType string) (Command, error) {
	var command Command
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		parameters := map[string]any{}
		if commandType == "SYNC" {
			rows, err := tx.Query(ctx, `SELECT DISTINCT split_part(c.parameter_path,'.',1)||'.' FROM acs.routers r JOIN acs.router_models m ON m.id=r.router_model_id AND m.tenant_id=r.tenant_id JOIN acs.tr069_data_model_commands c ON c.data_model_id=m.tr069_data_model_id AND c.tenant_id=m.tenant_id WHERE r.tenant_id=$1 AND r.id=$2 AND c.active ORDER BY 1`, tenantID, routerID)
			if err != nil {
				return err
			}
			items := make([]map[string]string, 0)
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					rows.Close()
					return err
				}
				items = append(items, map[string]string{"name": name, "type": "string"})
			}
			rows.Close()
			if len(items) == 0 {
				return ErrInvalidModel
			}
			parameters["parameters"] = items
		}
		encoded, _ := json.Marshal(parameters)
		err := tx.QueryRow(ctx, `INSERT INTO acs.router_commands(tenant_id,router_id,created_by,type,parameters,priority) SELECT $1,$2,$3,$4,$5,'HIGH' WHERE EXISTS(SELECT 1 FROM acs.routers WHERE tenant_id=$1 AND id=$2) RETURNING id,router_id,type,status,priority,parameters,idempotency_key`, tenantID, routerID, userID, commandType, encoded).Scan(&command.ID, &command.RouterID, &command.Type, &command.Status, &command.Priority, &encoded, &command.IdempotencyKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err == nil {
			err = json.Unmarshal(encoded, &command.Parameters)
		}
		return err
	})
	return command, err
}

func (r *Repository) Configure(ctx context.Context, tenantID, unknownID string, input ConfigureUnknownInput) (Router, error) {
	var configured Router
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var unknown Unknown
		err := tx.QueryRow(ctx, `
			SELECT id, manufacturer, oui, product_class, serial_number, software_version,
			       hardware_version, first_seen_at, last_seen_at, host(last_ip_address)
			FROM acs.unknown_routers
			WHERE tenant_id = $1 AND id = $2
			FOR UPDATE`, tenantID, unknownID).Scan(
			&unknown.ID, &unknown.Manufacturer, &unknown.OUI, &unknown.ProductClass, &unknown.SerialNumber,
			&unknown.SoftwareVersion, &unknown.HardwareVersion, &unknown.FirstSeenAt, &unknown.LastSeenAt, &unknown.LastIPAddress)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock unknown router: %w", err)
		}
		var modelOUI, modelProductClass *string
		var dataModelID string
		if err := tx.QueryRow(ctx, `SELECT m.oui,m.product_class,m.tr069_data_model_id FROM acs.router_models m JOIN acs.tr069_data_models d ON d.tenant_id=m.tenant_id AND d.id=m.tr069_data_model_id AND d.active WHERE m.tenant_id=$1 AND m.id=$2 AND m.active`, tenantID, input.RouterModelID).Scan(&modelOUI, &modelProductClass, &dataModelID); errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidModel
		} else if err != nil {
			return fmt.Errorf("find router model: %w", err)
		}
		if modelOUI != nil && *modelOUI != "" && !strings.EqualFold(*modelOUI, unknown.OUI) {
			return ErrIncompatibleModel
		}
		if modelProductClass != nil && *modelProductClass != "" && !strings.EqualFold(*modelProductClass, unknown.ProductClass) {
			return ErrIncompatibleModel
		}
		if input.HardwareVersionID != nil {
			var version string
			if err := tx.QueryRow(ctx, `SELECT version FROM acs.router_model_hardware_versions WHERE tenant_id=$1 AND router_model_id=$2 AND id=$3 AND active`, tenantID, input.RouterModelID, *input.HardwareVersionID).Scan(&version); errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidHardware
			} else if err != nil {
				return fmt.Errorf("find hardware version: %w", err)
			}
			if unknown.HardwareVersion != nil && *unknown.HardwareVersion != "" && !strings.EqualFold(version, *unknown.HardwareVersion) {
				return ErrIncompatibleHardware
			}
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO acs.routers
				(tenant_id, name, manufacturer, oui, product_class, serial_number, software_version,
				 hardware_version, first_seen_at, last_seen_at, last_inform_at, last_ip_address,router_model_id,hardware_version_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11,$12,$13)
			RETURNING id, name, manufacturer, oui, product_class, serial_number, software_version,
			          hardware_version,router_model_id,hardware_version_id,desired_status, reported_status, first_seen_at, last_seen_at, created_at`,
			tenantID, input.Name, unknown.Manufacturer, unknown.OUI, unknown.ProductClass, unknown.SerialNumber,
			unknown.SoftwareVersion, unknown.HardwareVersion, unknown.FirstSeenAt, unknown.LastSeenAt,
			unknown.LastIPAddress, input.RouterModelID, input.HardwareVersionID).Scan(&configured.ID, &configured.Name, &configured.Manufacturer,
			&configured.OUI, &configured.ProductClass, &configured.SerialNumber, &configured.SoftwareVersion,
			&configured.HardwareVersion, &configured.RouterModelID, &configured.HardwareVersionID, &configured.DesiredStatus, &configured.ReportedStatus,
			&configured.FirstSeenAt, &configured.LastSeenAt, &configured.CreatedAt)
		if err != nil {
			return fmt.Errorf("create configured router: %w", err)
		}
		parameters := make([]map[string]string, 0)
		seenRoots := make(map[string]bool)
		sourceParameterCount := 0
		rows, err := tx.Query(ctx, `SELECT parameter_path,value_type FROM acs.tr069_data_model_commands WHERE tenant_id=$1 AND data_model_id=$2 AND active ORDER BY parameter_path`, tenantID, dataModelID)
		if err != nil {
			return fmt.Errorf("load TR-069 data model commands: %w", err)
		}
		for rows.Next() {
			var name, valueType string
			if err := rows.Scan(&name, &valueType); err != nil {
				rows.Close()
				return fmt.Errorf("scan TR-069 data model command: %w", err)
			}
			sourceParameterCount++
			separator := strings.Index(name, ".")
			if separator <= 0 {
				continue
			}
			root := name[:separator+1]
			if !seenRoots[root] {
				seenRoots[root] = true
				parameters = append(parameters, map[string]string{"name": root, "type": valueType})
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate TR-069 data model commands: %w", err)
		}
		if len(parameters) == 0 {
			return ErrInvalidModel
		}
		encodedParameters, err := json.Marshal(map[string]any{"data_model_id": dataModelID, "source_parameter_count": sourceParameterCount, "parameters": parameters})
		if err != nil {
			return fmt.Errorf("encode sync parameters: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO acs.router_commands(tenant_id,router_id,created_by,type,parameters,priority) VALUES($1,$2,$3,'SYNC',$4,'HIGH')`, tenantID, configured.ID, input.CreatedBy, encodedParameters); err != nil {
			return fmt.Errorf("create initial sync command: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE acs.router_sessions
			SET router_id = $1, unknown_router_id = NULL
			WHERE tenant_id = $2 AND unknown_router_id = $3`, configured.ID, tenantID, unknownID); err != nil {
			return fmt.Errorf("move router sessions: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM acs.unknown_routers WHERE tenant_id = $1 AND id = $2`, tenantID, unknownID); err != nil {
			return fmt.Errorf("remove unknown router: %w", err)
		}
		return nil
	})
	return configured, err
}

func (r *Repository) SaveConfiguration(ctx context.Context, tenantID, routerID string, configuration DesiredConfiguration) (ConfigurationResult, error) {
	result := ConfigurationResult{RouterID: routerID}
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT configuration_version FROM acs.routers WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, routerID).Scan(&result.Version); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("lock router: %w", err)
		}
		for _, radio := range configuration.WiFi {
			if _, err := tx.Exec(ctx, `
				INSERT INTO acs.router_wifi_radios
					(tenant_id, router_id, band, ssid, encrypted_password, security_mode, channel, channel_mode, bandwidth_mhz, enabled)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
				ON CONFLICT(router_id,band) DO UPDATE SET ssid=EXCLUDED.ssid,encrypted_password=EXCLUDED.encrypted_password,
				security_mode=EXCLUDED.security_mode,channel=EXCLUDED.channel,channel_mode=EXCLUDED.channel_mode,
				bandwidth_mhz=EXCLUDED.bandwidth_mhz,enabled=EXCLUDED.enabled`, tenantID, routerID, radio.Band, radio.SSID,
				radio.EncryptedPassword, radio.SecurityMode, radio.Channel, radio.ChannelMode, radio.BandwidthMHz, radio.Enabled); err != nil {
				return fmt.Errorf("save Wi-Fi radio: %w", err)
			}
		}
		if configuration.BandSteeringEnabled != nil {
			if _, err := tx.Exec(ctx, `UPDATE acs.routers SET band_steering_enabled=$3 WHERE tenant_id=$1 AND id=$2`, tenantID, routerID, *configuration.BandSteeringEnabled); err != nil {
				return fmt.Errorf("save band steering configuration: %w", err)
			}
		}
		if configuration.Network == nil {
			return tx.QueryRow(ctx, `UPDATE acs.routers SET configuration_version = configuration_version + 1,configuration_pending=true WHERE id = $1 RETURNING configuration_version`, routerID).Scan(&result.Version)
		}
		network := *configuration.Network
		if _, err := tx.Exec(ctx, `
			INSERT INTO acs.router_network_settings
				(tenant_id,router_id,dhcp_enabled,dhcp_start,dhcp_end,dhcp_gateway,dhcp_subnet_mask,dhcp_lease_seconds,dns_primary,dns_secondary)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (router_id) DO UPDATE SET dhcp_enabled=EXCLUDED.dhcp_enabled,
				dhcp_start=EXCLUDED.dhcp_start,dhcp_end=EXCLUDED.dhcp_end,dhcp_gateway=EXCLUDED.dhcp_gateway,dhcp_subnet_mask=EXCLUDED.dhcp_subnet_mask,
				dhcp_lease_seconds=EXCLUDED.dhcp_lease_seconds, dns_primary=EXCLUDED.dns_primary,
				dns_secondary=EXCLUDED.dns_secondary`, tenantID, routerID, network.DHCPEnabled,
			network.DHCPStart, network.DHCPEnd, network.DHCPGateway, network.DHCPSubnetMask, network.DHCPLeaseSeconds, network.DNSPrimary, network.DNSSecondary); err != nil {
			return fmt.Errorf("save network configuration: %w", err)
		}
		return tx.QueryRow(ctx, `UPDATE acs.routers SET configuration_version = configuration_version + 1,configuration_pending=true WHERE id = $1 RETURNING configuration_version`, routerID).Scan(&result.Version)
	})
	return result, err
}

func (r *Repository) RevertConfiguration(ctx context.Context, tenantID, routerID string) error {
	return database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `UPDATE acs.routers SET configuration_pending=false WHERE tenant_id=$1 AND id=$2`, tenantID, routerID)
		if err != nil {
			return fmt.Errorf("revert router configuration: %w", err)
		}
		if result.RowsAffected() == 0 {
			return ErrNotFound
		}
		// Desired rows are discarded so the UI and the next edit start from the
		// latest values collected from the CPE instead of stale modifications.
		if _, err := tx.Exec(ctx, `DELETE FROM acs.router_wifi_radios WHERE tenant_id=$1 AND router_id=$2`, tenantID, routerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM acs.router_network_settings WHERE tenant_id=$1 AND router_id=$2`, tenantID, routerID); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) CreateActivationCommand(ctx context.Context, tenantID, routerID, userID, idempotencyKey, scope string, overrides ActivationOverrides) (Command, error) {
	var command Command
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var version int64
		var bandSteeringEnabled *bool
		if err := tx.QueryRow(ctx, `SELECT configuration_version,band_steering_enabled FROM acs.routers WHERE tenant_id=$1 AND id=$2`, tenantID, routerID).Scan(&version, &bandSteeringEnabled); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("find router: %w", err)
		}
		if version == 0 {
			return ErrConfigurationRequired
		}
		hasDNSOverrides := scope == "DNS" && (overrides.DNSPrimary != nil || overrides.DNSSecondary != nil)
		hasDHCPOverrides := scope == "DHCP" && (overrides.DHCPGateway != nil || overrides.DHCPSubnetMask != nil)
		if hasDNSOverrides || hasDHCPOverrides {
			if _, err := tx.Exec(ctx, `UPDATE acs.router_network_settings SET dns_primary=COALESCE($3::inet,dns_primary),dns_secondary=COALESCE($4::inet,dns_secondary),dhcp_gateway=COALESCE($5::inet,dhcp_gateway),dhcp_subnet_mask=COALESCE($6::inet,dhcp_subnet_mask) WHERE tenant_id=$1 AND router_id=$2`, tenantID, routerID, overrides.DNSPrimary, overrides.DNSSecondary, overrides.DHCPGateway, overrides.DHCPSubnetMask); err != nil {
				return fmt.Errorf("update desired network overrides: %w", err)
			}
			if err := tx.QueryRow(ctx, `UPDATE acs.routers SET configuration_version=configuration_version+1 WHERE tenant_id=$1 AND id=$2 RETURNING configuration_version`, tenantID, routerID).Scan(&version); err != nil {
				return fmt.Errorf("version desired DNS: %w", err)
			}
		}
		wifi := make([]map[string]any, 0)
		rows, err := tx.Query(ctx, `SELECT band, ssid, encrypted_password, security_mode, channel, channel_mode, bandwidth_mhz, enabled FROM acs.router_wifi_radios WHERE tenant_id=$1 AND router_id=$2 ORDER BY band`, tenantID, routerID)
		if err != nil {
			return fmt.Errorf("load Wi-Fi configuration: %w", err)
		}
		for rows.Next() {
			var band, ssid, securityMode, channelMode string
			var encrypted []byte
			var channel, bandwidth *int
			var enabled bool
			if err := rows.Scan(&band, &ssid, &encrypted, &securityMode, &channel, &channelMode, &bandwidth, &enabled); err != nil {
				rows.Close()
				return fmt.Errorf("scan Wi-Fi configuration: %w", err)
			}
			wifi = append(wifi, map[string]any{"band": band, "ssid": ssid, "encrypted_password": base64.StdEncoding.EncodeToString(encrypted), "security_mode": securityMode, "channel": channel, "channel_mode": channelMode, "bandwidth_mhz": bandwidth, "enabled": enabled})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate Wi-Fi configuration: %w", err)
		}
		var dhcpEnabled bool
		var dhcpStart, dhcpEnd, dhcpGateway, dhcpSubnetMask, dnsPrimary, dnsSecondary *string
		var lease *int
		hasNetwork := true
		if err := tx.QueryRow(ctx, `SELECT host(dhcp_start),host(dhcp_end),host(dhcp_gateway),host(dhcp_subnet_mask),dhcp_enabled,dhcp_lease_seconds,host(dns_primary),host(dns_secondary) FROM acs.router_network_settings WHERE tenant_id=$1 AND router_id=$2`, tenantID, routerID).Scan(&dhcpStart, &dhcpEnd, &dhcpGateway, &dhcpSubnetMask, &dhcpEnabled, &lease, &dnsPrimary, &dnsSecondary); errors.Is(err, pgx.ErrNoRows) {
			hasNetwork = false
		} else if err != nil {
			return fmt.Errorf("load network configuration: %w", err)
		}
		if scope == "WIFI_24" || scope == "WIFI_5" {
			wanted := "2.4GHZ"
			if scope == "WIFI_5" {
				wanted = "5GHZ"
			}
			found := false
			for _, radio := range wifi {
				if radio["band"] == wanted {
					found = true
					break
				}
			}
			if !found {
				return ErrScopeConfigurationRequired
			}
		}
		if scope == "DHCP" && (!hasNetwork || dhcpStart == nil || dhcpEnd == nil || dhcpGateway == nil || dhcpSubnetMask == nil) {
			return ErrScopeConfigurationRequired
		}
		if scope == "DNS" && (!hasNetwork || (dnsPrimary == nil && dnsSecondary == nil)) {
			return ErrScopeConfigurationRequired
		}
		var network any
		if hasNetwork {
			network = map[string]any{"dhcp_enabled": dhcpEnabled, "dhcp_start": dhcpStart, "dhcp_end": dhcpEnd, "dhcp_gateway": dhcpGateway, "dhcp_subnet_mask": dhcpSubnetMask, "dhcp_lease_seconds": lease, "dns_primary": dnsPrimary, "dns_secondary": dnsSecondary}
		}
		if len(wifi) == 0 && !hasNetwork {
			return ErrConfigurationRequired
		}
		parameters := map[string]any{"configuration_version": version, "scope": scope, "configuration": map[string]any{"wifi": wifi, "network": network, "band_steering_enabled": bandSteeringEnabled}}
		encodedParameters, _ := json.Marshal(parameters)
		var key any
		if idempotencyKey != "" {
			key = idempotencyKey
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO acs.router_commands (tenant_id, router_id, created_by, type, parameters, idempotency_key)
			VALUES ($1,$2,$3,'ACTIVATE',$4,$5)
			ON CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL
			DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key
			RETURNING id, router_id, type, status, priority, parameters, idempotency_key`,
			tenantID, routerID, userID, encodedParameters, key).Scan(&command.ID, &command.RouterID, &command.Type,
			&command.Status, &command.Priority, &encodedParameters, &command.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("create activation command: %w", err)
		}
		if err := json.Unmarshal(encodedParameters, &command.Parameters); err != nil {
			return err
		}
		command.Parameters = map[string]any{"configuration_version": version, "scope": scope}
		return nil
	})
	return command, err
}
