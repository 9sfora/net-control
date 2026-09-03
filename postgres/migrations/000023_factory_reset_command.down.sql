ALTER TABLE acs.router_commands DROP CONSTRAINT router_commands_type;
ALTER TABLE acs.router_commands ADD CONSTRAINT router_commands_type
CHECK (type IN ('ACTIVATE','REBOOT','BLOCK','UPDATE_FIRMWARE','SYNC'));
