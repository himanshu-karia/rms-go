package dna

import (
	"context"
	"fmt"

	"ingestion-go/internal/config/payloadschema"
)

// Repository exposes persistence for project DNA records.
type Repository interface {
	ListAll(ctx context.Context) ([]ProjectPayloadSchema, error)
	GetByProjectID(ctx context.Context, projectID string) (*ProjectPayloadSchema, error)
}

// AssembleScopes converts persisted rows into scope schemas compatible with the
// existing payloadschema helpers.
func AssembleScopes(records []ProjectPayloadSchema) (map[string]payloadschema.ScopeSchema, error) {
	if len(records) == 0 {
		return nil, nil
	}

	entries := make([]payloadschema.Entry, 0)
	for _, record := range records {
		if record.ProjectID == "" {
			return nil, fmt.Errorf("project dna missing project id")
		}
		rows := normaliseRows(record.Rows, record.ProjectID)
		entries = append(entries, rows...)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	scopes := payloadschema.BuildScopes(entries)
	return scopes, nil
}

func normaliseRows(rows []payloadschema.Entry, projectID string) []payloadschema.Entry {
	if len(rows) == 0 {
		return nil
	}

	normalised := make([]payloadschema.Entry, 0, len(rows))
	for _, row := range rows {
		entry := row
		if entry.ExpectedFor == "" {
			entry.ExpectedFor = "project"
		}
		if entry.ExpectedFor == "project" && entry.ScopeID == "" {
			entry.ScopeID = projectID
		}
		normalised = append(normalised, entry)
	}
	return normalised
}
