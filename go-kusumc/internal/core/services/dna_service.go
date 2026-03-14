package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ingestion-go/internal/config/dna"
)

// dnaWriter exposes the minimal persistence contract needed to store DNA records.
type dnaWriter interface {
	UpsertProjectDNA(record dna.ProjectPayloadSchema) error
}

type configSynchronizer interface {
	SyncProject(projectID string)
	InvalidateScopes()
}

// DNAService coordinates read/write operations for project DNA definitions.
type DNAService struct {
	writer dnaWriter
	repo   dna.Repository
	sync   configSynchronizer
}

// NewDNAService creates a DNA service backed by the provided repositories.
func NewDNAService(writer dnaWriter, repo dna.Repository, sync configSynchronizer) *DNAService {
	return &DNAService{writer: writer, repo: repo, sync: sync}
}

// ErrInvalidDNA indicates that the provided record failed validation.
var ErrInvalidDNA = errors.New("invalid project dna")

// ListAll returns every DNA record available. When the repository is not wired,
// it returns an empty slice so callers can handle legacy states gracefully.
func (s *DNAService) ListAll(ctx context.Context) ([]dna.ProjectPayloadSchema, error) {
	if s.repo == nil {
		return []dna.ProjectPayloadSchema{}, nil
	}

	ctx = ensureContext(ctx)
	records, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return []dna.ProjectPayloadSchema{}, nil
	}
	return records, nil
}

// Get fetches a single project DNA record.
func (s *DNAService) Get(ctx context.Context, projectID string) (*dna.ProjectPayloadSchema, error) {
	if s.repo == nil {
		return nil, nil
	}

	ctx = ensureContext(ctx)
	return s.repo.GetByProjectID(ctx, projectID)
}

// Save validates and persists the provided DNA record.
func (s *DNAService) Save(record dna.ProjectPayloadSchema) error {
	if s.writer == nil {
		return errors.New("dna writer not configured")
	}

	record.ProjectID = strings.TrimSpace(record.ProjectID)
	if record.ProjectID == "" {
		return fmt.Errorf("%w: projectId is required", ErrInvalidDNA)
	}

	if _, err := dna.AssembleScopes([]dna.ProjectPayloadSchema{record}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDNA, err)
	}

	if err := s.writer.UpsertProjectDNA(record); err != nil {
		return err
	}

	if s.sync != nil {
		s.sync.InvalidateScopes()
		s.sync.SyncProject(record.ProjectID)
	}

	return nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
