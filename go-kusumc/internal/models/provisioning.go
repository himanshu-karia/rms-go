package models

import "time"

// ProtocolProfile models a broker profile (primary or govt) per project.
type ProtocolProfile struct {
	ID              string         `json:"id"`
	ProjectID       string         `json:"project_id"`
	ServerVendor    string         `json:"server_vendor_org_id,omitempty"`
	Kind            string         `json:"kind"`
	Protocol        string         `json:"protocol"`
	Host            string         `json:"host"`
	Port            int            `json:"port"`
	PublishTopics   []string       `json:"publish_topics"`
	SubscribeTopics []string       `json:"subscribe_topics"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// GovtCredentialBundle holds per-device government broker creds (store/return only).
type GovtCredentialBundle struct {
	ID         string         `json:"id"`
	DeviceID   string         `json:"device_id"`
	ProtocolID string         `json:"protocol_id"`
	ClientID   string         `json:"client_id"`
	Username   string         `json:"username"`
	Password   string         `json:"password"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type VFDCommandImportJob struct {
	ID         string                 `json:"id"`
	ProjectID  string                 `json:"project_id"`
	VFDModelID *string                `json:"vfd_model_id"`
	Status     string                 `json:"status"`
	Error      *string                `json:"error"`
	Summary    map[string]interface{} `json:"summary"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// VFDManufacturer represents an OEM of a drive.
type VFDManufacturer struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// VFDModel captures RS485/command/fault metadata for a drive model.
type VFDModel struct {
	ID                 string           `json:"id"`
	ProjectID          string           `json:"project_id"`
	ManufacturerID     string           `json:"manufacturer_id"`
	Model              string           `json:"model"`
	Version            string           `json:"version"`
	RS485              map[string]any   `json:"rs485"`
	RealtimeParameters []map[string]any `json:"realtime_parameters"`
	FaultMap           []map[string]any `json:"fault_map"`
	CommandDictionary  []map[string]any `json:"command_dictionary"`
	Metadata           map[string]any   `json:"metadata,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// ProtocolVFDAssignment binds a protocol profile to a VFD model.
type ProtocolVFDAssignment struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"project_id"`
	ProtocolID string         `json:"protocol_id"`
	VFDModelID string         `json:"vfd_model_id"`
	AssignedBy string         `json:"assigned_by,omitempty"`
	AssignedAt time.Time      `json:"assigned_at"`
	RevokedAt  *time.Time     `json:"revoked_at,omitempty"`
	RevokedBy  *string        `json:"revoked_by,omitempty"`
	Reason     *string        `json:"revocation_reason,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
