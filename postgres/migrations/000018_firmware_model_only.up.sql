DROP INDEX IF EXISTS acs.firmwares_one_default_idx;
DROP INDEX IF EXISTS acs.firmwares_catalog_idx;
ALTER TABLE acs.firmwares DROP CONSTRAINT IF EXISTS firmwares_model_version_unique;
ALTER TABLE acs.firmwares DROP COLUMN hardware_version_id;
ALTER TABLE acs.firmwares ADD CONSTRAINT firmwares_model_version_unique UNIQUE(tenant_id,router_model_id,version);
CREATE UNIQUE INDEX firmwares_one_default_idx ON acs.firmwares(tenant_id,router_model_id) WHERE is_default AND active;
CREATE INDEX firmwares_catalog_idx ON acs.firmwares(tenant_id,router_model_id,active);
