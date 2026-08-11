package cwmp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoCommand = errors.New("no command available")

type Decryptor interface {
	Decrypt([]byte) (string, error)
}

type Dispatcher struct {
	pool       *pgxpool.Pool
	tenantSlug string
	decryptor  Decryptor
}

func NewDispatcher(pool *pgxpool.Pool, tenantSlug string, decryptor Decryptor) *Dispatcher {
	return &Dispatcher{pool: pool, tenantSlug: tenantSlug, decryptor: decryptor}
}

type activationSnapshot struct {
	WiFi []struct {
		Band              string `json:"band"`
		SSID              string `json:"ssid"`
		EncryptedPassword string `json:"encrypted_password"`
		SecurityMode      string `json:"security_mode"`
		Channel           *int   `json:"channel"`
		ChannelMode       string `json:"channel_mode"`
		BandwidthMHz      *int   `json:"bandwidth_mhz"`
		Enabled           bool   `json:"enabled"`
	} `json:"wifi"`
	Network struct {
		DHCPEnabled      bool    `json:"dhcp_enabled"`
		DHCPStart        *string `json:"dhcp_start"`
		DHCPEnd          *string `json:"dhcp_end"`
		DHCPLeaseSeconds *int    `json:"dhcp_lease_seconds"`
		DNSPrimary       *string `json:"dns_primary"`
		DNSSecondary     *string `json:"dns_secondary"`
	} `json:"network"`
}

type commandParameters struct {
	ConfigurationVersion int64              `json:"configuration_version"`
	Configuration        activationSnapshot `json:"configuration"`
}

func (d *Dispatcher) Next(ctx context.Context, sessionID string) (OutboundCommand, error) {
	tenantID, err := d.tenantID(ctx)
	if err != nil {
		return OutboundCommand{}, err
	}
	var outbound OutboundCommand
	err = database.InTenantTransaction(ctx, d.pool, tenantID, func(tx pgx.Tx) error {
		var routerID string
		if err := tx.QueryRow(ctx, `SELECT router_id FROM acs.router_sessions WHERE tenant_id=$1 AND id=$2 AND status='OPEN' AND router_id IS NOT NULL`, tenantID, sessionID).Scan(&routerID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCommand
		} else if err != nil {
			return fmt.Errorf("find CWMP session: %w", err)
		}
		var raw []byte
		var attemptNumber int
		if err := tx.QueryRow(ctx, `
			SELECT id, type, parameters, attempts_count + 1
			FROM acs.router_commands
			WHERE tenant_id=$1 AND router_id=$2
			  AND status IN ('PENDING','WAITING_DEVICE','RETRY_SCHEDULED')
			  AND available_at <= now() AND attempts_count < max_attempts
			ORDER BY CASE priority WHEN 'CRITICAL' THEN 1 WHEN 'HIGH' THEN 2 WHEN 'NORMAL' THEN 3 ELSE 4 END, created_at
			FOR UPDATE SKIP LOCKED LIMIT 1`, tenantID, routerID).Scan(&outbound.ID, &outbound.Type, &raw, &attemptNumber); errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCommand
		} else if err != nil {
			return fmt.Errorf("claim command: %w", err)
		}
		if outbound.Type != "ACTIVATE" {
			return fmt.Errorf("unsupported command type %s", outbound.Type)
		}
		var parameters commandParameters
		if err := json.Unmarshal(raw, &parameters); err != nil {
			return fmt.Errorf("decode command parameters: %w", err)
		}
		mapped, err := d.mapActivation(parameters.Configuration)
		if err != nil {
			return err
		}
		outbound.Parameters = mapped
		if err := tx.QueryRow(ctx, `
			INSERT INTO acs.router_command_attempts (tenant_id, router_command_id, session_id, attempt_number, request_payload)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`, tenantID, outbound.ID, sessionID, attemptNumber, raw).Scan(&outbound.AttemptID); err != nil {
			return fmt.Errorf("create command attempt: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='PROCESSING', attempts_count=$2, started_at=COALESCE(started_at,now()) WHERE id=$1`, outbound.ID, attemptNumber); err != nil {
			return fmt.Errorf("start command: %w", err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,from_status,to_status,reason,metadata) VALUES ($1,$2,NULL,'PROCESSING','Delivered in CWMP session',jsonb_build_object('session_id',$3::text,'attempt_id',$4::text))`, tenantID, outbound.ID, sessionID, outbound.AttemptID)
		return err
	})
	return outbound, err
}

func (d *Dispatcher) Complete(ctx context.Context, sessionID string, result CommandResult) error {
	tenantID, err := d.tenantID(ctx)
	if err != nil {
		return err
	}
	return database.InTenantTransaction(ctx, d.pool, tenantID, func(tx pgx.Tx) error {
		var attemptID, commandID, routerID string
		if err := tx.QueryRow(ctx, `
			SELECT a.id, a.router_command_id, c.router_id
			FROM acs.router_command_attempts a JOIN acs.router_commands c ON c.id=a.router_command_id
			WHERE a.tenant_id=$1 AND a.session_id=$2 AND a.status='PROCESSING'
			ORDER BY a.started_at DESC LIMIT 1 FOR UPDATE OF a,c`, tenantID, sessionID).Scan(&attemptID, &commandID, &routerID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCommand
		} else if err != nil {
			return fmt.Errorf("find active attempt: %w", err)
		}
		response, _ := json.Marshal(map[string]string{"xml": result.Raw})
		if result.Fault {
			if _, err := tx.Exec(ctx, `UPDATE acs.router_command_attempts SET status='FAILED',response_payload=$2,error_code=$3,error_message=$4,finished_at=now(),duration_ms=round(EXTRACT(EPOCH FROM(now()-started_at))*1000)::int WHERE id=$1`, attemptID, response, result.FaultCode, result.FaultString); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='FAILED',error_code=$2,error_message=$3,completed_at=now() WHERE id=$1`, commandID, result.FaultCode, result.FaultString); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,from_status,to_status,reason) VALUES ($1,$2,'PROCESSING','FAILED',$3)`, tenantID, commandID, result.FaultString)
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.router_command_attempts SET status='COMPLETED',response_payload=$2,finished_at=now(),duration_ms=round(EXTRACT(EPOCH FROM(now()-started_at))*1000)::int WHERE id=$1`, attemptID, response); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='COMPLETED',completed_at=now(),result='{"applied":true}'::jsonb WHERE id=$1`, commandID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.routers SET desired_status='ACTIVE',reported_status='ACTIVE' WHERE id=$1`, routerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,from_status,to_status,reason) VALUES ($1,$2,'PROCESSING','COMPLETED','CPE acknowledged SetParameterValues')`, tenantID, commandID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE acs.router_sessions SET status='COMPLETED',finished_at=now() WHERE id=$1`, sessionID)
		return err
	})
}

