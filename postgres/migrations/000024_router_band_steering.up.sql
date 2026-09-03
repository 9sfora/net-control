ALTER TABLE acs.routers ADD COLUMN band_steering_enabled BOOLEAN;

UPDATE acs.routers
SET band_steering_enabled=(collected_parameters->>'Device.WiFi.X_TP_BandSteering.Enable') IN ('1','true')
WHERE collected_parameters ? 'Device.WiFi.X_TP_BandSteering.Enable';

INSERT INTO acs.router_model_cwmp_parameters(tenant_id,router_model_id,parameter_key,parameter_name,xsd_type)
SELECT tenant_id,id,'wifi.band_steering','Device.WiFi.X_TP_BandSteering.Enable','boolean'
FROM acs.router_models
ON CONFLICT(tenant_id,router_model_id,parameter_key) DO NOTHING;
