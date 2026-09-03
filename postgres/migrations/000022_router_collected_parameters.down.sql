ALTER TABLE acs.routers
    DROP COLUMN IF EXISTS last_sync_at,
    DROP COLUMN IF EXISTS collected_parameters;
