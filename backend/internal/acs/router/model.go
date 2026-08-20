package router

import "time"

type Unknown struct {
	ID              string            `json:"id"`
	Manufacturer    string            `json:"manufacturer"`
	OUI             string            `json:"oui"`
	ProductClass    string            `json:"product_class"`
	SerialNumber    string            `json:"serial_number"`
	SoftwareVersion *string           `json:"software_version"`
	HardwareVersion *string           `json:"hardware_version"`
	FirstSeenAt     time.Time         `json:"first_seen_at"`
	LastSeenAt      time.Time         `json:"last_seen_at"`
	LastIPAddress   *string           `json:"last_ip_address"`
	LastUserAgent   *string           `json:"last_user_agent"`
	InformCount     int64             `json:"inform_count"`
	LastEvents      []string          `json:"last_events"`
	LastParameters  map[string]string `json:"last_parameters"`
}

type Router struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Manufacturer      string     `json:"manufacturer"`
	OUI               string     `json:"oui"`
	ProductClass      string     `json:"product_class"`
	SerialNumber      string     `json:"serial_number"`
	SoftwareVersion   *string    `json:"software_version"`
	HardwareVersion   *string    `json:"hardware_version"`
	RouterModelID     *string    `json:"router_model_id"`
	HardwareVersionID *string    `json:"hardware_version_id"`
	DesiredStatus     string     `json:"desired_status"`
	ReportedStatus    string     `json:"reported_status"`
	FirstSeenAt       *time.Time `json:"first_seen_at"`
	LastSeenAt        *time.Time `json:"last_seen_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

type Summary struct {
	Router
	ConnectivityStatus     string     `json:"connectivity_status"`
	LastInformAt           *time.Time `json:"last_inform_at"`
	LastIPAddress          *string    `json:"last_ip_address"`
	ConfigurationVersion   int64      `json:"configuration_version"`
	UpdatedAt              time.Time  `json:"updated_at"`
	RouterModelName        *string    `json:"router_model_name"`
	CatalogHardwareVersion *string    `json:"catalog_hardware_version"`
}

type Session struct {
	ID         string     `json:"id"`
	Protocol   string     `json:"protocol"`
	IPAddress  *string    `json:"ip_address"`
	UserAgent  *string    `json:"user_agent"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type RecentCommand struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	AttemptsCount int        `json:"attempts_count"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

type Detail struct {
	Summary
	WiFi     []WiFiView            `json:"wifi"`
	Network  *NetworkConfiguration `json:"network"`
	Sessions []Session             `json:"sessions"`
	Commands []RecentCommand       `json:"commands"`
}

type WiFiView struct {
	Band               string `json:"band"`
	SSID               string `json:"ssid"`
	PasswordConfigured bool   `json:"password_configured"`
	SecurityMode       string `json:"security_mode"`
	Channel            *int   `json:"channel"`
	ChannelMode        string `json:"channel_mode"`
	BandwidthMHz       *int   `json:"bandwidth_mhz"`
	Enabled            bool   `json:"enabled"`
}

type ListFilter struct {
	Search             string
	ConnectivityStatus string
	DesiredStatus      string
	OnlineCutoff       time.Time
	OfflineCutoff      time.Time
	Limit              int
	Offset             int
}

type ConfigureUnknownInput struct {
	Name              string
	RouterModelID     string
	HardwareVersionID *string
	HardwareVersion   *string
}
