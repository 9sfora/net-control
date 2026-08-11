package cwmp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reception struct {
	SessionID string
	Known     bool
}

type Repository struct {
	pool       *pgxpool.Pool
	tenantSlug string
}

func NewRepository(pool *pgxpool.Pool, tenantSlug string) *Repository {
	return &Repository{pool: pool, tenantSlug: tenantSlug}
}

func (r *Repository) RecordInform(ctx context.Context, inform Inform, ipAddress, userAgent string) (Reception, error) {
	var tenantID string
	if err := r.pool.QueryRow(ctx, `SELECT id FROM public.tenants WHERE slug = $1 AND active`, r.tenantSlug).Scan(&tenantID); err != nil {
		return Reception{}, fmt.Errorf("resolve ACS tenant: %w", err)
	}
	events, _ := json.Marshal(inform.Events)
	parameters, _ := json.Marshal(inform.Parameters)
	softwareVersion := Parameter(inform, ".DeviceInfo.SoftwareVersion")
	hardwareVersion := Parameter(inform, ".DeviceInfo.HardwareVersion")
	var ip any
	if strings.TrimSpace(ipAddress) != "" {
		ip = ipAddress
	}

	var reception Reception
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var routerID string
		err := tx.QueryRow(ctx, `
			SELECT id FROM acs.routers
			WHERE tenant_id = $1 AND oui = $2 AND product_class = $3 AND serial_number = $4`,
			tenantID, inform.OUI, inform.ProductClass, inform.SerialNumber).Scan(&routerID)
		if err == nil {
			reception.Known = true
			if _, err := tx.Exec(ctx, `
				UPDATE acs.routers SET manufacturer = $2, software_version = NULLIF($3, ''),
				hardware_version = NULLIF($4, ''), last_seen_at = now(), last_inform_at = now(), last_ip_address = $5
				WHERE id = $1`, routerID, inform.Manufacturer, softwareVersion, hardwareVersion, ip); err != nil {
				return fmt.Errorf("update router: %w", err)
			}
			return tx.QueryRow(ctx, `
				INSERT INTO acs.router_sessions (tenant_id, router_id, ip_address, user_agent, inform_events)
				VALUES ($1, $2, $3, $4, $5) RETURNING id`, tenantID, routerID, ip, userAgent, events).Scan(&reception.SessionID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find router: %w", err)
		}

		var unknownID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO acs.unknown_routers
				(tenant_id, manufacturer, oui, product_class, serial_number, software_version, hardware_version,
				 last_ip_address, last_user_agent, last_events, last_parameters)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9, $10, $11)
			ON CONFLICT (tenant_id, oui, product_class, serial_number) DO UPDATE SET
				manufacturer = EXCLUDED.manufacturer, software_version = EXCLUDED.software_version,
				hardware_version = EXCLUDED.hardware_version, last_seen_at = now(),
				last_ip_address = EXCLUDED.last_ip_address, last_user_agent = EXCLUDED.last_user_agent,
				last_events = EXCLUDED.last_events, last_parameters = EXCLUDED.last_parameters,
				inform_count = acs.unknown_routers.inform_count + 1
			RETURNING id`, tenantID, inform.Manufacturer, inform.OUI, inform.ProductClass, inform.SerialNumber,
			softwareVersion, hardwareVersion, ip, userAgent, events, parameters).Scan(&unknownID); err != nil {
			return fmt.Errorf("upsert unknown router: %w", err)
		}
		return tx.QueryRow(ctx, `
			INSERT INTO acs.router_sessions (tenant_id, unknown_router_id, ip_address, user_agent, inform_events)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, tenantID, unknownID, ip, userAgent, events).Scan(&reception.SessionID)
	})
	if err != nil {
		return Reception{}, err
	}
	return reception, nil
}
