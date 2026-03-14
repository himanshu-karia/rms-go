package services

import (
	"context"
	"errors"
	"testing"

	"ingestion-go/internal/config/dna"
)

type stubDNAWriter struct {
	record dna.ProjectPayloadSchema
	err    error
}

func (s *stubDNAWriter) UpsertProjectDNA(record dna.ProjectPayloadSchema) error {
	s.record = record
	return s.err
}

type stubDNARepo struct {
	records map[string]dna.ProjectPayloadSchema
}

func (s *stubDNARepo) ListAll(ctx context.Context) ([]dna.ProjectPayloadSchema, error) {
	out := make([]dna.ProjectPayloadSchema, 0, len(s.records))
	for _, record := range s.records {
		out = append(out, record)
	}
	return out, nil
}

func (s *stubDNARepo) GetByProjectID(ctx context.Context, projectID string) (*dna.ProjectPayloadSchema, error) {
	record, ok := s.records[projectID]
	if !ok {
		return nil, nil
	}
	return &record, nil
}

type stubSync struct {
	projectIDs  []string
	invalidated bool
}

func (s *stubSync) SyncProject(projectID string) {
	s.projectIDs = append(s.projectIDs, projectID)
}

func (s *stubSync) InvalidateScopes() {
	s.invalidated = true
}

func TestDNAServiceSaveRequiresProjectID(t *testing.T) {
	service := NewDNAService(&stubDNAWriter{}, &stubDNARepo{}, nil)
	err := service.Save(dna.ProjectPayloadSchema{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !errors.Is(err, ErrInvalidDNA) {
		t.Fatalf("expected ErrInvalidDNA, got %v", err)
	}
}

func TestDNAServiceSaveDelegatesToWriter(t *testing.T) {
	writer := &stubDNAWriter{}
	syncStub := &stubSync{}
	service := NewDNAService(writer, &stubDNARepo{}, syncStub)
	record := dna.ProjectPayloadSchema{ProjectID: "project-123"}

	if err := service.Save(record); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer.record.ProjectID != record.ProjectID {
		t.Fatalf("writer not invoked: %+v", writer.record)
	}
	if len(syncStub.projectIDs) != 1 || syncStub.projectIDs[0] != "project-123" {
		t.Fatalf("expected SyncProject to be called with project-123, got %+v", syncStub.projectIDs)
	}
	if !syncStub.invalidated {
		t.Fatalf("expected InvalidateScopes to be called")
	}
}

func TestDNAServiceListAllHandlesNilRepo(t *testing.T) {
	service := NewDNAService(&stubDNAWriter{}, nil, nil)
	records, err := service.ListAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty slice, got %d", len(records))
	}
}

func TestDNAServiceGetReturnsRecord(t *testing.T) {
	repo := &stubDNARepo{records: map[string]dna.ProjectPayloadSchema{
		"proj": {ProjectID: "proj"},
	}}
	service := NewDNAService(&stubDNAWriter{}, repo, nil)

	record, err := service.Get(context.Background(), "proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record == nil || record.ProjectID != "proj" {
		t.Fatalf("unexpected record: %+v", record)
	}
}
