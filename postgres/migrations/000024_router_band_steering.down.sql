DELETE FROM acs.router_model_cwmp_parameters
WHERE parameter_key='wifi.band_steering' AND parameter_name='Device.WiFi.X_TP_BandSteering.Enable';
ALTER TABLE acs.routers DROP COLUMN IF EXISTS band_steering_enabled;
