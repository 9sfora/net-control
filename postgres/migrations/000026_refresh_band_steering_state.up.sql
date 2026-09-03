UPDATE acs.routers
SET band_steering_enabled = lower(collected_parameters->>'Device.WiFi.X_TP_BandSteering.Enable') IN ('1','true','enabled')
WHERE collected_parameters ? 'Device.WiFi.X_TP_BandSteering.Enable';
