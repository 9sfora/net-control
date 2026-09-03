package cwmp

import (
	"encoding/base64"
	"testing"
)

type testDecryptor struct{}

func (testDecryptor) Decrypt([]byte) (string, error) { return "secret-pass", nil }

func TestMapActivationScopes(t *testing.T) {
	var snapshot activationSnapshot
	snapshot.WiFi = append(snapshot.WiFi, struct {
		Band              string `json:"band"`
		SSID              string `json:"ssid"`
		EncryptedPassword string `json:"encrypted_password"`
		SecurityMode      string `json:"security_mode"`
		Channel           *int   `json:"channel"`
		ChannelMode       string `json:"channel_mode"`
		BandwidthMHz      *int   `json:"bandwidth_mhz"`
		Enabled           bool   `json:"enabled"`
	}{Band: "2.4GHZ", SSID: "Scoped", EncryptedPassword: base64.StdEncoding.EncodeToString([]byte("encrypted"))})
	start, end, gateway, mask, dns := "192.168.4.100", "192.168.4.200", "192.168.4.1", "255.255.255.0", "8.8.8.8"
	snapshot.Network = &struct {
		DHCPEnabled      bool    `json:"dhcp_enabled"`
		DHCPStart        *string `json:"dhcp_start"`
		DHCPEnd          *string `json:"dhcp_end"`
		DHCPGateway      *string `json:"dhcp_gateway"`
		DHCPSubnetMask   *string `json:"dhcp_subnet_mask"`
		DHCPLeaseSeconds *int    `json:"dhcp_lease_seconds"`
		DNSPrimary       *string `json:"dns_primary"`
		DNSSecondary     *string `json:"dns_secondary"`
	}{}
	snapshot.Network.DHCPStart = &start
	snapshot.Network.DHCPEnd = &end
	snapshot.Network.DHCPGateway = &gateway
	snapshot.Network.DHCPSubnetMask = &mask
	snapshot.Network.DNSPrimary = &dns
	dispatcher := &Dispatcher{decryptor: testDecryptor{}}
	mappings := map[string]parameterMapping{
		"wifi.24.ssid":     {Name: "Device.WiFi.SSID.1.SSID", Type: "string"},
		"wifi.24.password": {Name: "Device.WiFi.AccessPoint.1.Security.KeyPassphrase", Type: "string"},
		"dhcp.subnet_mask": {Name: "Device.DHCPv4.Server.Pool.1.SubnetMask", Type: "string"},
		"dhcp.gateway":     {Name: "Device.DHCPv4.Server.Pool.1.IPRouters", Type: "string"},
		"dhcp.start":       {Name: "Device.DHCPv4.Server.Pool.1.MinAddress", Type: "string"},
		"dhcp.end":         {Name: "Device.DHCPv4.Server.Pool.1.MaxAddress", Type: "string"},
		"dns.servers":      {Name: "Device.DHCPv4.Server.Pool.1.DNSServers", Type: "string"},
	}
	tests := []struct {
		scope string
		names []string
	}{{"WIFI_24", []string{"Device.WiFi.SSID.1.SSID", "Device.WiFi.AccessPoint.1.Security.KeyPassphrase"}}, {"DHCP", []string{"Device.DHCPv4.Server.Pool.1.SubnetMask", "Device.DHCPv4.Server.Pool.1.IPRouters", "Device.DHCPv4.Server.Pool.1.MinAddress", "Device.DHCPv4.Server.Pool.1.MaxAddress"}}, {"DNS", []string{"Device.DHCPv4.Server.Pool.1.DNSServers"}}}
	for _, test := range tests {
		values, err := dispatcher.mapActivation(snapshot, test.scope, mappings)
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != len(test.names) {
			t.Fatalf("scope %s generated %d parameters: %+v", test.scope, len(values), values)
		}
		for i, name := range test.names {
			if values[i].Name != name {
				t.Fatalf("scope %s parameter %d = %s, want %s", test.scope, i, values[i].Name, name)
			}
		}
	}
}

