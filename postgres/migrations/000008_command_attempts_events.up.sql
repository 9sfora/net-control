CREATE TABLE acs.router_command_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    router_command_id UUID NOT NULL,
    session_id UUID NOT NULL,
    attempt_number INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'PROCESSING',
    request_payload JSONB,
    response_payload JSONB,
    error_code TEXT,
    error_message TEXT,
    duration_ms INTEGER,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, router_command_id) REFERENCES acs.router_commands (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, session_id) REFERENCES acs.router_sessions (tenant_id, id),
    CONSTRAINT command_attempts_number_positive CHECK (attempt_number > 0),
    CONSTRAINT command_attempts_status CHECK (status IN ('PROCESSING', 'COMPLETED', 'FAILED')),
    CONSTRAINT command_attempts_command_number_unique UNIQUE (router_command_id, attempt_number)
);

CREATE TABLE acs.router_command_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    router_command_id UUID NOT NULL,
    from_status TEXT,
    to_status TEXT NOT NULL,
    reason TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, router_command_id) REFERENCES acs.router_commands (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX router_command_attempts_session_idx
    ON acs.router_command_attempts (tenant_id, session_id, status, started_at DESC);
CREATE INDEX router_command_events_command_idx
    ON acs.router_command_events (tenant_id, router_command_id, created_at);

ALTER TABLE acs.router_command_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_command_attempts FORCE ROW LEVEL SECURITY;
CREATE POLICY router_command_attempts_tenant_isolation ON acs.router_command_attempts
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE acs.router_command_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_command_events FORCE ROW LEVEL SECURITY;
CREATE POLICY router_command_events_tenant_isolation ON acs.router_command_events
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON acs.router_command_attempts, acs.router_command_events TO netcontrol_app;
