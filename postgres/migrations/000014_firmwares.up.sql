CREATE TABLE acs.firmwares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    router_model_id UUID NOT NULL,
    hardware_version_id UUID,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    description TEXT,
    file_path TEXT,
    file_name TEXT,
    file_size BIGINT,
    checksum_sha256 TEXT,
    release_notes TEXT,
    status TEXT NOT NULL DEFAULT 'NO_FILE',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    uploaded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id,router_model_id) REFERENCES acs.router_models(tenant_id,id),
    FOREIGN KEY (tenant_id,hardware_version_id) REFERENCES acs.router_model_hardware_versions(tenant_id,id),
    CONSTRAINT firmwares_name_version_not_blank CHECK(btrim(name)<>'' AND btrim(version)<>''),
    CONSTRAINT firmwares_status CHECK(status IN('NO_FILE','READY')),
    CONSTRAINT firmwares_file_consistency CHECK(
        (status='NO_FILE' AND file_path IS NULL AND file_size IS NULL AND checksum_sha256 IS NULL AND uploaded_at IS NULL) OR
        (status='READY' AND file_path IS NOT NULL AND file_size>=0 AND checksum_sha256 ~ '^[0-9a-f]{64}$' AND uploaded_at IS NOT NULL)
    ),
    CONSTRAINT firmwares_tenant_id_id_unique UNIQUE(tenant_id,id),
    CONSTRAINT firmwares_model_version_unique UNIQUE NULLS NOT DISTINCT(tenant_id,router_model_id,hardware_version_id,version)
);
CREATE UNIQUE INDEX firmwares_one_default_idx ON acs.firmwares(tenant_id,router_model_id,hardware_version_id) NULLS NOT DISTINCT WHERE is_default AND active;
CREATE INDEX firmwares_catalog_idx ON acs.firmwares(tenant_id,router_model_id,hardware_version_id,active);
CREATE TRIGGER firmwares_set_updated_at BEFORE UPDATE ON acs.firmwares FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
ALTER TABLE acs.firmwares ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.firmwares FORCE ROW LEVEL SECURITY;
CREATE POLICY firmwares_tenant_isolation ON acs.firmwares
    USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid)
    WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid);
GRANT SELECT,INSERT,UPDATE,DELETE ON acs.firmwares TO netcontrol_app;
