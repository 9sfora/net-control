ALTER TABLE acs.routers
    ADD COLUMN configuration_version BIGINT NOT NULL DEFAULT 0;

CREATE TABLE acs.router_wifi_radios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    router_id UUID NOT NULL,
    band TEXT NOT NULL,
    ssid TEXT NOT NULL,
    encrypted_password BYTEA NOT NULL,
    security_mode TEXT NOT NULL DEFAULT 'WPA2_PSK',
    channel INTEGER,
    channel_mode TEXT NOT NULL DEFAULT 'AUTO',
    bandwidth_mhz INTEGER,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, router_id) REFERENCES acs.routers (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT router_wifi_radios_band CHECK (band IN ('2.4GHZ', '5GHZ')),
    CONSTRAINT router_wifi_radios_channel_mode CHECK (channel_mode IN ('AUTO', 'MANUAL')),
    CONSTRAINT router_wifi_radios_security CHECK (security_mode IN ('WPA2_PSK', 'WPA3_SAE', 'WPA2_WPA3')),
    CONSTRAINT router_wifi_radios_channel_valid CHECK (channel IS NULL OR channel BETWEEN 1 AND 196),
    CONSTRAINT router_wifi_radios_band_unique UNIQUE (router_id, band)
);

CREATE TABLE acs.router_network_settings (
    router_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    dhcp_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    dhcp_start INET,
    dhcp_end INET,
    dhcp_lease_seconds INTEGER,
    dns_primary INET,
    dns_secondary INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, router_id) REFERENCES acs.routers (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT router_network_lease_positive CHECK (dhcp_lease_seconds IS NULL OR dhcp_lease_seconds > 0)
);

CREATE TABLE acs.router_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    router_id UUID NOT NULL,
    created_by UUID,
    type TEXT NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'PENDING',
    priority TEXT NOT NULL DEFAULT 'NORMAL',
    attempts_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    idempotency_key TEXT,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_code TEXT,
    error_message TEXT,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, router_id) REFERENCES acs.routers (tenant_id, id),
    FOREIGN KEY (tenant_id, created_by) REFERENCES public.users (tenant_id, id),
    CONSTRAINT router_commands_type CHECK (type IN ('ACTIVATE', 'REBOOT', 'BLOCK', 'UPDATE_FIRMWARE')),
    CONSTRAINT router_commands_status CHECK (status IN ('PENDING', 'WAITING_DEVICE', 'PROCESSING', 'RETRY_SCHEDULED', 'COMPLETED', 'FAILED', 'CANCELED', 'EXPIRED')),
    CONSTRAINT router_commands_priority CHECK (priority IN ('LOW', 'NORMAL', 'HIGH', 'CRITICAL')),
    CONSTRAINT router_commands_attempts CHECK (attempts_count >= 0 AND max_attempts > 0),
    CONSTRAINT router_commands_tenant_id_id_unique UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX router_commands_idempotency_unique
    ON acs.router_commands (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX router_commands_queue_idx
    ON acs.router_commands (tenant_id, status, priority, available_at, created_at);

CREATE TRIGGER router_wifi_radios_set_updated_at BEFORE UPDATE ON acs.router_wifi_radios
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER router_network_settings_set_updated_at BEFORE UPDATE ON acs.router_network_settings
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER router_commands_set_updated_at BEFORE UPDATE ON acs.router_commands
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

ALTER TABLE acs.router_wifi_radios ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_wifi_radios FORCE ROW LEVEL SECURITY;
CREATE POLICY router_wifi_radios_tenant_isolation ON acs.router_wifi_radios
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE acs.router_network_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_network_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY router_network_settings_tenant_isolation ON acs.router_network_settings
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE acs.router_commands ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_commands FORCE ROW LEVEL SECURITY;
CREATE POLICY router_commands_tenant_isolation ON acs.router_commands
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON acs.router_wifi_radios, acs.router_network_settings, acs.router_commands TO netcontrol_app;
