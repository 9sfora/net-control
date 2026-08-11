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
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Manufacturer    string    `json:"manufacturer"`
	OUI             string    `json:"oui"`
	ProductClass    string    `json:"product_class"`
	SerialNumber    string    `json:"serial_number"`
	SoftwareVersion *string   `json:"software_version"`
	HardwareVersion *string   `json:"hardware_version"`
	DesiredStatus   string    `json:"desired_status"`
	ReportedStatus  string    `json:"reported_status"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	CreatedAt       time.Time `json:"created_at"`
}
