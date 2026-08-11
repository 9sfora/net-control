CREATE TABLE acs.manufacturers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    name TEXT NOT NULL,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT manufacturers_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT manufacturers_tenant_id_id_unique UNIQUE (tenant_id,id)
);
CREATE UNIQUE INDEX manufacturers_tenant_name_unique ON acs.manufacturers(tenant_id,lower(name));

CREATE TABLE acs.router_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    manufacturer_id UUID NOT NULL,
    name TEXT NOT NULL,
    manufacturer_model_name TEXT,
    supports_wifi_2_4 BOOLEAN NOT NULL DEFAULT FALSE,
    supports_wifi_5 BOOLEAN NOT NULL DEFAULT FALSE,
    supported_auth_modes JSONB NOT NULL DEFAULT '[]'::jsonb,
    product_class TEXT,
    oui TEXT,
    protocol TEXT NOT NULL DEFAULT 'TR-069',
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id,manufacturer_id) REFERENCES acs.manufacturers(tenant_id,id),
    CONSTRAINT router_models_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT router_models_protocol CHECK (protocol IN ('TR-069')),
    CONSTRAINT router_models_tenant_id_id_unique UNIQUE (tenant_id,id)
);
CREATE UNIQUE INDEX router_models_tenant_manufacturer_name_unique ON acs.router_models(tenant_id,manufacturer_id,lower(name));

CREATE TABLE acs.router_model_hardware_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    router_model_id UUID NOT NULL,
    version TEXT NOT NULL,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id,router_model_id) REFERENCES acs.router_models(tenant_id,id) ON DELETE CASCADE,
    CONSTRAINT hardware_versions_version_not_blank CHECK (btrim(version) <> ''),
    CONSTRAINT hardware_versions_tenant_id_id_unique UNIQUE (tenant_id,id)
);
CREATE UNIQUE INDEX hardware_versions_model_version_unique ON acs.router_model_hardware_versions(tenant_id,router_model_id,lower(version));

ALTER TABLE acs.routers
    ADD COLUMN router_model_id UUID,
    ADD COLUMN hardware_version_id UUID,
    ADD FOREIGN KEY (tenant_id,router_model_id) REFERENCES acs.router_models(tenant_id,id),
    ADD FOREIGN KEY (tenant_id,hardware_version_id) REFERENCES acs.router_model_hardware_versions(tenant_id,id);

CREATE TRIGGER manufacturers_set_updated_at BEFORE UPDATE ON acs.manufacturers FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER router_models_set_updated_at BEFORE UPDATE ON acs.router_models FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER hardware_versions_set_updated_at BEFORE UPDATE ON acs.router_model_hardware_versions FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

ALTER TABLE acs.manufacturers ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.manufacturers FORCE ROW LEVEL SECURITY;
CREATE POLICY manufacturers_tenant_isolation ON acs.manufacturers
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid);
ALTER TABLE acs.router_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_models FORCE ROW LEVEL SECURITY;
CREATE POLICY router_models_tenant_isolation ON acs.router_models
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid);
ALTER TABLE acs.router_model_hardware_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_model_hardware_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY hardware_versions_tenant_isolation ON acs.router_model_hardware_versions
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid);

GRANT SELECT,INSERT,UPDATE,DELETE ON acs.manufacturers,acs.router_models,acs.router_model_hardware_versions TO netcontrol_app;

INSERT INTO public.permissions(code,description) VALUES
    ('acs.catalog.read','View ACS manufacturers and router models'),
    ('acs.catalog.manage','Manage ACS manufacturers and router models');
INSERT INTO public.role_permissions(tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id FROM public.roles r CROSS JOIN public.permissions p
WHERE r.name='Administrator' AND p.code IN('acs.catalog.read','acs.catalog.manage') ON CONFLICT DO NOTHING;
