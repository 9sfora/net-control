ALTER TABLE acs.router_commands
    ADD COLUMN last_attempt_at TIMESTAMPTZ,
    ADD COLUMN next_attempt_at TIMESTAMPTZ,
    ADD COLUMN expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '24 hours'),
    ADD COLUMN canceled_at TIMESTAMPTZ;

CREATE INDEX router_commands_expiration_idx
    ON acs.router_commands (tenant_id, expires_at)
    WHERE status IN ('PENDING', 'WAITING_DEVICE', 'RETRY_SCHEDULED');

CREATE INDEX router_commands_processing_idx
    ON acs.router_commands (tenant_id, last_attempt_at)
    WHERE status = 'PROCESSING';
