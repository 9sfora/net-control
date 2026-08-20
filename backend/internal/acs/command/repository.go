package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("command not found")
	ErrCannotCancel = errors.New("command cannot be canceled")
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) List(ctx context.Context, tenantID string, filter Filter) ([]Item, error) {
	items := make([]Item, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT c.id,c.router_id,r.name,c.type,c.status,c.priority,c.attempts_count,c.max_attempts,
			       c.idempotency_key,c.available_at,c.next_attempt_at,c.expires_at,c.completed_at,
			       c.error_code,c.error_message,c.result,c.created_at,c.updated_at
			FROM acs.router_commands c JOIN acs.routers r ON r.id=c.router_id
			WHERE c.tenant_id=$1 AND ($2='' OR c.status=$2) AND (NULLIF($3,'') IS NULL OR c.router_id=NULLIF($3,'')::uuid)
			ORDER BY c.created_at DESC,c.id
			LIMIT $4 OFFSET $5`, tenantID, filter.Status, filter.RouterID, filter.Limit, filter.Offset)
		if err != nil {
			return fmt.Errorf("list commands: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item Item
			if err := rows.Scan(&item.ID, &item.RouterID, &item.RouterName, &item.Type, &item.Status, &item.Priority,
				&item.AttemptsCount, &item.MaxAttempts, &item.IdempotencyKey, &item.AvailableAt, &item.NextAttemptAt,
				&item.ExpiresAt, &item.CompletedAt, &item.ErrorCode, &item.ErrorMessage, &item.Result, &item.CreatedAt, &item.UpdatedAt); err != nil {
				return fmt.Errorf("scan command: %w", err)
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) Get(ctx context.Context, tenantID, commandID string) (Detail, error) {
	var detail Detail
	detail.Attempts = make([]Attempt, 0)
	detail.Events = make([]Event, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT c.id,c.router_id,r.name,c.type,c.status,c.priority,c.attempts_count,c.max_attempts,
			       c.idempotency_key,c.available_at,c.next_attempt_at,c.expires_at,c.completed_at,
			       c.error_code,c.error_message,c.result,c.created_at,c.updated_at,
			       c.parameters
			FROM acs.router_commands c JOIN acs.routers r ON r.id=c.router_id
			WHERE c.tenant_id=$1 AND c.id=$2`, tenantID, commandID).Scan(&detail.ID, &detail.RouterID, &detail.RouterName,
			&detail.Type, &detail.Status, &detail.Priority, &detail.AttemptsCount, &detail.MaxAttempts, &detail.IdempotencyKey,
			&detail.AvailableAt, &detail.NextAttemptAt, &detail.ExpiresAt, &detail.CompletedAt, &detail.ErrorCode,
			&detail.ErrorMessage, &detail.Result, &detail.CreatedAt, &detail.UpdatedAt, &detail.Parameters)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get command: %w", err)
		}
		attemptRows, err := tx.Query(ctx, `SELECT id,session_id,attempt_number,status,response_payload,error_code,error_message,duration_ms,started_at,finished_at FROM acs.router_command_attempts WHERE tenant_id=$1 AND router_command_id=$2 ORDER BY attempt_number`, tenantID, commandID)
		if err != nil {
			return fmt.Errorf("list attempts: %w", err)
		}
		for attemptRows.Next() {
			var attempt Attempt
			if err := attemptRows.Scan(&attempt.ID, &attempt.SessionID, &attempt.AttemptNumber, &attempt.Status, &attempt.Response, &attempt.ErrorCode, &attempt.ErrorMessage, &attempt.DurationMS, &attempt.StartedAt, &attempt.FinishedAt); err != nil {
				attemptRows.Close()
				return err
			}
			detail.Attempts = append(detail.Attempts, attempt)
		}
		if err := attemptRows.Err(); err != nil {
			attemptRows.Close()
			return err
		}
		attemptRows.Close()
		eventRows, err := tx.Query(ctx, `SELECT id,from_status,to_status,reason,metadata,created_at FROM acs.router_command_events WHERE tenant_id=$1 AND router_command_id=$2 ORDER BY created_at,id`, tenantID, commandID)
		if err != nil {
			return fmt.Errorf("list command events: %w", err)
		}
		defer eventRows.Close()
		for eventRows.Next() {
			var event Event
			if err := eventRows.Scan(&event.ID, &event.FromStatus, &event.ToStatus, &event.Reason, &event.Metadata, &event.CreatedAt); err != nil {
				return err
			}
			detail.Events = append(detail.Events, event)
		}
		return eventRows.Err()
	})
	return detail, err
}

func (r *Repository) Cancel(ctx context.Context, tenantID, commandID, userID string) (Item, error) {
	var item Item
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM acs.router_commands WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, commandID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if status != "PENDING" && status != "WAITING_DEVICE" && status != "RETRY_SCHEDULED" {
			return ErrCannotCancel
		}
		err := tx.QueryRow(ctx, `
			UPDATE acs.router_commands SET status='CANCELED',canceled_at=now(),completed_at=now(),error_code=NULL,error_message=NULL
			WHERE id=$1 RETURNING id,router_id,(SELECT name FROM acs.routers WHERE id=router_id),type,status,priority,attempts_count,max_attempts,idempotency_key,
			available_at,next_attempt_at,expires_at,completed_at,error_code,error_message,result,created_at,updated_at`, commandID).Scan(
			&item.ID, &item.RouterID, &item.RouterName, &item.Type, &item.Status, &item.Priority, &item.AttemptsCount, &item.MaxAttempts,
			&item.IdempotencyKey, &item.AvailableAt, &item.NextAttemptAt, &item.ExpiresAt, &item.CompletedAt, &item.ErrorCode, &item.ErrorMessage, &item.Result, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return fmt.Errorf("cancel command: %w", err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO acs.router_command_events(tenant_id,router_command_id,from_status,to_status,reason,metadata)VALUES($1,$2,$3,'CANCELED','Canceled by user',jsonb_build_object('user_id',$4::text))`, tenantID, commandID, status, userID)
		return err
	})
	return item, err
}
