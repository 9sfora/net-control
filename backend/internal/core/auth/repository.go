package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type User struct {
	ID           string
	TenantID     string
	Name         string
	Email        string
	PasswordHash string
	Permissions  []string
}

type Repository interface {
	FindUserForLogin(context.Context, string, string) (User, error)
}

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindUserForLogin(ctx context.Context, tenantSlug, email string) (User, error) {
	var tenantID string
	err := r.pool.QueryRow(ctx,
		"SELECT id FROM public.tenants WHERE slug = $1 AND active", strings.ToLower(strings.TrimSpace(tenantSlug)),
	).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("find tenant: %w", err)
	}

	var user User
	err = database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, email, password_hash
			FROM public.users
			WHERE lower(email) = lower($1) AND active`, strings.TrimSpace(email),
		).Scan(&user.ID, &user.TenantID, &user.Name, &user.Email, &user.PasswordHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidCredentials
		}
		if err != nil {
			return fmt.Errorf("find user: %w", err)
		}

		rows, err := tx.Query(ctx, `
			SELECT DISTINCT p.code
			FROM public.user_roles ur
			JOIN public.role_permissions rp ON rp.role_id = ur.role_id
			JOIN public.permissions p ON p.id = rp.permission_id
			JOIN public.roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.active
			ORDER BY p.code`, user.ID)
		if err != nil {
			return fmt.Errorf("query permissions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var permission string
			if err := rows.Scan(&permission); err != nil {
				return fmt.Errorf("scan permission: %w", err)
			}
			user.Permissions = append(user.Permissions, permission)
		}
		return rows.Err()
	})
	if err != nil {
		return User{}, err
	}
	return user, nil
}
