package catalog

import (
	"encoding/json"
	"time"
)

type Manufacturer struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type HardwareVersion struct {
	ID            string    `json:"id"`
	RouterModelID string    `json:"router_model_id"`
	Version       string    `json:"version"`
	Description   *string   `json:"description"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type RouterModel struct {
	ID                    string            `json:"id"`
	ManufacturerID        string            `json:"manufacturer_id"`
	ManufacturerName      string            `json:"manufacturer_name"`
	TR069DataModelID      *string           `json:"tr069_data_model_id"`
	TR069DataModelName    *string           `json:"tr069_data_model_name"`
	Name                  string            `json:"name"`
	ManufacturerModelName *string           `json:"manufacturer_model_name"`
	SupportsWiFi24        bool              `json:"supports_wifi_2_4"`
	SupportsWiFi5         bool              `json:"supports_wifi_5"`
	SupportedAuthModes    json.RawMessage   `json:"supported_auth_modes"`
	ProductClass          *string           `json:"product_class"`
	OUI                   *string           `json:"oui"`
	Protocol              string            `json:"protocol"`
	Description           *string           `json:"description"`
	Active                bool              `json:"active"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
	HardwareVersions      []HardwareVersion `json:"hardware_versions,omitempty"`
	CWMPParameters        []CWMPParameter   `json:"cwmp_parameters,omitempty"`
}
type CWMPParameter struct {
	ID            string    `json:"id"`
	RouterModelID string    `json:"router_model_id"`
	ParameterKey  string    `json:"parameter_key"`
	ParameterName string    `json:"parameter_name"`
	XSDType       string    `json:"xsd_type"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type CWMPParameterInput struct {
	ParameterKey  string `json:"parameter_key"`
	ParameterName string `json:"parameter_name"`
	XSDType       string `json:"xsd_type"`
	Active        *bool  `json:"active"`
}
type CWMPParametersInput struct {
	Parameters []CWMPParameterInput `json:"parameters"`
}
type ManufacturerInput struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Active      *bool   `json:"active"`
}
type RouterModelInput struct {
	ManufacturerID        string   `json:"manufacturer_id"`
	TR069DataModelID      *string  `json:"tr069_data_model_id"`
	Name                  string   `json:"name"`
	ManufacturerModelName *string  `json:"manufacturer_model_name"`
	SupportsWiFi24        bool     `json:"supports_wifi_2_4"`
	SupportsWiFi5         bool     `json:"supports_wifi_5"`
	SupportedAuthModes    []string `json:"supported_auth_modes"`
	ProductClass          *string  `json:"product_class"`
	OUI                   *string  `json:"oui"`
	Protocol              string   `json:"protocol"`
	Description           *string  `json:"description"`
	Active                *bool    `json:"active"`
}
type TR069Command struct {
	ID            string    `json:"id"`
	DataModelID   string    `json:"data_model_id"`
	ParameterPath string    `json:"parameter_path"`
	ValueType     string    `json:"value_type"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type TR069DataModel struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Active    bool           `json:"active"`
	Commands  []TR069Command `json:"commands"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
type TR069CommandInput struct {
	ParameterPath string `json:"parameter_path"`
	ValueType     string `json:"value_type"`
	Active        *bool  `json:"active"`
}
type TR069DataModelInput struct {
	Name     string              `json:"name"`
	Active   *bool               `json:"active"`
	Commands []TR069CommandInput `json:"commands"`
}
type HardwareVersionInput struct {
	Version     string  `json:"version"`
	Description *string `json:"description"`
	Active      *bool   `json:"active"`
}
