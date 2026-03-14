package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/ports"
)

// CommandService contains higher-level operations for commands.
type CommandService struct {
	repo ports.CommandRepository
	pool *pgxpool.Pool
}

// NewCommandService creates a service instance. Pass either a repo implementation
// or nil (then a Postgres repo is created from pool).
func NewCommandService(pool *pgxpool.Pool, repo ports.CommandRepository) *CommandService {
	if repo == nil {
		repo = secondary.NewPostgresCommandRepo(pool)
	}
	return &CommandService{repo: repo, pool: pool}
}

// SendCommand creates a command request and returns the correlation id.
// NOTE: actual transport (publish) is TODO — this method only persists the request.
func (s *CommandService) SendCommand(ctx context.Context, deviceID, projectID, commandID string, payload interface{}) (string, error) {
	correlation := uuid.New().String()
	req := &ports.CommandRequest{
		ID:            uuid.New().String(),
		DeviceID:      deviceID,
		ProjectID:     projectID,
		CommandID:     commandID,
		Payload:       payload,
		Status:        "queued",
		Retries:       0,
		CorrelationID: correlation,
		CreatedAt:     time.Now(),
	}

	if err := s.repo.CreateRequest(ctx, req); err != nil {
		return "", err
	}

	// TODO: enqueue/publish to MQTT or HTTP transport and set status->published, published_at

	return correlation, nil
}