func TestMapActivationUsesModelMappings(t *testing.T) {
	var snapshot activationSnapshot
	snapshot.Network = &struct {
		DHCPEnabled      bool    `json:"dhcp_enabled"`
		DHCPStart        *string `json:"dhcp_start"`
		DHCPEnd          *string `json:"dhcp_end"`
		DHCPGateway      *string `json:"dhcp_gateway"`
		DHCPSubnetMask   *string `json:"dhcp_subnet_mask"`
		DHCPLeaseSeconds *int    `json:"dhcp_lease_seconds"`
		DNSPrimary       *string `json:"dns_primary"`
		DNSSecondary     *string `json:"dns_secondary"`
	}{}
	start, end := "192.168.4.100", "192.168.4.200"
	gateway, mask := "192.168.4.1", "255.255.255.0"
	snapshot.Network.DHCPStart = &start
	snapshot.Network.DHCPEnd = &end
	snapshot.Network.DHCPGateway = &gateway
	snapshot.Network.DHCPSubnetMask = &mask
	prefix := "Vendor.Model.DHCP."
	mappings := map[string]parameterMapping{
		"dhcp.subnet_mask": {Name: prefix + "Mask", Type: "string"},
		"dhcp.gateway":     {Name: prefix + "Gateway", Type: "string"},
		"dhcp.start":       {Name: prefix + "Start", Type: "string"},
		"dhcp.end":         {Name: prefix + "End", Type: "string"},
	}
	values, err := (&Dispatcher{decryptor: testDecryptor{}}).mapActivation(snapshot, "DHCP", mappings)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if len(value.Name) < len(prefix) || value.Name[:len(prefix)] != prefix {
			t.Fatalf("unexpected model parameter %s", value.Name)
		}
	}
}

func TestMapActivationFailsWhenMappingIsMissing(t *testing.T) {
	var snapshot activationSnapshot
	snapshot.Network = &struct {
		DHCPEnabled      bool    `json:"dhcp_enabled"`
		DHCPStart        *string `json:"dhcp_start"`
		DHCPEnd          *string `json:"dhcp_end"`
		DHCPGateway      *string `json:"dhcp_gateway"`
		DHCPSubnetMask   *string `json:"dhcp_subnet_mask"`
		DHCPLeaseSeconds *int    `json:"dhcp_lease_seconds"`
		DNSPrimary       *string `json:"dns_primary"`
		DNSSecondary     *string `json:"dns_secondary"`
	}{}
	start := "192.168.4.100"
	snapshot.Network.DHCPStart = &start
	_, err := (&Dispatcher{}).mapActivation(snapshot, "DHCP", map[string]parameterMapping{})
	if err == nil {
		t.Fatal("expected missing mapping error")
	}
}

func TestApplyCollectedWiFiMappingsUsesSSIDLinkedToRadio(t *testing.T) {
	mappings := map[string]parameterMapping{
		"wifi.24.ssid":          {Name: "Device.WiFi.SSID.1.SSID", Type: "string"},
		"wifi.24.security_mode": {Name: "Device.WiFi.AccessPoint.1.Security.ModeEnabled", Type: "string"},
		"wifi.24.password":      {Name: "Device.WiFi.AccessPoint.1.Security.KeyPassphrase", Type: "string"},
		"wifi.5.ssid":           {Name: "Device.WiFi.SSID.2.SSID", Type: "string"},
		"wifi.5.security_mode":  {Name: "Device.WiFi.AccessPoint.2.Security.ModeEnabled", Type: "string"},
		"wifi.5.password":       {Name: "Device.WiFi.AccessPoint.2.Security.KeyPassphrase", Type: "string"},
	}
	collected := map[string]string{
		"Device.WiFi.SSID.1.LowerLayers":          "Device.WiFi.Radio.1.",
		"Device.WiFi.SSID.1.Enable":               "1",
		"Device.WiFi.SSID.2.LowerLayers":          "Device.WiFi.Radio.1.",
		"Device.WiFi.SSID.2.Enable":               "0",
		"Device.WiFi.SSID.3.LowerLayers":          "Device.WiFi.Radio.2.",
		"Device.WiFi.SSID.3.Enable":               "true",
		"Device.WiFi.AccessPoint.1.SSIDReference": "Device.WiFi.SSID.1.",
		"Device.WiFi.AccessPoint.2.SSIDReference": "Device.WiFi.SSID.2.",
		"Device.WiFi.AccessPoint.3.SSIDReference": "Device.WiFi.SSID.3.",
	}

	applyCollectedWiFiMappings(collected, mappings)

	if got := mappings["wifi.5.ssid"].Name; got != "Device.WiFi.SSID.3.SSID" {
		t.Fatalf("5 GHz SSID mapping = %s", got)
	}
	if got := mappings["wifi.5.password"].Name; got != "Device.WiFi.AccessPoint.3.Security.KeyPassphrase" {
		t.Fatalf("5 GHz password mapping = %s", got)
	}
	if got := mappings["wifi.24.ssid"].Name; got != "Device.WiFi.SSID.1.SSID" {
		t.Fatalf("2.4 GHz SSID mapping = %s", got)
	}
}
