package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound         = errors.New("catalog item not found")
	ErrConflict         = errors.New("catalog item already exists")
	ErrInvalidReference = errors.New("invalid catalog reference")
	ErrInUse            = errors.New("catalog item is in use")
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) ListManufacturers(ctx context.Context, tenantID, search string) ([]Manufacturer, error) {
	items := make([]Manufacturer, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,name,description,active,created_at,updated_at FROM acs.manufacturers WHERE tenant_id=$1 AND ($2='' OR strpos(lower(name),lower($2))>0) ORDER BY name`, tenantID, search)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Manufacturer
			if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) SaveManufacturer(ctx context.Context, tenantID, id string, input ManufacturerInput) (Manufacturer, error) {
	var item Manufacturer
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		var err error
		if id == "" {
			err = tx.QueryRow(ctx, `INSERT INTO acs.manufacturers(tenant_id,name,description,active)VALUES($1,$2,$3,$4)RETURNING id,name,description,active,created_at,updated_at`, tenantID, input.Name, input.Description, active).Scan(&item.ID, &item.Name, &item.Description, &item.Active, &item.CreatedAt, &item.UpdatedAt)
		} else {
			err = tx.QueryRow(ctx, `UPDATE acs.manufacturers SET name=$3,description=$4,active=$5 WHERE tenant_id=$1 AND id=$2 RETURNING id,name,description,active,created_at,updated_at`, tenantID, id, input.Name, input.Description, active).Scan(&item.ID, &item.Name, &item.Description, &item.Active, &item.CreatedAt, &item.UpdatedAt)
		}
		return catalogError(err)
	})
	return item, err
}

func (r *Repository) DeleteManufacturer(ctx context.Context, tenantID, id string) error {
	return database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM acs.manufacturers WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		if err != nil {
			return deleteCatalogError(err)
		}
		if result.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *Repository) ListModels(ctx context.Context, tenantID, search, manufacturerID string) ([]RouterModel, error) {
	items := make([]RouterModel, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT m.id,m.manufacturer_id,f.name,m.name,m.manufacturer_model_name,m.supports_wifi_2_4,m.supports_wifi_5,m.supported_auth_modes,m.product_class,m.oui,m.protocol,m.description,m.active,m.created_at,m.updated_at FROM acs.router_models m JOIN acs.manufacturers f ON f.id=m.manufacturer_id WHERE m.tenant_id=$1 AND ($2='' OR strpos(lower(m.name||' '||f.name),lower($2))>0) AND (NULLIF($3,'') IS NULL OR m.manufacturer_id=NULLIF($3,'')::uuid) ORDER BY f.name,m.name`, tenantID, search, manufacturerID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item RouterModel
			if err := scanModel(rows, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) GetModel(ctx context.Context, tenantID, id string) (RouterModel, error) {
	var item RouterModel
	item.HardwareVersions = make([]HardwareVersion, 0)
	item.CWMPParameters = make([]CWMPParameter, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		err := scanModel(tx.QueryRow(ctx, `SELECT m.id,m.manufacturer_id,f.name,m.name,m.manufacturer_model_name,m.supports_wifi_2_4,m.supports_wifi_5,m.supported_auth_modes,m.product_class,m.oui,m.protocol,m.description,m.active,m.created_at,m.updated_at FROM acs.router_models m JOIN acs.manufacturers f ON f.id=m.manufacturer_id WHERE m.tenant_id=$1 AND m.id=$2`, tenantID, id), &item)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id,router_model_id,version,description,active,created_at,updated_at FROM acs.router_model_hardware_versions WHERE tenant_id=$1 AND router_model_id=$2 ORDER BY version`, tenantID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var version HardwareVersion
			if err := rows.Scan(&version.ID, &version.RouterModelID, &version.Version, &version.Description, &version.Active, &version.CreatedAt, &version.UpdatedAt); err != nil {
				return err
			}
			item.HardwareVersions = append(item.HardwareVersions, version)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		parameterRows, err := tx.Query(ctx, `SELECT id,router_model_id,parameter_key,parameter_name,xsd_type,active,created_at,updated_at FROM acs.router_model_cwmp_parameters WHERE tenant_id=$1 AND router_model_id=$2 ORDER BY parameter_key`, tenantID, id)
		if err != nil {
			return err
		}
		defer parameterRows.Close()
		for parameterRows.Next() {
			var parameter CWMPParameter
			if err := scanCWMPParameter(parameterRows, &parameter); err != nil {
				return err
			}
			item.CWMPParameters = append(item.CWMPParameters, parameter)
		}
		return parameterRows.Err()
	})
	return item, err
}

func (r *Repository) SaveModel(ctx context.Context, tenantID, id string, input RouterModelInput) (RouterModel, error) {
	var item RouterModel
	authModes, _ := json.Marshal(input.SupportedAuthModes)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		var row pgx.Row
		if id == "" {
			row = tx.QueryRow(ctx, `INSERT INTO acs.router_models(tenant_id,manufacturer_id,name,manufacturer_model_name,supports_wifi_2_4,supports_wifi_5,supported_auth_modes,product_class,oui,protocol,description,active)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)RETURNING id`, tenantID, input.ManufacturerID, input.Name, input.ManufacturerModelName, input.SupportsWiFi24, input.SupportsWiFi5, authModes, input.ProductClass, input.OUI, input.Protocol, input.Description, active)
		} else {
			row = tx.QueryRow(ctx, `UPDATE acs.router_models SET manufacturer_id=$3,name=$4,manufacturer_model_name=$5,supports_wifi_2_4=$6,supports_wifi_5=$7,supported_auth_modes=$8,product_class=$9,oui=$10,protocol=$11,description=$12,active=$13 WHERE tenant_id=$1 AND id=$2 RETURNING id`, tenantID, id, input.ManufacturerID, input.Name, input.ManufacturerModelName, input.SupportsWiFi24, input.SupportsWiFi5, authModes, input.ProductClass, input.OUI, input.Protocol, input.Description, active)
		}
		if err := row.Scan(&item.ID); err != nil {
			return catalogError(err)
		}
		if id == "" {
			if _, err := tx.Exec(ctx, defaultCWMPParametersSQL, tenantID, item.ID); err != nil {
				return fmt.Errorf("create default CWMP parameter mappings: %w", err)
			}
		}
		var err error
		item, err = r.getModelTx(ctx, tx, tenantID, item.ID)
		return err
	})
	return item, err
}

func (r *Repository) DeleteModel(ctx context.Context, tenantID, id string) error {
	return database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM acs.router_models WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		if err != nil {
			return deleteCatalogError(err)
		}
		if result.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *Repository) ListCWMPParameters(ctx context.Context, tenantID, modelID string) ([]CWMPParameter, error) {
	items := make([]CWMPParameter, 0)
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT true FROM acs.router_models WHERE tenant_id=$1 AND id=$2`, tenantID, modelID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id,router_model_id,parameter_key,parameter_name,xsd_type,active,created_at,updated_at FROM acs.router_model_cwmp_parameters WHERE tenant_id=$1 AND router_model_id=$2 ORDER BY parameter_key`, tenantID, modelID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item CWMPParameter
			if err := scanCWMPParameter(rows, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) ReplaceCWMPParameters(ctx context.Context, tenantID, modelID string, inputs []CWMPParameterInput) ([]CWMPParameter, error) {
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT true FROM acs.router_models WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, modelID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM acs.router_model_cwmp_parameters WHERE tenant_id=$1 AND router_model_id=$2`, tenantID, modelID); err != nil {
			return err
		}
		for _, input := range inputs {
			active := true
			if input.Active != nil {
				active = *input.Active
			}
			if _, err := tx.Exec(ctx, `INSERT INTO acs.router_model_cwmp_parameters(tenant_id,router_model_id,parameter_key,parameter_name,xsd_type,active) VALUES($1,$2,$3,$4,$5,$6)`, tenantID, modelID, input.ParameterKey, input.ParameterName, input.XSDType, active); err != nil {
				return catalogError(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.ListCWMPParameters(ctx, tenantID, modelID)
}

func (r *Repository) SaveHardwareVersion(ctx context.Context, tenantID, modelID, id string, input HardwareVersionInput) (HardwareVersion, error) {
	var item HardwareVersion
	err := database.InTenantTransaction(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		active := true
		if input.Active != nil {
			active = *input.Active
		}
		var err error
		if id == "" {
			err = tx.QueryRow(ctx, `INSERT INTO acs.router_model_hardware_versions(tenant_id,router_model_id,version,description,active)VALUES($1,$2,$3,$4,$5)RETURNING id,router_model_id,version,description,active,created_at,updated_at`, tenantID, modelID, input.Version, input.Description, active).Scan(&item.ID, &item.RouterModelID, &item.Version, &item.Description, &item.Active, &item.CreatedAt, &item.UpdatedAt)
		} else {
			err = tx.QueryRow(ctx, `UPDATE acs.router_model_hardware_versions SET version=$4,description=$5,active=$6 WHERE tenant_id=$1 AND router_model_id=$2 AND id=$3 RETURNING id,router_model_id,version,description,active,created_at,updated_at`, tenantID, modelID, id, input.Version, input.Description, active).Scan(&item.ID, &item.RouterModelID, &item.Version, &item.Description, &item.Active, &item.CreatedAt, &item.UpdatedAt)
		}
		return catalogError(err)
	})
	return item, err
}

func (r *Repository) getModelTx(ctx context.Context, tx pgx.Tx, tenantID, id string) (RouterModel, error) {
	var item RouterModel
	err := scanModel(tx.QueryRow(ctx, `SELECT m.id,m.manufacturer_id,f.name,m.name,m.manufacturer_model_name,m.supports_wifi_2_4,m.supports_wifi_5,m.supported_auth_modes,m.product_class,m.oui,m.protocol,m.description,m.active,m.created_at,m.updated_at FROM acs.router_models m JOIN acs.manufacturers f ON f.id=m.manufacturer_id WHERE m.tenant_id=$1 AND m.id=$2`, tenantID, id), &item)
	return item, err
}

type scanner interface{ Scan(...any) error }

func scanModel(row scanner, item *RouterModel) error {
	return row.Scan(&item.ID, &item.ManufacturerID, &item.ManufacturerName, &item.Name, &item.ManufacturerModelName, &item.SupportsWiFi24, &item.SupportsWiFi5, &item.SupportedAuthModes, &item.ProductClass, &item.OUI, &item.Protocol, &item.Description, &item.Active, &item.CreatedAt, &item.UpdatedAt)
}
func scanCWMPParameter(row scanner, item *CWMPParameter) error {
	return row.Scan(&item.ID, &item.RouterModelID, &item.ParameterKey, &item.ParameterName, &item.XSDType, &item.Active, &item.CreatedAt, &item.UpdatedAt)
}

const defaultCWMPParametersSQL = `INSERT INTO acs.router_model_cwmp_parameters(tenant_id,router_model_id,parameter_key,parameter_name,xsd_type)
SELECT $1,$2,p.parameter_key,p.parameter_name,p.xsd_type FROM (VALUES
('wifi.24.enabled','Device.WiFi.Radio.1.Enable','boolean'),('wifi.24.ssid','Device.WiFi.SSID.1.SSID','string'),
('wifi.24.security_mode','Device.WiFi.AccessPoint.1.Security.ModeEnabled','string'),('wifi.24.password','Device.WiFi.AccessPoint.1.Security.KeyPassphrase','string'),
('wifi.24.channel','Device.WiFi.Radio.1.Channel','unsignedInt'),('wifi.24.bandwidth','Device.WiFi.Radio.1.OperatingChannelBandwidth','string'),
('wifi.5.enabled','Device.WiFi.Radio.2.Enable','boolean'),('wifi.5.ssid','Device.WiFi.SSID.2.SSID','string'),
('wifi.5.security_mode','Device.WiFi.AccessPoint.2.Security.ModeEnabled','string'),('wifi.5.password','Device.WiFi.AccessPoint.2.Security.KeyPassphrase','string'),
('wifi.5.channel','Device.WiFi.Radio.2.Channel','unsignedInt'),('wifi.5.bandwidth','Device.WiFi.Radio.2.OperatingChannelBandwidth','string'),
('dhcp.enabled','Device.DHCPv4.Server.Pool.1.Enable','boolean'),('dhcp.start','Device.DHCPv4.Server.Pool.1.MinAddress','string'),
('dhcp.end','Device.DHCPv4.Server.Pool.1.MaxAddress','string'),('dhcp.gateway','Device.DHCPv4.Server.Pool.1.IPRouters','string'),
('dhcp.subnet_mask','Device.DHCPv4.Server.Pool.1.SubnetMask','string'),('dhcp.lease_seconds','Device.DHCPv4.Server.Pool.1.LeaseTime','int'),
('dns.servers','Device.DHCPv4.Server.Pool.1.DNSServers','string')) AS p(parameter_key,parameter_name,xsd_type)`

func catalogError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return ErrConflict
		}
		if pgErr.Code == "23503" {
			return ErrInvalidReference
		}
	}
	if err != nil {
		return fmt.Errorf("catalog persistence: %w", err)
	}
	return nil
}

func deleteCatalogError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrInUse
	}
	if err != nil {
		return fmt.Errorf("delete catalog item: %w", err)
	}
	return nil
}
