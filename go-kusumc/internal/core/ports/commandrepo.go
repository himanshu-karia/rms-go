package ports

import (
	"context"
	"time"
)

type CommandCatalog struct {
	ID            string
	Name          string
	Scope         string
	ProtocolID    *string
	ModelID       *string
	ProjectID     *string
	PayloadSchema interface{}
	Transport     string
	CreatedAt     time.Time
}

type CommandRequest struct {
	ID            string
	DeviceID      string
	ProjectID     string
	CommandID     string
	Payload       interface{}
	Status        string
	Retries       int
	CorrelationID string
	CreatedAt     time.Time
	PublishedAt   *time.Time
	CompletedAt   *time.Time
}

type CommandResponse struct {
	CorrelationID    string
	DeviceID         string
	ProjectID        string
	RawResponse      interface{}
	Parsed           interface{}
	MatchedPatternID *string
	ReceivedAt       time.Time
}

type CommandRepository interface {
	CreateCatalogEntry(ctx context.Context, c *CommandCatalog) error
	ListCatalogForProject(ctx context.Context, projectID string) ([]*CommandCatalog, error)
	CreateRequest(ctx context.Context, r *CommandRequest) error
	GetRequestByCorrelation(ctx context.Context, correlation string) (*CommandRequest, error)
	SaveResponse(ctx context.Context, resp *CommandResponse) error
}
