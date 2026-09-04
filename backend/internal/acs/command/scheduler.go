package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Scheduler struct {
	pool              *pgxpool.Pool
	logger            *slog.Logger
	processingTimeout time.Duration
	retryIntervals    []time.Duration
	now               func() time.Time
}

type Result struct {
	Waiting   int64
	Expired   int64
	Recovered int64
	Failed    int64
}

func NewScheduler(pool *pgxpool.Pool, logger *slog.Logger, processingTimeout time.Duration, retryIntervals []time.Duration) *Scheduler {
	return &Scheduler{pool: pool, logger: logger, processingTimeout: processingTimeout, retryIntervals: retryIntervals, now: time.Now}
}

func (s *Scheduler) Run(ctx context.Context) (Result, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM public.tenants WHERE active ORDER BY id`)
	if err != nil {
		return Result{}, fmt.Errorf("list worker tenants: %w", err)
	}
	var tenantIDs []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			rows.Close()
			return Result{}, fmt.Errorf("scan worker tenant: %w", err)
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Result{}, fmt.Errorf("iterate worker tenants: %w", err)
	}
	rows.Close()

	var total Result
	for _, tenantID := range tenantIDs {
		result, err := s.runTenant(ctx, tenantID)
		if err != nil {
			s.logger.ErrorContext(ctx, "command maintenance failed", "tenant_id", tenantID, "error", err)
			continue
		}
		total.Waiting += result.Waiting
		total.Expired += result.Expired
		total.Recovered += result.Recovered
		total.Failed += result.Failed
	}
	return total, nil
}

func (s *Scheduler) runTenant(ctx context.Context, tenantID string) (Result, error) {
	var result Result
	err := database.InTenantTransaction(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		now := s.now()
		expired, err := tx.Query(ctx, `
			UPDATE acs.router_commands SET status='EXPIRED', completed_at=$2, error_code='COMMAND_EXPIRED', error_message='Command expired before execution'
			WHERE tenant_id=$1 AND status IN ('PENDING','WAITING_DEVICE','RETRY_SCHEDULED') AND expires_at <= $2
			RETURNING id`, tenantID, now)
		if err != nil {
			return fmt.Errorf("expire commands: %w", err)
		}
		var expiredIDs []string
		for expired.Next() {
			var commandID string
			if err := expired.Scan(&commandID); err != nil {
				expired.Close()
				return err
			}
			expiredIDs = append(expiredIDs, commandID)
			result.Expired++
		}
		expired.Close()
		for _, commandID := range expiredIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,to_status,reason) VALUES ($1,$2,'EXPIRED','Expiration deadline reached')`, tenantID, commandID); err != nil {
				return err
			}
		}

		waiting, err := tx.Query(ctx, `UPDATE acs.router_commands SET status='WAITING_DEVICE' WHERE tenant_id=$1 AND status='PENDING' AND available_at <= $2 RETURNING id`, tenantID, now)
		if err != nil {
			return fmt.Errorf("queue waiting commands: %w", err)
		}
		var waitingIDs []string
		for waiting.Next() {
			var commandID string
			if err := waiting.Scan(&commandID); err != nil {
				waiting.Close()
				return err
			}
			waitingIDs = append(waitingIDs, commandID)
			result.Waiting++
		}
		waiting.Close()
		for _, commandID := range waitingIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,from_status,to_status,reason) VALUES ($1,$2,'PENDING','WAITING_DEVICE','Waiting for next CPE session')`, tenantID, commandID); err != nil {
				return err
			}
		}

		stuck, err := tx.Query(ctx, `
			SELECT c.id, c.attempts_count, c.max_attempts, a.id
			FROM acs.router_commands c
			LEFT JOIN LATERAL (
				SELECT id FROM acs.router_command_attempts WHERE router_command_id=c.id AND status='PROCESSING' ORDER BY started_at DESC LIMIT 1
			) a ON true
			WHERE c.tenant_id=$1 AND c.status='PROCESSING'
			  AND c.last_attempt_at < CASE WHEN c.type='UPDATE_FIRMWARE' THEN $3::timestamptz ELSE $2::timestamptz END
			FOR UPDATE OF c SKIP LOCKED`, tenantID, now.Add(-s.processingTimeout), now.Add(-firmwareProcessingTimeout(s.processingTimeout)))
		if err != nil {
			return fmt.Errorf("find stuck commands: %w", err)
		}
		type stuckCommand struct {
			id                    string
			attempts, maxAttempts int
			attemptID             *string
		}
		var commands []stuckCommand
		for stuck.Next() {
			var item stuckCommand
			if err := stuck.Scan(&item.id, &item.attempts, &item.maxAttempts, &item.attemptID); err != nil {
				stuck.Close()
				return err
			}
			commands = append(commands, item)
		}
		stuck.Close()
		for _, item := range commands {
			if item.attemptID != nil {
				if _, err := tx.Exec(ctx, `UPDATE acs.router_command_attempts SET status='FAILED',error_code='PROCESSING_TIMEOUT',error_message='CPE did not respond in time',finished_at=$2,duration_ms=round(EXTRACT(EPOCH FROM($2-started_at))*1000)::int WHERE id=$1`, *item.attemptID, now); err != nil {
					return err
				}
			}
			if item.attempts >= item.maxAttempts {
				if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='FAILED',completed_at=$2,error_code='MAX_ATTEMPTS_EXCEEDED',error_message='Maximum attempts exceeded' WHERE id=$1`, item.id, now); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,from_status,to_status,reason) VALUES ($1,$2,'PROCESSING','FAILED','Maximum attempts exceeded')`, tenantID, item.id); err != nil {
					return err
				}
				result.Failed++
				continue
			}
			next := now.Add(s.retryDelay(item.attempts))
			if _, err := tx.Exec(ctx, `UPDATE acs.router_commands SET status='RETRY_SCHEDULED',available_at=$2,next_attempt_at=$2,error_code='PROCESSING_TIMEOUT',error_message='CPE did not respond in time' WHERE id=$1`, item.id, next); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO acs.router_command_events (tenant_id,router_command_id,from_status,to_status,reason,metadata) VALUES ($1,$2,'PROCESSING','RETRY_SCHEDULED','CPE response timeout',jsonb_build_object('next_attempt_at',$3::timestamptz))`, tenantID, item.id, next); err != nil {
				return err
			}
			result.Recovered++
		}
		return nil
	})
	return result, err
}

func firmwareProcessingTimeout(defaultTimeout time.Duration) time.Duration {
	if defaultTimeout > 30*time.Minute {
		return defaultTimeout
	}
	return 30 * time.Minute
}

func (s *Scheduler) retryDelay(attempts int) time.Duration {
	index := attempts - 1
	if index < 0 {
		index = 0
	}
	if index >= len(s.retryIntervals) {
		index = len(s.retryIntervals) - 1
	}
	return s.retryIntervals[index]
}
