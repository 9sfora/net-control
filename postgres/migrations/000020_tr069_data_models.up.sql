CREATE TABLE acs.tr069_data_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tr069_data_models_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT tr069_data_models_tenant_id_id_unique UNIQUE (tenant_id,id)
);
CREATE UNIQUE INDEX tr069_data_models_tenant_name_unique ON acs.tr069_data_models(tenant_id,lower(name));

CREATE TABLE acs.tr069_data_model_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    data_model_id UUID NOT NULL,
    parameter_path TEXT NOT NULL,
    value_type TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id,data_model_id) REFERENCES acs.tr069_data_models(tenant_id,id) ON DELETE CASCADE,
    CONSTRAINT tr069_data_model_commands_path_not_blank CHECK (btrim(parameter_path) <> ''),
    CONSTRAINT tr069_data_model_commands_type CHECK (value_type IN ('string','boolean','int','unsignedInt','dateTime','base64','hexBinary')),
    CONSTRAINT tr069_data_model_commands_path_unique UNIQUE (data_model_id,parameter_path)
);

ALTER TABLE acs.router_models ADD COLUMN tr069_data_model_id UUID;
ALTER TABLE acs.router_models ADD FOREIGN KEY(tenant_id,tr069_data_model_id) REFERENCES acs.tr069_data_models(tenant_id,id);

CREATE TRIGGER tr069_data_models_set_updated_at BEFORE UPDATE ON acs.tr069_data_models FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER tr069_data_model_commands_set_updated_at BEFORE UPDATE ON acs.tr069_data_model_commands FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

ALTER TABLE acs.tr069_data_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.tr069_data_models FORCE ROW LEVEL SECURITY;
CREATE POLICY tr069_data_models_tenant_isolation ON acs.tr069_data_models
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid);
ALTER TABLE acs.tr069_data_model_commands ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.tr069_data_model_commands FORCE ROW LEVEL SECURITY;
CREATE POLICY tr069_data_model_commands_tenant_isolation ON acs.tr069_data_model_commands
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid);

GRANT SELECT,INSERT,UPDATE,DELETE ON acs.tr069_data_models,acs.tr069_data_model_commands TO netcontrol_app;
