package router

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
)

type scanner interface{ Scan(...any) error }

func (r *Repository) ListRouters(ctx context.Context, tenantID string, filter ListFilter) ([]Summary, error) {
	items := make([]Summary, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id,name,manufacturer,oui,product_class,serial_number,software_version,hardware_version,
			       desired_status,reported_status,first_seen_at,last_seen_at,created_at,last_inform_at,
			       host(last_ip_address),configuration_version,updated_at
			FROM acs.routers
			WHERE tenant_id=$1
			  AND ($2='' OR strpos(lower(name||' '||serial_number||' '||oui||' '||product_class),lower($2))>0)
			  AND ($3='' OR desired_status=$3)
			  AND ($4='' OR
			      ($4='NEVER_CONNECTED' AND last_seen_at IS NULL) OR
			      ($4='ONLINE' AND last_seen_at >= $5) OR
			      ($4='STALE' AND last_seen_at < $5 AND last_seen_at >= $6) OR
			      ($4='OFFLINE' AND last_seen_at < $6))
			ORDER BY name,id LIMIT $7 OFFSET $8`, tenantID, filter.Search, filter.DesiredStatus,
			filter.ConnectivityStatus, filter.OnlineCutoff, filter.OfflineCutoff, filter.Limit, filter.Offset)
		if err != nil {
			return fmt.Errorf("list routers: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item Summary
			if err := scanSummary(rows, &item); err != nil {
				return fmt.Errorf("scan router: %w", err)
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) GetRouter(ctx context.Context, tenantID, routerID string) (Detail, error) {
	detail := Detail{WiFi: make([]WiFiView, 0), Sessions: make([]Session, 0), Commands: make([]RecentCommand, 0)}
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id,name,manufacturer,oui,product_class,serial_number,software_version,hardware_version,
			       desired_status,reported_status,first_seen_at,last_seen_at,created_at,last_inform_at,
			       host(last_ip_address),configuration_version,updated_at
			FROM acs.routers WHERE tenant_id=$1 AND id=$2`, tenantID, routerID)
		if err := scanSummary(row, &detail.Summary); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("get router: %w", err)
		}

		wifiRows, err := tx.Query(ctx, `SELECT band,ssid,length(encrypted_password)>0,security_mode,channel,channel_mode,bandwidth_mhz,enabled FROM acs.router_wifi_radios WHERE tenant_id=$1 AND router_id=$2 ORDER BY band`, tenantID, routerID)
		if err != nil {
			return fmt.Errorf("list router Wi-Fi: %w", err)
		}
		for wifiRows.Next() {
			var radio WiFiView
			if err := wifiRows.Scan(&radio.Band, &radio.SSID, &radio.PasswordConfigured, &radio.SecurityMode, &radio.Channel, &radio.ChannelMode, &radio.BandwidthMHz, &radio.Enabled); err != nil {
				wifiRows.Close()
				return err
			}
			detail.WiFi = append(detail.WiFi, radio)
		}
		if err := wifiRows.Err(); err != nil {
			wifiRows.Close()
			return err
		}
		wifiRows.Close()

		var network NetworkConfiguration
		err = tx.QueryRow(ctx, `SELECT dhcp_enabled,host(dhcp_start),host(dhcp_end),dhcp_lease_seconds,host(dns_primary),host(dns_secondary) FROM acs.router_network_settings WHERE tenant_id=$1 AND router_id=$2`, tenantID, routerID).Scan(&network.DHCPEnabled, &network.DHCPStart, &network.DHCPEnd, &network.DHCPLeaseSeconds, &network.DNSPrimary, &network.DNSSecondary)
		if err == nil {
			detail.Network = &network
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get router network: %w", err)
		}

		sessionRows, err := tx.Query(ctx, `SELECT id,protocol,host(ip_address),user_agent,status,started_at,finished_at FROM acs.router_sessions WHERE tenant_id=$1 AND router_id=$2 ORDER BY started_at DESC LIMIT 20`, tenantID, routerID)
		if err != nil {
			return fmt.Errorf("list router sessions: %w", err)
		}
		for sessionRows.Next() {
			var session Session
			if err := sessionRows.Scan(&session.ID, &session.Protocol, &session.IPAddress, &session.UserAgent, &session.Status, &session.StartedAt, &session.FinishedAt); err != nil {
				sessionRows.Close()
				return err
			}
			detail.Sessions = append(detail.Sessions, session)
		}
		if err := sessionRows.Err(); err != nil {
			sessionRows.Close()
			return err
		}
		sessionRows.Close()

		commandRows, err := tx.Query(ctx, `SELECT id,type,status,attempts_count,created_at,completed_at FROM acs.router_commands WHERE tenant_id=$1 AND router_id=$2 ORDER BY created_at DESC LIMIT 20`, tenantID, routerID)
		if err != nil {
			return fmt.Errorf("list router commands: %w", err)
		}
		defer commandRows.Close()
		for commandRows.Next() {
			var command RecentCommand
			if err := commandRows.Scan(&command.ID, &command.Type, &command.Status, &command.AttemptsCount, &command.CreatedAt, &command.CompletedAt); err != nil {
				return err
			}
			detail.Commands = append(detail.Commands, command)
		}
		return commandRows.Err()
	})
	return detail, err
}

func scanSummary(row scanner, item *Summary) error {
	return row.Scan(&item.ID, &item.Name, &item.Manufacturer, &item.OUI, &item.ProductClass, &item.SerialNumber,
		&item.SoftwareVersion, &item.HardwareVersion, &item.DesiredStatus, &item.ReportedStatus, &item.FirstSeenAt,
		&item.LastSeenAt, &item.CreatedAt, &item.LastInformAt, &item.LastIPAddress, &item.ConfigurationVersion, &item.UpdatedAt)
}

func ConnectivityStatus(lastSeen *time.Time, now time.Time, onlineTimeout, offlineTimeout time.Duration) string {
	if lastSeen == nil {
		return "NEVER_CONNECTED"
	}
	age := now.Sub(*lastSeen)
	if age < onlineTimeout {
		return "ONLINE"
	}
	if age < offlineTimeout {
		return "STALE"
	}
	return "OFFLINE"
}
