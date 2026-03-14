package secondary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ingestion-go/internal/config/dna"
)

// PostgresDNARepo implements dna.Repository backed by Postgres JSONB columns.
type PostgresDNARepo struct {
	pool *pgxpool.Pool
}

// NewPostgresDNARepo constructs a DNA repository using the provided pool. The
// returned instance is nil when the pool is absent, allowing callers to treat
// the repository as optional.
func NewPostgresDNARepo(pool *pgxpool.Pool) *PostgresDNARepo {
	if pool == nil {
		return nil
	}
	return &PostgresDNARepo{pool: pool}
}

// ListAll fetches every project DNA record available. It gracefully returns an
// empty slice when the underlying table is present but empty.
func (r *PostgresDNARepo) ListAll(ctx context.Context) ([]dna.ProjectPayloadSchema, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT project_id,
			   payload_rows,
			   edge_rules,
			   virtual_sensors,
			   automation_flows,
			   metadata
		FROM project_dna
	`)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
			log.Printf("[ConfigSync] project_dna table missing; skipping DNA load: %s", pgErr.Message)
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	result := make([]dna.ProjectPayloadSchema, 0)
	for rows.Next() {
		record, err := materializeProjectDNA(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetByProjectID fetches a single project DNA record.
func (r *PostgresDNARepo) GetByProjectID(ctx context.Context, projectID string) (*dna.ProjectPayloadSchema, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}

	row := r.pool.QueryRow(ctx, `
		SELECT payload_rows,
		       edge_rules,
		       virtual_sensors,
		       automation_flows,
		       metadata
		FROM project_dna
		WHERE project_id = $1
	`, projectID)

	record, err := materializeSingleRecord(projectID, row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
			log.Printf("[ConfigSync] project_dna table missing; skipping DNA fetch: %s", pgErr.Message)
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

func materializeProjectDNA(rows pgx.Rows) (dna.ProjectPayloadSchema, error) {
	var (
		projectID   string
		rawRows     interface{}
		rawRules    interface{}
		rawSensors  interface{}
		rawFlows    interface{}
		rawMetadata interface{}
	)

	if err := rows.Scan(&projectID, &rawRows, &rawRules, &rawSensors, &rawFlows, &rawMetadata); err != nil {
		return dna.ProjectPayloadSchema{}, err
	}

	return buildProjectDNA(projectID, rawRows, rawRules, rawSensors, rawFlows, rawMetadata)
}

func materializeSingleRecord(projectID string, row pgx.Row) (*dna.ProjectPayloadSchema, error) {
	var (
		rawRows     interface{}
		rawRules    interface{}
		rawSensors  interface{}
		rawFlows    interface{}
		rawMetadata interface{}
	)

	if err := row.Scan(&rawRows, &rawRules, &rawSensors, &rawFlows, &rawMetadata); err != nil {
		return nil, err
	}

	record, err := buildProjectDNA(projectID, rawRows, rawRules, rawSensors, rawFlows, rawMetadata)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func buildProjectDNA(projectID string, rawRows, rawRules, rawSensors, rawFlows, rawMetadata interface{}) (dna.ProjectPayloadSchema, error) {
	record := dna.ProjectPayloadSchema{ProjectID: projectID}

	if err := decodeJSON(rawRows, &record.Rows); err != nil {
		return dna.ProjectPayloadSchema{}, fmt.Errorf("decode payload_rows for %s: %w", projectID, err)
	}
	if err := decodeJSON(rawRules, &record.EdgeRules); err != nil {
		return dna.ProjectPayloadSchema{}, fmt.Errorf("decode edge_rules for %s: %w", projectID, err)
	}
	if err := decodeJSON(rawSensors, &record.VirtualSensors); err != nil {
		return dna.ProjectPayloadSchema{}, fmt.Errorf("decode virtual_sensors for %s: %w", projectID, err)
	}
	if err := decodeJSON(rawFlows, &record.AutomationFlows); err != nil {
		return dna.ProjectPayloadSchema{}, fmt.Errorf("decode automation_flows for %s: %w", projectID, err)
	}
	if err := decodeJSON(rawMetadata, &record.Metadata); err != nil {
		return dna.ProjectPayloadSchema{}, fmt.Errorf("decode metadata for %s: %w", projectID, err)
	}

	return record, nil
}

func decodeJSON(src interface{}, dst interface{}) error {
	if dst == nil {
		return fmt.Errorf("destination must not be nil")
	}
	if src == nil {
		return nil
	}

	switch value := src.(type) {
	case []byte:
		if len(value) == 0 {
			return nil
		}
		return json.Unmarshal(value, dst)
	case string:
		if value == "" {
			return nil
		}
		return json.Unmarshal([]byte(value), dst)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return json.Unmarshal(encoded, dst)
	}
}
