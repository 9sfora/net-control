CREATE TABLE acs.router_model_cwmp_parameters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    router_model_id UUID NOT NULL,
    parameter_key TEXT NOT NULL,
    parameter_name TEXT NOT NULL,
    xsd_type TEXT NOT NULL DEFAULT 'string',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id,router_model_id) REFERENCES acs.router_models(tenant_id,id) ON DELETE CASCADE,
    CONSTRAINT router_model_cwmp_parameter_key_not_blank CHECK (btrim(parameter_key) <> ''),
    CONSTRAINT router_model_cwmp_parameter_name_not_blank CHECK (btrim(parameter_name) <> ''),
    CONSTRAINT router_model_cwmp_parameter_type CHECK (xsd_type IN ('string','boolean','int','unsignedInt')),
    CONSTRAINT router_model_cwmp_parameters_tenant_id_id_unique UNIQUE (tenant_id,id),
    CONSTRAINT router_model_cwmp_parameters_model_key_unique UNIQUE (tenant_id,router_model_id,parameter_key)
);

CREATE TRIGGER router_model_cwmp_parameters_set_updated_at
BEFORE UPDATE ON acs.router_model_cwmp_parameters
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

ALTER TABLE acs.router_model_cwmp_parameters ENABLE ROW LEVEL SECURITY;
ALTER TABLE acs.router_model_cwmp_parameters FORCE ROW LEVEL SECURITY;
CREATE POLICY router_model_cwmp_parameters_tenant_isolation ON acs.router_model_cwmp_parameters
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid);

GRANT SELECT,INSERT,UPDATE,DELETE ON acs.router_model_cwmp_parameters TO netcontrol_app;

INSERT INTO acs.router_model_cwmp_parameters
    (tenant_id,router_model_id,parameter_key,parameter_name,xsd_type)
SELECT m.tenant_id,m.id,p.parameter_key,p.parameter_name,p.xsd_type
FROM acs.router_models m
CROSS JOIN (VALUES
    ('wifi.24.enabled','Device.WiFi.Radio.1.Enable','boolean'),
    ('wifi.24.ssid','Device.WiFi.SSID.1.SSID','string'),
    ('wifi.24.security_mode','Device.WiFi.AccessPoint.1.Security.ModeEnabled','string'),
    ('wifi.24.password','Device.WiFi.AccessPoint.1.Security.KeyPassphrase','string'),
    ('wifi.24.channel','Device.WiFi.Radio.1.Channel','unsignedInt'),
    ('wifi.24.bandwidth','Device.WiFi.Radio.1.OperatingChannelBandwidth','string'),
    ('wifi.5.enabled','Device.WiFi.Radio.2.Enable','boolean'),
    ('wifi.5.ssid','Device.WiFi.SSID.2.SSID','string'),
    ('wifi.5.security_mode','Device.WiFi.AccessPoint.2.Security.ModeEnabled','string'),
    ('wifi.5.password','Device.WiFi.AccessPoint.2.Security.KeyPassphrase','string'),
    ('wifi.5.channel','Device.WiFi.Radio.2.Channel','unsignedInt'),
    ('wifi.5.bandwidth','Device.WiFi.Radio.2.OperatingChannelBandwidth','string'),
    ('dhcp.enabled','Device.DHCPv4.Server.Pool.1.Enable','boolean'),
    ('dhcp.start','Device.DHCPv4.Server.Pool.1.MinAddress','string'),
    ('dhcp.end','Device.DHCPv4.Server.Pool.1.MaxAddress','string'),
    ('dhcp.gateway','Device.DHCPv4.Server.Pool.1.IPRouters','string'),
    ('dhcp.subnet_mask','Device.DHCPv4.Server.Pool.1.SubnetMask','string'),
    ('dhcp.lease_seconds','Device.DHCPv4.Server.Pool.1.LeaseTime','int'),
    ('dns.servers','Device.DHCPv4.Server.Pool.1.DNSServers','string')
) AS p(parameter_key,parameter_name,xsd_type);
