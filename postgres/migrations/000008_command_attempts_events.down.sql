DROP TABLE IF EXISTS acs.router_command_events;
DROP TABLE IF EXISTS acs.router_command_attempts;
ALTER TABLE acs.router_sessions DROP CONSTRAINT IF EXISTS router_sessions_tenant_id_id_unique;
