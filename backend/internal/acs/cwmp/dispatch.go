package cwmp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
	acsBaseURL string
}

func NewDispatcher(pool *pgxpool.Pool, tenantSlug string, decryptor Decryptor, acsBaseURL string) *Dispatcher {
	return &Dispatcher{pool: pool, tenantSlug: tenantSlug, decryptor: decryptor, acsBaseURL: acsBaseURL}
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
		DHCPGateway      *string `json:"dhcp_gateway"`
		DHCPSubnetMask   *string `json:"dhcp_subnet_mask"`
		DHCPLeaseSeconds *int    `json:"dhcp_lease_seconds"`
		DNSPrimary       *string `json:"dns_primary"`
		DNSSecondary     *string `json:"dns_secondary"`
	} `json:"network"`
}

type commandParameters struct {
	ConfigurationVersion int64              `json:"configuration_version"`
	Scope                string             `json:"scope"`
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
		var routerModelID *string
		if err := tx.QueryRow(ctx, `SELECT s.router_id,r.router_model_id FROM acs.router_sessions s JOIN acs.routers r ON r.id=s.router_id AND r.tenant_id=s.tenant_id WHERE s.tenant_id=$1 AND s.id=$2 AND s.status='OPEN'`, tenantID, sessionID).Scan(&routerID, &routerModelID); errors.Is(err, pgx.ErrNoRows) {
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
		if outbound.Type != "ACTIVATE" && outbound.Type != "UPDATE_FIRMWARE" {
			return fmt.Errorf("unsupported command type %s", outbound.Type)
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO acs.router_command_attempts (tenant_id, router_command_id, session_id, attempt_number, request_payload)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`, tenantID, outbound.ID, sessionID, attemptNumber, raw).Scan(&outbound.AttemptID); err != nil {
			return fmt.Errorf("create command attempt: %w", err)
		}
		if outbound.Type == "ACTIVATE" {
			var parameters commandParameters
			if err := json.Unmarshal(raw, &parameters); err != nil {
				return fmt.Errorf("decode command parameters: %w", err)
			}
			if routerModelID == nil {
				return errors.New("router has no model assigned for CWMP parameter mapping")
			}
			mappings, err := loadParameterMappings(ctx, tx, tenantID, *routerModelID)
			if err != nil {
				return err
			}
			mapped, err := d.mapActivation(parameters.Configuration, parameters.Scope, mappings)
			if err != nil {
				return err
			}
			outbound.Parameters = mapped
		} else {
			var parameters struct {
				FirmwareID string `json:"firmware_id"`
			}
			if err := json.Unmarshal(raw, &parameters); err != nil {
				return fmt.Errorf("decode firmware parameters: %w", err)
			}
			var size int64
			var name string
			if err := tx.QueryRow(ctx, `SELECT file_size,file_name FROM acs.firmwares WHERE tenant_id=$1 AND id=$2 AND active AND status='READY'`, tenantID, parameters.FirmwareID).Scan(&size, &name); errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("firmware is no longer available")
			} else if err != nil {
				return err
			}
			outbound.Download = &DownloadRequest{URL: fmt.Sprintf("%s/firmwares/%s/download?token=%s", d.acsBaseURL, parameters.FirmwareID, outbound.AttemptID), FileSize: size, FileName: name}
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='PROCESSING', attempts_count=$2, started_at=COALESCE(started_at,now()), last_attempt_at=now(), next_attempt_at=NULL WHERE id=$1`, outbound.ID, attemptNumber); err != nil {
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
		if result.Kind == "TRANSFER_COMPLETE" {
			return d.completeTransfer(ctx, tx, tenantID, sessionID, result)
		}
		var attemptID, commandID, routerID, commandType string
		if err := tx.QueryRow(ctx, `
			SELECT a.id, a.router_command_id, c.router_id, c.type
			FROM acs.router_command_attempts a JOIN acs.router_commands c ON c.id=a.router_command_id
			WHERE a.tenant_id=$1 AND a.session_id=$2 AND a.status='PROCESSING'
			ORDER BY a.started_at DESC LIMIT 1 FOR UPDATE OF a,c`, tenantID, sessionID).Scan(&attemptID, &commandID, &routerID, &commandType); errors.Is(err, pgx.ErrNoRows) {
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
		if result.Kind == "DOWNLOAD_ACCEPTED" {
			if _, err := tx.Exec(ctx, `UPDATE acs.router_command_attempts SET status='COMPLETED',response_payload=$2,finished_at=now(),duration_ms=round(EXTRACT(EPOCH FROM(now()-started_at))*1000)::int WHERE id=$1`, attemptID, response); err != nil {
				return err
			}
			if result.ResponseStatus == "0" {
				if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='COMPLETED',completed_at=now(),result=jsonb_build_object('downloaded',true,'immediate',true) WHERE id=$1`, commandID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE acs.routers r SET software_version=f.version FROM acs.router_commands c JOIN acs.firmwares f ON f.id=(c.parameters->>'firmware_id')::uuid WHERE r.id=$1 AND c.id=$2`, routerID, commandID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events(tenant_id,router_command_id,from_status,to_status,reason) VALUES($1,$2,'PROCESSING','COMPLETED','CPE completed firmware download synchronously')`, tenantID, commandID); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `UPDATE acs.router_sessions SET status='COMPLETED',finished_at=now() WHERE id=$1`, sessionID)
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events(tenant_id,router_command_id,from_status,to_status,reason) VALUES($1,$2,'PROCESSING','PROCESSING','CPE accepted firmware download')`, tenantID, commandID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE acs.router_sessions SET status='COMPLETED',finished_at=now() WHERE id=$1`, sessionID)
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.router_command_attempts SET status='COMPLETED',response_payload=$2,finished_at=now(),duration_ms=round(EXTRACT(EPOCH FROM(now()-started_at))*1000)::int WHERE id=$1`, attemptID, response); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='COMPLETED',completed_at=now(),result='{"applied":true}'::jsonb WHERE id=$1`, commandID); err != nil {
			return err
		}
		if commandType == "ACTIVATE" {
			if _, err := tx.Exec(ctx, `UPDATE acs.routers SET desired_status='ACTIVE',reported_status='ACTIVE' WHERE id=$1`, routerID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,from_status,to_status,reason) VALUES ($1,$2,'PROCESSING','COMPLETED','CPE acknowledged SetParameterValues')`, tenantID, commandID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE acs.router_sessions SET status='COMPLETED',finished_at=now() WHERE id=$1`, sessionID)
		return err
	})
}

func (d *Dispatcher) completeTransfer(ctx context.Context, tx pgx.Tx, tenantID, sessionID string, result CommandResult) error {
	var commandID, routerID, firmwareID string
	err := tx.QueryRow(ctx, `SELECT c.id,c.router_id,c.parameters->>'firmware_id' FROM acs.router_commands c WHERE c.tenant_id=$1 AND c.id=$2 AND c.type='UPDATE_FIRMWARE' AND c.status='PROCESSING' FOR UPDATE`, tenantID, result.CommandKey).Scan(&commandID, &routerID, &firmwareID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoCommand
	}
	if err != nil {
		return err
	}
	if result.Fault {
		if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='FAILED',error_code=$2,error_message=$3,completed_at=now() WHERE id=$1`, commandID, result.TransferCode, result.FaultString); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events(tenant_id,router_command_id,from_status,to_status,reason) VALUES($1,$2,'PROCESSING','FAILED',$3)`, tenantID, commandID, result.FaultString); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='COMPLETED',completed_at=now(),result=jsonb_build_object('downloaded',true,'firmware_id',$2::text) WHERE id=$1`, commandID, firmwareID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE acs.routers r SET software_version=f.version FROM acs.firmwares f WHERE r.id=$1 AND f.id=$2`, routerID, firmwareID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events(tenant_id,router_command_id,from_status,to_status,reason) VALUES($1,$2,'PROCESSING','COMPLETED','CPE reported successful firmware transfer')`, tenantID, commandID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE acs.router_sessions SET status='COMPLETED',finished_at=now() WHERE id=$1`, sessionID)
	return err
}

func (d *Dispatcher) tenantID(ctx context.Context) (string, error) {
	var tenantID string
	if err := d.pool.QueryRow(ctx, `SELECT id FROM public.tenants WHERE slug=$1 AND active`, d.tenantSlug).Scan(&tenantID); err != nil {
		return "", fmt.Errorf("resolve ACS tenant: %w", err)
	}
	return tenantID, nil
}

type parameterMapping struct {
	Name string
	Type string
}

func loadParameterMappings(ctx context.Context, tx pgx.Tx, tenantID, modelID string) (map[string]parameterMapping, error) {
	rows, err := tx.Query(ctx, `SELECT parameter_key,parameter_name,xsd_type FROM acs.router_model_cwmp_parameters WHERE tenant_id=$1 AND router_model_id=$2 AND active`, tenantID, modelID)
	if err != nil {
		return nil, fmt.Errorf("load model CWMP parameters: %w", err)
	}
	defer rows.Close()
	mappings := make(map[string]parameterMapping)
	for rows.Next() {
		var key string
		var mapping parameterMapping
		if err := rows.Scan(&key, &mapping.Name, &mapping.Type); err != nil {
			return nil, err
		}
		mappings[key] = mapping
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(mappings) == 0 {
		return nil, errors.New("router model has no active CWMP parameter mappings")
	}
	return mappings, nil
}

func (d *Dispatcher) mapActivation(snapshot activationSnapshot, scope string, mappings map[string]parameterMapping) ([]ParameterValue, error) {
	if scope == "" {
		scope = "ALL"
	}
	values := make([]ParameterValue, 0, len(snapshot.WiFi)*6+6)
	appendValue := func(key, value string) error {
		mapping, ok := mappings[key]
		if !ok {
			return fmt.Errorf("CWMP parameter mapping %q is not configured for router model", key)
		}
		values = append(values, ParameterValue{Name: mapping.Name, Value: value, Type: mapping.Type})
		return nil
	}
	for _, radio := range snapshot.WiFi {
		bandKey := "wifi.24"
		if radio.Band == "5GHZ" {
			bandKey = "wifi.5"
		}
		if (scope == "WIFI_24" && radio.Band != "2.4GHZ") || (scope == "WIFI_5" && radio.Band != "5GHZ") || (scope != "ALL" && scope != "WIFI_24" && scope != "WIFI_5") {
			continue
		}
		encrypted, err := base64.StdEncoding.DecodeString(radio.EncryptedPassword)
		if err != nil {
			return nil, fmt.Errorf("decode Wi-Fi password: %w", err)
		}
		password, err := d.decryptor.Decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt Wi-Fi password: %w", err)
		}
		if scope == "WIFI_24" || scope == "WIFI_5" {
			if err := appendValue(bandKey+".ssid", radio.SSID); err != nil {
				return nil, err
			}
			if err := appendValue(bandKey+".password", password); err != nil {
				return nil, err
			}
			continue
		}
		for _, item := range []struct{ key, value string }{{bandKey + ".enabled", boolean(radio.Enabled)}, {bandKey + ".ssid", radio.SSID}, {bandKey + ".security_mode", tr181Security(radio.SecurityMode)}, {bandKey + ".password", password}} {
			if err := appendValue(item.key, item.value); err != nil {
				return nil, err
			}
		}
		if radio.Channel != nil {
			if err := appendValue(bandKey+".channel", strconv.Itoa(*radio.Channel)); err != nil {
				return nil, err
			}
		}
		if radio.BandwidthMHz != nil {
			if err := appendValue(bandKey+".bandwidth", strconv.Itoa(*radio.BandwidthMHz)+"MHz"); err != nil {
				return nil, err
			}
		}
	}
	network := snapshot.Network
	if scope == "DHCP" {
		if network.DHCPSubnetMask != nil {
			if err := appendValue("dhcp.subnet_mask", *network.DHCPSubnetMask); err != nil {
				return nil, err
			}
		}
		if network.DHCPGateway != nil {
			if err := appendValue("dhcp.gateway", *network.DHCPGateway); err != nil {
				return nil, err
			}
		}
		if network.DHCPStart != nil {
			if err := appendValue("dhcp.start", *network.DHCPStart); err != nil {
				return nil, err
			}
		}
		if network.DHCPEnd != nil {
			if err := appendValue("dhcp.end", *network.DHCPEnd); err != nil {
				return nil, err
			}
		}
		return values, nil
	}
	if scope == "DNS" {
		dns := make([]string, 0, 2)
		if network.DNSPrimary != nil {
			dns = append(dns, *network.DNSPrimary)
		}
		if network.DNSSecondary != nil {
			dns = append(dns, *network.DNSSecondary)
		}
		if len(dns) > 0 {
			if err := appendValue("dns.servers", strings.Join(dns, ",")); err != nil {
				return nil, err
			}
		}
		return values, nil
	}
	if scope != "ALL" {
		return values, nil
	}
	if err := appendValue("dhcp.enabled", boolean(network.DHCPEnabled)); err != nil {
		return nil, err
	}
	if network.DHCPStart != nil {
		if err := appendValue("dhcp.start", *network.DHCPStart); err != nil {
			return nil, err
		}
	}
	if network.DHCPEnd != nil {
		if err := appendValue("dhcp.end", *network.DHCPEnd); err != nil {
			return nil, err
		}
	}
	if network.DHCPSubnetMask != nil {
		if err := appendValue("dhcp.subnet_mask", *network.DHCPSubnetMask); err != nil {
			return nil, err
		}
	}
	if network.DHCPGateway != nil {
		if err := appendValue("dhcp.gateway", *network.DHCPGateway); err != nil {
			return nil, err
		}
	}
	if network.DHCPLeaseSeconds != nil {
		if err := appendValue("dhcp.lease_seconds", strconv.Itoa(*network.DHCPLeaseSeconds)); err != nil {
			return nil, err
		}
	}
	dns := make([]string, 0, 2)
	if network.DNSPrimary != nil {
		dns = append(dns, *network.DNSPrimary)
	}
	if network.DNSSecondary != nil {
		dns = append(dns, *network.DNSSecondary)
	}
	if len(dns) > 0 {
		if err := appendValue("dns.servers", strings.Join(dns, ",")); err != nil {
			return nil, err
		}
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