func (d *Dispatcher) tenantID(ctx context.Context) (string, error) {
	var tenantID string
	if err := d.pool.QueryRow(ctx, `SELECT id FROM public.tenants WHERE slug=$1 AND active`, d.tenantSlug).Scan(&tenantID); err != nil {
		return "", fmt.Errorf("resolve ACS tenant: %w", err)
	}
	return tenantID, nil
}

func (d *Dispatcher) mapActivation(snapshot activationSnapshot) ([]ParameterValue, error) {
	values := make([]ParameterValue, 0, len(snapshot.WiFi)*6+6)
	for _, radio := range snapshot.WiFi {
		index := "1"
		if radio.Band == "5GHZ" {
			index = "2"
		}
		encrypted, err := base64.StdEncoding.DecodeString(radio.EncryptedPassword)
		if err != nil {
			return nil, fmt.Errorf("decode Wi-Fi password: %w", err)
		}
		password, err := d.decryptor.Decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt Wi-Fi password: %w", err)
		}
		values = append(values,
			ParameterValue{"Device.WiFi.Radio." + index + ".Enable", boolean(radio.Enabled), "boolean"},
			ParameterValue{"Device.WiFi.SSID." + index + ".SSID", radio.SSID, "string"},
			ParameterValue{"Device.WiFi.AccessPoint." + index + ".Security.ModeEnabled", tr181Security(radio.SecurityMode), "string"},
			ParameterValue{"Device.WiFi.AccessPoint." + index + ".Security.KeyPassphrase", password, "string"})
		if radio.Channel != nil {
			values = append(values, ParameterValue{"Device.WiFi.Radio." + index + ".Channel", strconv.Itoa(*radio.Channel), "unsignedInt"})
		}
		if radio.BandwidthMHz != nil {
			values = append(values, ParameterValue{"Device.WiFi.Radio." + index + ".OperatingChannelBandwidth", strconv.Itoa(*radio.BandwidthMHz) + "MHz", "string"})
		}
	}
	network := snapshot.Network
	values = append(values, ParameterValue{"Device.DHCPv4.Server.Pool.1.Enable", boolean(network.DHCPEnabled), "boolean"})
	if network.DHCPStart != nil {
		values = append(values, ParameterValue{"Device.DHCPv4.Server.Pool.1.MinAddress", *network.DHCPStart, "string"})
	}
	if network.DHCPEnd != nil {
		values = append(values, ParameterValue{"Device.DHCPv4.Server.Pool.1.MaxAddress", *network.DHCPEnd, "string"})
	}
	if network.DHCPLeaseSeconds != nil {
		values = append(values, ParameterValue{"Device.DHCPv4.Server.Pool.1.LeaseTime", strconv.Itoa(*network.DHCPLeaseSeconds), "int"})
	}
	if network.DNSPrimary != nil {
		values = append(values, ParameterValue{"Device.DNS.Client.Server.1.DNSServer", *network.DNSPrimary, "string"})
	}
	if network.DNSSecondary != nil {
		values = append(values, ParameterValue{"Device.DNS.Client.Server.2.DNSServer", *network.DNSSecondary, "string"})
	}
	return values, nil
}

func boolean(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
func tr181Security(value string) string {
	switch value {
	case "WPA3_SAE":
		return "WPA3-Personal"
	case "WPA2_WPA3":
		return "WPA2-WPA3-Personal"
	default:
		return "WPA2-Personal"
	}
}
