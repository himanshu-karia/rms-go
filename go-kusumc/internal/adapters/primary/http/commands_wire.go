package http

import (
	"ingestion-go/internal/core/domain"
	"time"
)

type wireCommandCatalog struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Scope         string         `json:"scope"`
	ProtocolID    *string        `json:"protocol_id,omitempty"`
	ModelID       *string        `json:"model_id,omitempty"`
	ProjectID     *string        `json:"project_id,omitempty"`
	PayloadSchema map[string]any `json:"payload_schema,omitempty"`
	Transport     string         `json:"transport"`
	CreatedAt     time.Time      `json:"created_at"`
}

type wireCommandRequest struct {
	ID                 string         `json:"id"`
	DeviceID           string         `json:"device_id"`
	ProjectID          string         `json:"project_id"`
	CommandID          string         `json:"command_id"`
	Payload            map[string]any `json:"payload"`
	Status             string         `json:"status"`
	Retries            int            `json:"retries"`
	CorrelationID      string         `json:"correlation_id"`
	CorrelationIDAlias string         `json:"correlationId,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	PublishedAt        *time.Time     `json:"published_at,omitempty"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

type wireCommandResponse struct {
	CorrelationID      string         `json:"correlation_id"`
	CorrelationIDAlias string         `json:"correlationId,omitempty"`
	DeviceID           string         `json:"device_id"`
	ProjectID          string         `json:"project_id"`
	RawResponse        map[string]any `json:"raw_response,omitempty"`
	Parsed             map[string]any `json:"parsed,omitempty"`
	MatchedPatternID   *string        `json:"matched_pattern_id,omitempty"`
	ReceivedAt         time.Time      `json:"received_at"`
}

type wireCommandWorkerConfig struct {
	IntervalMs int `json:"interval_ms"`
	AgeSeconds int `json:"age_seconds"`
	Batch      int `json:"batch"`
	MaxRetries int `json:"max_retries"`
}

type wireCommandStats struct {
	DeviceID          string                  `json:"device_id"`
	ProjectID         string                  `json:"project_id"`
	StatusCounts      map[string]int          `json:"status_counts"`
	TotalRetries      int                     `json:"total_retries"`
	PendingPastCutoff int                     `json:"pending_past_cutoff"`
	WorkerConfig      wireCommandWorkerConfig `json:"worker_config"`
}

func toWireCommandCatalog(rec domain.CommandCatalog) wireCommandCatalog {
	return wireCommandCatalog{
		ID:            rec.ID,
		Name:          rec.Name,
		Scope:         rec.Scope,
		ProtocolID:    rec.ProtocolID,
		ModelID:       rec.ModelID,
		ProjectID:     rec.ProjectID,
		PayloadSchema: rec.PayloadSchema,
		Transport:     rec.Transport,
		CreatedAt:     rec.CreatedAt,
	}
}

func toWireCommandRequest(rec domain.CommandRequest) wireCommandRequest {
	return wireCommandRequest{
		ID:                 rec.ID,
		DeviceID:           rec.DeviceID,
		ProjectID:          rec.ProjectID,
		CommandID:          rec.CommandID,
		Payload:            rec.Payload,
		Status:             rec.Status,
		Retries:            rec.Retries,
		CorrelationID:      rec.CorrelationID,
		CorrelationIDAlias: rec.CorrelationID,
		CreatedAt:          rec.CreatedAt,
		PublishedAt:        rec.PublishedAt,
		CompletedAt:        rec.CompletedAt,
	}
}

func toWireCommandResponse(rec domain.CommandResponse) wireCommandResponse {
	return wireCommandResponse{
		CorrelationID:      rec.CorrelationID,
		CorrelationIDAlias: rec.CorrelationID,
		DeviceID:           rec.DeviceID,
		ProjectID:          rec.ProjectID,
		RawResponse:        rec.RawResponse,
		Parsed:             rec.Parsed,
		MatchedPatternID:   rec.MatchedPatternID,
		ReceivedAt:         rec.ReceivedAt,
	}
}

func toWireCommandStats(stats domain.CommandStats) wireCommandStats {
	return wireCommandStats{
		DeviceID:          stats.DeviceID,
		ProjectID:         stats.ProjectID,
		StatusCounts:      stats.StatusCounts,
		TotalRetries:      stats.TotalRetries,
		PendingPastCutoff: stats.PendingPastCutoff,
		WorkerConfig: wireCommandWorkerConfig{
			IntervalMs: stats.WorkerConfig.IntervalMs,
			AgeSeconds: stats.WorkerConfig.AgeSeconds,
			Batch:      stats.WorkerConfig.Batch,
			MaxRetries: stats.WorkerConfig.MaxRetries,
		},
	}
}
