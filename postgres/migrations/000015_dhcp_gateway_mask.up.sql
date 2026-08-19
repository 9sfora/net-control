ALTER TABLE acs.router_network_settings
    ADD COLUMN dhcp_gateway INET,
    ADD COLUMN dhcp_subnet_mask INET;
