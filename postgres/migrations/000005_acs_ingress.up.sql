CREATE SCHEMA acs;

CREATE TABLE acs.routers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    name TEXT NOT NULL,
    manufacturer TEXT NOT NULL DEFAULT '',
    oui TEXT NOT NULL,
    product_class TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    software_version TEXT,
    hardware_version TEXT,
    desired_status TEXT NOT NULL DEFAULT 'INACTIVE',
    reported_status TEXT NOT NULL DEFAULT 'UNKNOWN',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_inform_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT routers_identity_not_blank CHECK (btrim(oui) <> '' AND btrim(serial_number) <> ''),
    CONSTRAINT routers_tenant_identity_unique UNIQUE (tenant_id, oui, product_class, serial_number),
    CONSTRAINT routers_tenant_id_id_unique UNIQUE (tenant_id, id)
);

CREATE TABLE acs.unknown_routers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    manufacturer TEXT NOT NULL DEFAULT '',
    oui TEXT NOT NULL,
    product_class TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    software_version TEXT,
    hardware_version TEXT,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_ip_address INET,
    last_user_agent TEXT,
    inform_count BIGINT NOT NULL DEFAULT 1,
    last_events JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT unknown_routers_identity_not_blank CHECK (btrim(oui) <> '' AND btrim(serial_number) <> ''),
    CONSTRAINT unknown_routers_tenant_identity_unique UNIQUE (tenant_id, oui, product_class, serial_number),
    CONSTRAINT unknown_routers_tenant_id_id_unique UNIQUE (tenant_id, id)
);

CREATE TABLE acs.router_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    router_id UUID,
    unknown_router_id UUID,
    protocol TEXT NOT NULL DEFAULT 'TR-069',
    ip_address INET,
    user_agent TEXT,
    status TEXT NOT NULL DEFAULT 'OPEN',
    inform_events JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, router_id) REFERENCES acs.routers (tenant_id, id),
    FOREIGN KEY (tenant_id, unknown_router_id) REFERENCES acs.unknown_routers (tenant_id, id),
    CONSTRAINT router_sessions_one_device CHECK (num_nonnulls(router_id, unknown_router_id) = 1)
);

CREATE INDEX routers_tenant_last_seen_idx ON acs.routers (tenant_id, last_seen_at DESC);
CREATE INDEX unknown_routers_tenant_last_seen_idx ON acs.unknown_routers (tenant_id, last_seen_at DESC);
CREATE INDEX router_sessions_tenant_started_idx ON acs.router_sessions (tenant_id, started_at DESC);

CREATE TRIGGER routers_set_updated_at BEFORE UPDATE ON acs.routers
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER unknown_routers_set_updated_at BEFORE UPDATE ON acs.unknown_routers
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

ALTER TABLE acs.routers ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.routers FORCE ROW LEVEL SECURITY;
CREATE POLICY routers_tenant_isolation ON acs.routers
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE acs.unknown_routers ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.unknown_routers FORCE ROW LEVEL SECURITY;
CREATE POLICY unknown_routers_tenant_isolation ON acs.unknown_routers
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE acs.router_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY router_sessions_tenant_isolation ON acs.router_sessions
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

GRANT USAGE ON SCHEMA acs TO netcontrol_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA acs TO netcontrol_app;
ALTER DEFAULT PRIVILEGES FOR ROLE netcontrol IN SCHEMA acs
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO netcontrol_app;
