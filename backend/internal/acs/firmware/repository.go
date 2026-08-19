package firmware

import (
	"context"
	"errors"
	"fmt"
	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound             = errors.New("firmware not found")
	ErrConflict             = errors.New("firmware already exists")
	ErrInvalidCompatibility = errors.New("invalid firmware compatibility")
	ErrImmutable            = errors.New("firmware with file is immutable")
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }
func (r *Repository) List(ctx context.Context, tenantID, modelID string) ([]Firmware, error) {
	items := make([]Firmware, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, baseSelect+` WHERE f.tenant_id=$1 AND (NULLIF($2,'') IS NULL OR f.router_model_id=NULLIF($2,'')::uuid) ORDER BY f.created_at DESC`, tenantID, modelID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Firmware
			if err := scan(rows, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}
func (r *Repository) Get(ctx context.Context, tenantID, id string) (Firmware, error) {
	var item Firmware
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		err := scan(tx.QueryRow(ctx, baseSelect+` WHERE f.tenant_id=$1 AND f.id=$2`, tenantID, id), &item)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return item, err
}
func (r *Repository) Create(ctx context.Context, tenantID string, input Input) (Firmware, error) {
	var item Firmware
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		if err := validateCompatibility(ctx, tx, tenantID, input.RouterModelID, input.HardwareVersionID); err != nil {
			return err
		}
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		var id string
		err := tx.QueryRow(ctx, `INSERT INTO acs.firmwares(tenant_id,router_model_id,hardware_version_id,name,version,description,release_notes,is_default,active)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)RETURNING id`, tenantID, input.RouterModelID, input.HardwareVersionID, input.Name, input.Version, input.Description, input.ReleaseNotes, input.IsDefault, active).Scan(&id)
		if err != nil {
			return persistenceError(err)
		}
		return scan(tx.QueryRow(ctx, baseSelect+` WHERE f.tenant_id=$1 AND f.id=$2`, tenantID, id), &item)
	})
	return item, err
}
func (r *Repository) Update(ctx context.Context, tenantID, id string, input Input) (Firmware, error) {
	var item Firmware
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		if err := validateCompatibility(ctx, tx, tenantID, input.RouterModelID, input.HardwareVersionID); err != nil {
			return err
		}
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		var status string
		err := tx.QueryRow(ctx, `UPDATE acs.firmwares SET router_model_id=$3,hardware_version_id=$4,name=$5,version=$6,description=$7,release_notes=$8,is_default=$9,active=$10 WHERE tenant_id=$1 AND id=$2 AND status='NO_FILE' RETURNING status`, tenantID, id, input.RouterModelID, input.HardwareVersionID, input.Name, input.Version, input.Description, input.ReleaseNotes, input.IsDefault, active).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, `SELECT true FROM acs.firmwares WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&exists); errors.Is(checkErr, pgx.ErrNoRows) {
				return ErrNotFound
			} else if checkErr != nil {
				return checkErr
			}
			return ErrImmutable
		}
		if err != nil {
			return persistenceError(err)
		}
		return scan(tx.QueryRow(ctx, baseSelect+` WHERE f.tenant_id=$1 AND f.id=$2`, tenantID, id), &item)
	})
	return item, err
}
func (r *Repository) Delete(ctx context.Context, tenantID, id string) (string, error) {
	var path *string
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `DELETE FROM acs.firmwares WHERE tenant_id=$1 AND id=$2 RETURNING file_path`, tenantID, id).Scan(&path)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	if path == nil {
		return "", err
	}
	return *path, err
}
func (r *Repository) Attach(ctx context.Context, tenantID, id, originalName string, file StoredFile) (Firmware, error) {
	var item Firmware
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `UPDATE acs.firmwares SET file_path=$3,file_name=$4,file_size=$5,checksum_sha256=$6,status='READY',uploaded_at=now() WHERE tenant_id=$1 AND id=$2 AND status='NO_FILE' RETURNING id`, tenantID, id, file.Path, originalName, file.Size, file.Checksum).Scan(&item.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, `SELECT true FROM acs.firmwares WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&exists); errors.Is(checkErr, pgx.ErrNoRows) {
				return ErrNotFound
			} else if checkErr != nil {
				return checkErr
			}
			return ErrImmutable
		}
		if err != nil {
			return err
		}
		return scan(tx.QueryRow(ctx, baseSelect+` WHERE f.tenant_id=$1 AND f.id=$2`, tenantID, id), &item)
	})
	return item, err
}
func (r *Repository) Exists(ctx context.Context, tenantID, id string) error {
	return database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM acs.firmwares WHERE tenant_id=$1 AND id=$2`, tenantID, id).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err == nil && status != "NO_FILE" {
			return ErrImmutable
		}
		return err
	})
}

const baseSelect = `SELECT f.id,f.router_model_id,m.name,f.hardware_version_id,h.version,f.name,f.version,f.description,f.file_name,f.file_size,f.checksum_sha256,f.release_notes,f.status,f.is_default,f.active,f.uploaded_at,f.created_at,f.updated_at FROM acs.firmwares f JOIN acs.router_models m ON m.id=f.router_model_id LEFT JOIN acs.router_model_hardware_versions h ON h.id=f.hardware_version_id`

type scanner interface{ Scan(...any) error }

func scan(row scanner, item *Firmware) error {
	return row.Scan(&item.ID, &item.RouterModelID, &item.RouterModelName, &item.HardwareVersionID, &item.HardwareVersion, &item.Name, &item.Version, &item.Description, &item.FileName, &item.FileSize, &item.ChecksumSHA256, &item.ReleaseNotes, &item.Status, &item.IsDefault, &item.Active, &item.UploadedAt, &item.CreatedAt, &item.UpdatedAt)
}
func validateCompatibility(ctx context.Context, tx pgx.Tx, tenantID, modelID string, hardwareID *string) error {
	var active bool
	if err := tx.QueryRow(ctx, `SELECT active FROM acs.router_models WHERE tenant_id=$1 AND id=$2`, tenantID, modelID).Scan(&active); errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidCompatibility
	} else if err != nil {
		return err
	}
	if !active {
		return ErrInvalidCompatibility
	}
	if hardwareID != nil {
		if err := tx.QueryRow(ctx, `SELECT active FROM acs.router_model_hardware_versions WHERE tenant_id=$1 AND router_model_id=$2 AND id=$3`, tenantID, modelID, *hardwareID).Scan(&active); errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidCompatibility
		} else if err != nil {
			return err
		}
		if !active {
			return ErrInvalidCompatibility
		}
	}
	return nil
}
func persistenceError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return fmt.Errorf("firmware persistence: %w", err)
}
