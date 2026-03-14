package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/models"
)

// ProtocolService manages broker profiles (primary/govt).
type ProtocolService struct {
	repo *secondary.PostgresProtocolRepo
}

func NewProtocolService(repo *secondary.PostgresProtocolRepo) *ProtocolService {
	return &ProtocolService{repo: repo}
}

func (s *ProtocolService) Create(ctx context.Context, projectID, kind, protocol, host string, port int, pub []string, sub []string, serverVendorOrgID string, metadata map[string]any) (models.ProtocolProfile, error) {
	id := uuid.NewString()
	rec := models.ProtocolProfile{
		ID:              id,
		ProjectID:       projectID,
		ServerVendor:    serverVendorOrgID,
		Kind:            kind,
		Protocol:        protocol,
		Host:            host,
		Port:            port,
		PublishTopics:   pub,
		SubscribeTopics: sub,
		Metadata:        metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.repo.Upsert(ctx, rec); err != nil {
		return models.ProtocolProfile{}, fmt.Errorf("protocol upsert: %w", err)
	}
	return rec, nil
}

func (s *ProtocolService) ListByProject(ctx context.Context, projectID string) ([]models.ProtocolProfile, error) {
	return s.repo.GetByProject(ctx, projectID)
}

func (s *ProtocolService) GetByID(ctx context.Context, id string) (*models.ProtocolProfile, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProtocolService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

type ProtocolVersionFilter struct {
	StateID        string
	AuthorityID    string
	ProjectID      string
	ServerVendorID string
}

func (s *ProtocolService) ListProtocolVersions(ctx context.Context, filter ProtocolVersionFilter) ([]models.ProtocolProfile, error) {
	if filter.ProjectID == "" {
		return []models.ProtocolProfile{}, nil
	}
	items, err := s.repo.GetByProject(ctx, filter.ProjectID)
	if err != nil {
		return nil, err
	}
	var out []models.ProtocolProfile
	for _, item := range items {
		if !isProtocolVersion(item.Metadata) {
			continue
		}
		if filter.ServerVendorID != "" && item.ServerVendor != filter.ServerVendorID {
			continue
		}
		if filter.StateID != "" && !metaMatches(item.Metadata, "state_id", filter.StateID) && !metaMatches(item.Metadata, "stateId", filter.StateID) {
			continue
		}
		if filter.AuthorityID != "" && !metaMatches(item.Metadata, "authority_id", filter.AuthorityID) && !metaMatches(item.Metadata, "authorityId", filter.AuthorityID) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *ProtocolService) CreateProtocolVersion(ctx context.Context, projectID, stateID, authorityID, serverVendorID, version, name string, metadata map[string]any) (models.ProtocolProfile, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["type"] = "protocol_version"
	metadata["state_id"] = stateID
	metadata["authority_id"] = authorityID
	metadata["version"] = version
	if name != "" {
		metadata["name"] = name
	}
	host := "protocol-version:" + version
	return s.Create(ctx, projectID, "govt", "mqtt", host, 0, []string{}, []string{}, serverVendorID, metadata)
}

func (s *ProtocolService) UpdateProtocolVersion(ctx context.Context, id string, version *string, name *string, serverVendorID *string, metadata map[string]any) (models.ProtocolProfile, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return models.ProtocolProfile{}, err
	}
	if current == nil {
		return models.ProtocolProfile{}, fmt.Errorf("protocol version not found")
	}
	if current.Metadata == nil {
		current.Metadata = map[string]any{}
	}
	if metadata != nil {
		for k, v := range metadata {
			current.Metadata[k] = v
		}
	}
	current.Metadata["type"] = "protocol_version"
	if version != nil {
		current.Metadata["version"] = *version
		current.Host = "protocol-version:" + *version
	}
	if name != nil {
		current.Metadata["name"] = *name
	}
	if serverVendorID != nil {
		current.ServerVendor = *serverVendorID
	}
	current.UpdatedAt = time.Now()
	if err := s.repo.Upsert(ctx, *current); err != nil {
		return models.ProtocolProfile{}, fmt.Errorf("protocol upsert: %w", err)
	}
	return *current, nil
}

func isProtocolVersion(meta map[string]any) bool {
	if meta == nil {
		return false
	}
	if v, ok := meta["type"].(string); ok && v == "protocol_version" {
		return true
	}
	if v, ok := meta["protocol_version"].(bool); ok {
		return v
	}
	return false
}

func metaMatches(meta map[string]any, key, value string) bool {
	if meta == nil {
		return false
	}
	if v, ok := meta[key]; ok {
		if s, ok := v.(string); ok {
			return s == value
		}
	}
	return false
}
