DELETE FROM public.permissions WHERE code IN('acs.catalog.read','acs.catalog.manage');
ALTER TABLE acs.routers DROP COLUMN IF EXISTS hardware_version_id,DROP COLUMN IF EXISTS router_model_id;
DROP TABLE IF EXISTS acs.router_model_hardware_versions;
DROP TABLE IF EXISTS acs.router_models;
DROP TABLE IF EXISTS acs.manufacturers;
