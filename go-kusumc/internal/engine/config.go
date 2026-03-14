package engine

import "ingestion-go/internal/config/payloadschema"

// ProjectConfig represents the minimal project DNA needed for ingestion.
type ProjectConfig struct {
	ID             string                                `json:"id"`
	Type           string                                `json:"type"`
	Hardware       HardwareConfig                        `json:"hardware"`
	Firmware       FirmwareConfig                        `json:"firmware"`
	PayloadSchemas map[string]payloadschema.PacketSchema `json:"payloadSchemas,omitempty"`
}

type HardwareConfig struct {
	Sensors []SensorConfig `json:"sensors"`
}

type SensorConfig struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`  // gpio, modbus, etc
	Param string `json:"param"` // canonical key for payload
	Unit  string `json:"unit"`

	// Transformation Logic
	TransformMode string  `json:"transformMode"`
	RawMin        float64 `json:"raw_min,omitempty"`
	RawMax        float64 `json:"raw_max,omitempty"`
	Min           float64 `json:"min,omitempty"`
	Max           float64 `json:"max,omitempty"`

	// Digital Mapping
	Digital0Label string `json:"digital_0_label,omitempty"`
	Digital1Label string `json:"digital_1_label,omitempty"`

	// Expression
	Expression string `json:"expression,omitempty"`
}

type FirmwareConfig struct {
	Version  string `json:"version"`
	Interval int    `json:"interval"`
}

// RuleConfig represents a rule to be evaluated
type RuleConfig struct {
	ID        string         `json:"_id"`
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	ProjectID string         `json:"projectId"`
	DeviceID  string         `json:"deviceId,omitempty"`
	Trigger   TriggerConfig  `json:"trigger"`
	Actions   []ActionConfig `json:"actions"`
}

type TriggerConfig struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // >, <, =, etc
	Value    interface{} `json:"value"`
}

type ActionConfig struct {
	Type     string `json:"type"`
	Target   string `json:"target"`
	Payload  string `json:"payload"`
	Cooldown int    `json:"cooldown"`
}
