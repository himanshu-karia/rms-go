package domain

import "time"

type OTACampaign struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Version       string                 `json:"version"`
	S3Url         string                 `json:"s3Url"`
	Checksum      string                 `json:"checksum"`
	ProjectType   string                 `json:"projectType"`
	Status        string                 `json:"status"` // pending, active, completed
	StartedAt     time.Time              `json:"startedAt"`
	Stats         map[string]interface{} `json:"stats"` // total, pending, success, failed
	TargetDevices []string               `json:"targetDevices"`
}

type Rule struct {
	ID        string                   `json:"id"`
	ProjectID string                   `json:"projectId"`
	Name      string                   `json:"name"`
	DeviceID  string                   `json:"deviceId"`
	Trigger   map[string]interface{}   `json:"trigger"`
	Actions   []map[string]interface{} `json:"actions"`
	Enabled   bool                     `json:"enabled"`
}

type CommandCatalog struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Scope         string         `json:"scope"`
	ProtocolID    *string        `json:"protocolId,omitempty"`
	ModelID       *string        `json:"modelId,omitempty"`
	ProjectID     *string        `json:"projectId,omitempty"`
	PayloadSchema map[string]any `json:"payloadSchema,omitempty"`
	Transport     string         `json:"transport"`
	CreatedAt     time.Time      `json:"createdAt"`
}

type DeviceCapability struct {
	DeviceID  string `json:"deviceId"`
	CommandID string `json:"commandId"`
}

type ProjectCommandOverride struct {
	ProjectID string `json:"projectId"`
	CommandID string `json:"commandId"`
	Enabled   bool   `json:"enabled"`
}

type ResponsePattern struct {
	ID          string         `json:"id"`
	CommandID   string         `json:"commandId"`
	PatternType string         `json:"patternType"`
	Pattern     string         `json:"pattern"`
	Success     bool           `json:"success"`
	Extract     map[string]any `json:"extract,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type CommandRequest struct {
	ID            string         `json:"id"`
	DeviceID      string         `json:"deviceId"`
	ProjectID     string         `json:"projectId"`
	CommandID     string         `json:"commandId"`
	Payload       map[string]any `json:"payload"`
	Status        string         `json:"status"`
	Retries       int            `json:"retries"`
	CorrelationID string         `json:"correlationId"`
	CreatedAt     time.Time      `json:"createdAt"`
	PublishedAt   *time.Time     `json:"publishedAt,omitempty"`
	CompletedAt   *time.Time     `json:"completedAt,omitempty"`
}

type CommandResponse struct {
	CorrelationID    string         `json:"correlationId"`
	DeviceID         string         `json:"deviceId"`
	ProjectID        string         `json:"projectId"`
	RawResponse      map[string]any `json:"rawResponse,omitempty"`
	Parsed           map[string]any `json:"parsed,omitempty"`
	MatchedPatternID *string        `json:"matchedPatternId,omitempty"`
	ReceivedAt       time.Time      `json:"receivedAt"`
}

// CommandWorkerConfig exposes retry worker tunables for observability.
type CommandWorkerConfig struct {
	IntervalMs int `json:"intervalMs"`
	AgeSeconds int `json:"ageSeconds"`
	Batch      int `json:"batch"`
	MaxRetries int `json:"maxRetries"`
}

// CommandStats aggregates per-device command status counters and retry posture.
type CommandStats struct {
	DeviceID          string              `json:"deviceId"`
	ProjectID         string              `json:"projectId"`
	StatusCounts      map[string]int      `json:"statusCounts"`
	TotalRetries      int                 `json:"totalRetries"`
	PendingPastCutoff int                 `json:"pendingPastCutoff"`
	WorkerConfig      CommandWorkerConfig `json:"workerConfig"`
}
