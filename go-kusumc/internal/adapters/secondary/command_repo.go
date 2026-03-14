package secondary

import (
	"context"
	"time"

	"ingestion-go/internal/core/ports"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCommandRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresCommandRepo(pool *pgxpool.Pool) *PostgresCommandRepo {
	return &PostgresCommandRepo{pool: pool}
}

func (r *PostgresCommandRepo) CreateCatalogEntry(ctx context.Context, c *ports.CommandCatalog) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `INSERT INTO command_catalog (id, name, scope, protocol_id, model_id, project_id, payload_schema, transport, created_at)
              VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.pool.Exec(ctx, query, c.ID, c.Name, c.Scope, c.ProtocolID, c.ModelID, c.ProjectID, c.PayloadSchema, c.Transport, c.CreatedAt)
	return err
}

func (r *PostgresCommandRepo) ListCatalogForProject(ctx context.Context, projectID string) ([]*ports.CommandCatalog, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, `SELECT id, name, scope, protocol_id, model_id, project_id, payload_schema, transport, created_at FROM command_catalog WHERE project_id = $1 OR scope = 'core'`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ports.CommandCatalog
	for rows.Next() {
		var c ports.CommandCatalog
		if err := rows.Scan(&c.ID, &c.Name, &c.Scope, &c.ProtocolID, &c.ModelID, &c.ProjectID, &c.PayloadSchema, &c.Transport, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, &c)
	}
	return out, nil
}

func (r *PostgresCommandRepo) CreateRequest(ctx context.Context, req *ports.CommandRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `INSERT INTO command_requests (id, device_id, project_id, command_id, payload, status, retries, correlation_id, created_at)
              VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.pool.Exec(ctx, query, req.ID, req.DeviceID, req.ProjectID, req.CommandID, req.Payload, req.Status, req.Retries, req.CorrelationID, req.CreatedAt)
	return err
}

func (r *PostgresCommandRepo) GetRequestByCorrelation(ctx context.Context, correlation string) (*ports.CommandRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var rr ports.CommandRequest
	query := `SELECT id, device_id, project_id, command_id, payload, status, retries, correlation_id, created_at, published_at, completed_at FROM command_requests WHERE correlation_id = $1`
	err := r.pool.QueryRow(ctx, query, correlation).Scan(&rr.ID, &rr.DeviceID, &rr.ProjectID, &rr.CommandID, &rr.Payload, &rr.Status, &rr.Retries, &rr.CorrelationID, &rr.CreatedAt, &rr.PublishedAt, &rr.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &rr, nil
}

func (r *PostgresCommandRepo) SaveResponse(ctx context.Context, resp *ports.CommandResponse) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `INSERT INTO command_responses (correlation_id, device_id, project_id, raw_response, parsed, matched_pattern_id, received_at)
              VALUES ($1,$2,$3,$4,$5,$6,$7)
              ON CONFLICT (correlation_id) DO UPDATE SET raw_response = EXCLUDED.raw_response, parsed = EXCLUDED.parsed, matched_pattern_id = EXCLUDED.matched_pattern_id, received_at = EXCLUDED.received_at`

	_, err := r.pool.Exec(ctx, query, resp.CorrelationID, resp.DeviceID, resp.ProjectID, resp.RawResponse, resp.Parsed, resp.MatchedPatternID, resp.ReceivedAt)
	return err
}
