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

var ErrNotFound = errors.New("unknown router not found")

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
