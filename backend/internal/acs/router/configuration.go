package router

type WiFiConfiguration struct {
	Band              string `json:"band"`
	SSID              string `json:"ssid"`
	Password          string `json:"password,omitempty"`
	EncryptedPassword []byte `json:"-"`
	SecurityMode      string `json:"security_mode"`
	Channel           *int   `json:"channel"`
	ChannelMode       string `json:"channel_mode"`
	BandwidthMHz      *int   `json:"bandwidth_mhz"`
	Enabled           bool   `json:"enabled"`
}

type NetworkConfiguration struct {
	DHCPEnabled      bool    `json:"dhcp_enabled"`
	DHCPStart        *string `json:"dhcp_start"`
	DHCPEnd          *string `json:"dhcp_end"`
	DHCPGateway      *string `json:"dhcp_gateway"`
	DHCPSubnetMask   *string `json:"dhcp_subnet_mask"`
	DHCPLeaseSeconds *int    `json:"dhcp_lease_seconds"`
	DNSPrimary       *string `json:"dns_primary"`
	DNSSecondary     *string `json:"dns_secondary"`
}

type ActivationOverrides struct {
	DNSPrimary     *string
	DNSSecondary   *string
	DHCPGateway    *string
	DHCPSubnetMask *string
}

type DesiredConfiguration struct {
	WiFi    []WiFiConfiguration   `json:"wifi"`
	Network *NetworkConfiguration `json:"network"`
}

type ConfigurationResult struct {
	RouterID string `json:"router_id"`
	Version  int64  `json:"version"`
}

type Command struct {
	ID             string         `json:"id"`
	RouterID       string         `json:"router_id"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	Priority       string         `json:"priority"`
	Parameters     map[string]any `json:"parameters"`
	IdempotencyKey *string        `json:"idempotency_key"`
}
