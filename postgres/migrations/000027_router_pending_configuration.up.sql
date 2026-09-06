ALTER TABLE acs.routers
    ADD COLUMN configuration_pending BOOLEAN NOT NULL DEFAULT FALSE;
