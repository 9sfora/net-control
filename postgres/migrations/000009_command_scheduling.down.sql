DROP INDEX IF EXISTS acs.router_commands_processing_idx;
DROP INDEX IF EXISTS acs.router_commands_expiration_idx;
ALTER TABLE acs.router_commands
    DROP COLUMN IF EXISTS canceled_at,
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS last_attempt_at;
