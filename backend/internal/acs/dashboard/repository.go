package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Snapshot(ctx context.Context, tenantID string, now time.Time, onlineTimeout, offlineTimeout time.Duration) (Snapshot, error) {
	result := Snapshot{GeneratedAt: now, RecentConnections: make([]RecentConnection, 0), RecentActivity: make([]RecentCommand, 0), RecentFailures: make([]RecentCommand, 0)}
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT count(*),
			 count(*) FILTER(WHERE last_seen_at >= $2),
			 count(*) FILTER(WHERE last_seen_at < $2 AND last_seen_at >= $3),
			 count(*) FILTER(WHERE last_seen_at < $3),
			 count(*) FILTER(WHERE last_seen_at IS NULL),
			 count(*) FILTER(WHERE desired_status='BLOCKED' OR reported_status='BLOCKED')
			FROM acs.routers WHERE tenant_id=$1`, tenantID, now.Add(-onlineTimeout), now.Add(-offlineTimeout)).Scan(
			&result.Routers.Total, &result.Routers.Online, &result.Routers.Stale, &result.Routers.Offline, &result.Routers.NeverConnected, &result.Routers.Blocked); err != nil {
			return fmt.Errorf("aggregate routers: %w", err)
		}
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FILTER(WHERE status='PENDING'),count(*) FILTER(WHERE status='WAITING_DEVICE'),
			 count(*) FILTER(WHERE status='PROCESSING'),count(*) FILTER(WHERE status='RETRY_SCHEDULED'),
			 count(*) FILTER(WHERE status='FAILED'),count(*) FILTER(WHERE status='EXPIRED'),
			 count(*) FILTER(WHERE status='COMPLETED' AND completed_at >= $2)
			FROM acs.router_commands WHERE tenant_id=$1`, tenantID, now.Add(-24*time.Hour)).Scan(&result.Commands.Pending,
			&result.Commands.WaitingDevice, &result.Commands.Processing, &result.Commands.RetryScheduled, &result.Commands.Failed,
			&result.Commands.Expired, &result.Commands.CompletedLast24); err != nil {
			return fmt.Errorf("aggregate commands: %w", err)
		}

		connections, err := tx.Query(ctx, `SELECT id,name,serial_number,host(last_ip_address),last_seen_at FROM acs.routers WHERE tenant_id=$1 AND last_seen_at IS NOT NULL ORDER BY last_seen_at DESC LIMIT 10`, tenantID)
		if err != nil {
			return fmt.Errorf("recent connections: %w", err)
		}
		for connections.Next() {
			var item RecentConnection
			if err := connections.Scan(&item.RouterID, &item.RouterName, &item.SerialNumber, &item.IPAddress, &item.LastSeenAt); err != nil {
				connections.Close()
				return err
			}
			result.RecentConnections = append(result.RecentConnections, item)
		}
		if err := connections.Err(); err != nil {
			connections.Close()
			return err
		}
		connections.Close()

		activity, err := tx.Query(ctx, `
			SELECT c.id,c.router_id,r.name,c.type,c.status,c.error_code,c.error_message,c.created_at,c.completed_at
			FROM acs.router_commands c JOIN acs.routers r ON r.id=c.router_id
			WHERE c.tenant_id=$1 ORDER BY c.updated_at DESC LIMIT 10`, tenantID)
		if err != nil {
			return fmt.Errorf("recent activity: %w", err)
		}
		for activity.Next() {
			var item RecentCommand
			if err := scanCommand(activity, &item); err != nil {
				activity.Close()
				return err
			}
			result.RecentActivity = append(result.RecentActivity, item)
		}
		if err := activity.Err(); err != nil {
			activity.Close()
			return err
		}
		activity.Close()

		failures, err := tx.Query(ctx, `
			SELECT c.id,c.router_id,r.name,c.type,c.status,c.error_code,c.error_message,c.created_at,c.completed_at
			FROM acs.router_commands c JOIN acs.routers r ON r.id=c.router_id
			WHERE c.tenant_id=$1 AND c.status IN('FAILED','EXPIRED') ORDER BY c.updated_at DESC LIMIT 10`, tenantID)
		if err != nil {
			return fmt.Errorf("recent failures: %w", err)
		}
		defer failures.Close()
		for failures.Next() {
			var item RecentCommand
			if err := scanCommand(failures, &item); err != nil {
				return err
			}
			result.RecentFailures = append(result.RecentFailures, item)
		}
		return failures.Err()
	})
	return result, err
}

type scanner interface{ Scan(...any) error }

func scanCommand(row scanner, item *RecentCommand) error {
	return row.Scan(&item.CommandID, &item.RouterID, &item.RouterName, &item.Type, &item.Status, &item.ErrorCode, &item.ErrorMessage, &item.CreatedAt, &item.CompletedAt)
}
