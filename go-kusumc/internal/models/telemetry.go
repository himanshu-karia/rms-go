package models

import "time"

// TelemetryPayload represents the raw JSON payload arriving via MQTT
type TelemetryPayload struct {
	MsgID         string                 `json:"msgid"`
	Imei          string                 `json:"imei"`
	ProjectID     string                 `json:"projectId,omitempty"`
	ProjectIDAlt  string                 `json:"project_id,omitempty"`
	DeviceUUID    string                 `json:"device_id,omitempty"`
	Timestamp     interface{}            `json:"timestamp"` // Changed to interface{} to accept string/int
	WindSpeed     float64                `json:"windSpeed,omitempty"`
	WindDirection int                    `json:"windDirection,omitempty"`
	Humidity      float64                `json:"humidity,omitempty"`
	Data          map[string]interface{} `json:"data"`
}

// EnrichedPayload represents the data after processing (ready for DB)
type EnrichedPayload struct {
	TelemetryPayload
	Imei              string    `json:"imei"`
	ProjectID         string    `json:"projectId"`
	DeviceUUID        string    `json:"deviceUuid"` // Added for TimescaleDB linking
	ReceivedAt        time.Time `json:"receivedAt"`
	Quality           string    `json:"quality"`
	VerificationError string    `json:"verificationError,omitempty"`
}
