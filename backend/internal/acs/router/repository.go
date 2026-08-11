package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound              = errors.New("router not found")
	ErrConfigurationRequired = errors.New("router configuration is required")
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

func (r *Repository) Configure(ctx context.Context, tenantID, unknownID, name string) (Router, error) {
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

		err = tx.QueryRow(ctx, `
			INSERT INTO acs.routers
				(tenant_id, name, manufacturer, oui, product_class, serial_number, software_version,
				 hardware_version, first_seen_at, last_seen_at, last_inform_at, last_ip_address)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11)
			RETURNING id, name, manufacturer, oui, product_class, serial_number, software_version,
			          hardware_version, desired_status, reported_status, first_seen_at, last_seen_at, created_at`,
			tenantID, name, unknown.Manufacturer, unknown.OUI, unknown.ProductClass, unknown.SerialNumber,
			unknown.SoftwareVersion, unknown.HardwareVersion, unknown.FirstSeenAt, unknown.LastSeenAt,
			unknown.LastIPAddress).Scan(&configured.ID, &configured.Name, &configured.Manufacturer,
			&configured.OUI, &configured.ProductClass, &configured.SerialNumber, &configured.SoftwareVersion,
			&configured.HardwareVersion, &configured.DesiredStatus, &configured.ReportedStatus,
			&configured.FirstSeenAt, &configured.LastSeenAt, &configured.CreatedAt)
		if err != nil {
			return fmt.Errorf("create configured router: %w", err)
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
		if _, err := tx.Exec(ctx, `DELETE FROM acs.router_wifi_radios WHERE tenant_id = $1 AND router_id = $2`, tenantID, routerID); err != nil {
			return fmt.Errorf("replace Wi-Fi configuration: %w", err)
		}
		for _, radio := range configuration.WiFi {
			if _, err := tx.Exec(ctx, `
				INSERT INTO acs.router_wifi_radios
					(tenant_id, router_id, band, ssid, encrypted_password, security_mode, channel, channel_mode, bandwidth_mhz, enabled)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, tenantID, routerID, radio.Band, radio.SSID,
				radio.EncryptedPassword, radio.SecurityMode, radio.Channel, radio.ChannelMode, radio.BandwidthMHz, radio.Enabled); err != nil {
				return fmt.Errorf("save Wi-Fi radio: %w", err)
			}
		}
		network := configuration.Network
		if _, err := tx.Exec(ctx, `
			INSERT INTO acs.router_network_settings
				(tenant_id, router_id, dhcp_enabled, dhcp_start, dhcp_end, dhcp_lease_seconds, dns_primary, dns_secondary)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (router_id) DO UPDATE SET dhcp_enabled=EXCLUDED.dhcp_enabled,
				dhcp_start=EXCLUDED.dhcp_start, dhcp_end=EXCLUDED.dhcp_end,
				dhcp_lease_seconds=EXCLUDED.dhcp_lease_seconds, dns_primary=EXCLUDED.dns_primary,
				dns_secondary=EXCLUDED.dns_secondary`, tenantID, routerID, network.DHCPEnabled,
			network.DHCPStart, network.DHCPEnd, network.DHCPLeaseSeconds, network.DNSPrimary, network.DNSSecondary); err != nil {
			return fmt.Errorf("save network configuration: %w", err)
		}
		return tx.QueryRow(ctx, `UPDATE acs.routers SET configuration_version = configuration_version + 1 WHERE id = $1 RETURNING configuration_version`, routerID).Scan(&result.Version)
	})
	return result, err
}

func (r *Repository) CreateActivationCommand(ctx context.Context, tenantID, routerID, userID, idempotencyKey string) (Command, error) {
	var command Command
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var version int64
		if err := tx.QueryRow(ctx, `SELECT configuration_version FROM acs.routers WHERE tenant_id=$1 AND id=$2`, tenantID, routerID).Scan(&version); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("find router: %w", err)
		}
		if version == 0 {
			return ErrConfigurationRequired
		}
		parameters := map[string]any{"configuration_version": version}
		encodedParameters, _ := json.Marshal(parameters)
		var key any
		if idempotencyKey != "" {
			key = idempotencyKey
		}
		err := tx.QueryRow(ctx, `
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
		return json.Unmarshal(encodedParameters, &command.Parameters)
	})
	return command, err
}
