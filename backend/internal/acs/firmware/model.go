package firmware

import "time"

type Firmware struct {
	ID                string     `json:"id"`
	RouterModelID     string     `json:"router_model_id"`
	RouterModelName   string     `json:"router_model_name"`
	HardwareVersionID *string    `json:"hardware_version_id"`
	HardwareVersion   *string    `json:"hardware_version"`
	Name              string     `json:"name"`
	Version           string     `json:"version"`
	Description       *string    `json:"description"`
	FileName          *string    `json:"file_name"`
	FileSize          *int64     `json:"file_size"`
	ChecksumSHA256    *string    `json:"checksum_sha256"`
	ReleaseNotes      *string    `json:"release_notes"`
	Status            string     `json:"status"`
	IsDefault         bool       `json:"is_default"`
	Active            bool       `json:"active"`
	UploadedAt        *time.Time `json:"uploaded_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
type Input struct {
	RouterModelID     string  `json:"router_model_id"`
	HardwareVersionID *string `json:"hardware_version_id"`
	Name              string  `json:"name"`
	Version           string  `json:"version"`
	Description       *string `json:"description"`
	ReleaseNotes      *string `json:"release_notes"`
	IsDefault         bool    `json:"is_default"`
	Active            *bool   `json:"active"`
}
