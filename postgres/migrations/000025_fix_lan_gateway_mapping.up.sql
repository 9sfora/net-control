UPDATE acs.router_model_cwmp_parameters
SET parameter_name = 'Device.IP.Interface.1.IPv4Address.1.IPAddress',
    updated_at = now()
WHERE parameter_key = 'dhcp.gateway'
  AND parameter_name = 'Device.DHCPv4.Server.Pool.1.IPRouters';
