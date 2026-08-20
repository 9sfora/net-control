DROP INDEX IF EXISTS acs.firmwares_one_default_idx;
DROP INDEX IF EXISTS acs.firmwares_catalog_idx;
ALTER TABLE acs.firmwares DROP CONSTRAINT IF EXISTS firmwares_model_version_unique;
ALTER TABLE acs.firmwares ADD COLUMN hardware_version_id UUID;
ALTER TABLE acs.firmwares ADD FOREIGN KEY(tenant_id,hardware_version_id) REFERENCES acs.router_model_hardware_versions(tenant_id,id);
ALTER TABLE acs.firmwares ADD CONSTRAINT firmwares_model_version_unique UNIQUE NULLS NOT DISTINCT(tenant_id,router_model_id,hardware_version_id,version);
CREATE UNIQUE INDEX firmwares_one_default_idx ON acs.firmwares(tenant_id,router_model_id,hardware_version_id) NULLS NOT DISTINCT WHERE is_default AND active;
CREATE INDEX firmwares_catalog_idx ON acs.firmwares(tenant_id,router_model_id,hardware_version_id,active);
