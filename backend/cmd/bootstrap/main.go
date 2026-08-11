package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	values, err := environment()
	if err != nil {
		logger.Error("invalid bootstrap configuration", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, values.databaseURL)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(values.password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("password hashing failed", "error", err)
		os.Exit(1)
	}
	if err := bootstrap(ctx, pool, values, string(hash)); err != nil {
		logger.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	logger.Info("tenant administrator created", "tenant", values.tenantSlug, "email", values.email)
}

type input struct {
	databaseURL, tenantName, tenantSlug, name, email, password string
}

func environment() (input, error) {
	values := input{
		databaseURL: os.Getenv("DATABASE_URL"), tenantName: os.Getenv("BOOTSTRAP_TENANT_NAME"),
		tenantSlug: strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_TENANT_SLUG"))),
		name:       os.Getenv("BOOTSTRAP_ADMIN_NAME"), email: strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))),
		password: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
	}
	if values.databaseURL == "" || values.tenantName == "" || values.tenantSlug == "" || values.name == "" || values.email == "" || values.password == "" {
		return input{}, errors.New("DATABASE_URL and all BOOTSTRAP_* variables are required")
	}
	if len(values.password) < 12 {
		return input{}, errors.New("BOOTSTRAP_ADMIN_PASSWORD must contain at least 12 characters")
	}
	return values, nil
}

func bootstrap(ctx context.Context, pool *pgxpool.Pool, values input, passwordHash string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tenantID string
	if err := tx.QueryRow(ctx, `INSERT INTO public.tenants (name, slug) VALUES ($1, $2) RETURNING id`, values.tenantName, values.tenantSlug).Scan(&tenantID); err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}
	var userID, roleID string
	if err := tx.QueryRow(ctx, `INSERT INTO public.users (tenant_id, name, email, password_hash) VALUES ($1, $2, $3, $4) RETURNING id`, tenantID, values.name, values.email, passwordHash).Scan(&userID); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO public.roles (tenant_id, name, description) VALUES ($1, 'Administrator', 'Full tenant access') RETURNING id`, tenantID).Scan(&roleID); err != nil {
		return fmt.Errorf("create role: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.user_roles (tenant_id, user_id, role_id) VALUES ($1, $2, $3)`, tenantID, userID, roleID); err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.role_permissions (tenant_id, role_id, permission_id) SELECT $1, $2, id FROM public.permissions`, tenantID, roleID); err != nil {
		return fmt.Errorf("assign permissions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return err
	}
	return nil
}
