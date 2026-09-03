ALTER TABLE acs.router_commands DROP CONSTRAINT router_commands_type;
ALTER TABLE acs.router_commands ADD CONSTRAINT router_commands_type
CHECK (type IN ('ACTIVATE','REBOOT','FACTORY_RESET','BLOCK','UPDATE_FIRMWARE','SYNC'));
