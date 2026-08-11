DROP TABLE IF EXISTS acs.router_commands;
DROP TABLE IF EXISTS acs.router_network_settings;
DROP TABLE IF EXISTS acs.router_wifi_radios;
ALTER TABLE acs.routers DROP COLUMN IF EXISTS configuration_version;
