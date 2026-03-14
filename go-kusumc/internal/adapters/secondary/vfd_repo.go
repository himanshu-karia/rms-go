package secondary

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ingestion-go/internal/models"
)

// PostgresVFDRepo handles VFD manufacturers/models and protocol assignments.
type PostgresVFDRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresVFDRepo(pool *pgxpool.Pool) *PostgresVFDRepo {
	if pool == nil {
		return nil
	}
	return &PostgresVFDRepo{pool: pool}
}

func (r *PostgresVFDRepo) CreateManufacturer(ctx context.Context, m models.VFDManufacturer) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
        INSERT INTO vfd_manufacturers (id, project_id, name, metadata)
        VALUES ($1, $2, $3, $4)
    `, m.ID, m.ProjectID, m.Name, m.Metadata)
	return err
}

func (r *PostgresVFDRepo) ListManufacturers(ctx context.Context, projectID string) ([]models.VFDManufacturer, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
        SELECT id, project_id, name, metadata, created_at, updated_at
        FROM vfd_manufacturers
        WHERE project_id = $1
        ORDER BY name ASC
    `, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.VFDManufacturer
	for rows.Next() {
		var rec models.VFDManufacturer
		if err := rows.Scan(&rec.ID, &rec.ProjectID, &rec.Name, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresVFDRepo) CreateModel(ctx context.Context, m models.VFDModel) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
        INSERT INTO vfd_models (id, project_id, manufacturer_id, model, version, rs485, realtime_parameters, fault_map, command_dictionary, metadata)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    `,
		m.ID, m.ProjectID, m.ManufacturerID, m.Model, m.Version, m.RS485, m.RealtimeParameters, m.FaultMap, m.CommandDictionary, m.Metadata,
	)
	return err
}

func (r *PostgresVFDRepo) ListModels(ctx context.Context, projectID string) ([]models.VFDModel, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
        SELECT id, project_id, manufacturer_id, model, version, rs485, realtime_parameters, fault_map, command_dictionary, metadata, created_at, updated_at
        FROM vfd_models
        WHERE project_id = $1
        ORDER BY model ASC, version ASC
    `, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.VFDModel
	for rows.Next() {
		var rec models.VFDModel
		if err := rows.Scan(&rec.ID, &rec.ProjectID, &rec.ManufacturerID, &rec.Model, &rec.Version, &rec.RS485, &rec.RealtimeParameters, &rec.FaultMap, &rec.CommandDictionary, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresVFDRepo) ListModelsByIDs(ctx context.Context, projectID string, ids []string) ([]models.VFDModel, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	if len(ids) == 0 {
		return []models.VFDModel{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, manufacturer_id, model, version, rs485, realtime_parameters, fault_map, command_dictionary, metadata, created_at, updated_at
		FROM vfd_models
		WHERE project_id = $1 AND id = ANY($2)
		ORDER BY model ASC, version ASC
	`, projectID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.VFDModel
	for rows.Next() {
		var rec models.VFDModel
		if err := rows.Scan(&rec.ID, &rec.ProjectID, &rec.ManufacturerID, &rec.Model, &rec.Version, &rec.RS485, &rec.RealtimeParameters, &rec.FaultMap, &rec.CommandDictionary, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresVFDRepo) GetModelByID(ctx context.Context, id string) (*models.VFDModel, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	row := r.pool.QueryRow(ctx, `
		SELECT id, project_id, manufacturer_id, model, version, rs485, realtime_parameters, fault_map, command_dictionary, metadata, created_at, updated_at
		FROM vfd_models
		WHERE id = $1
	`, id)

	var rec models.VFDModel
	if err := row.Scan(&rec.ID, &rec.ProjectID, &rec.ManufacturerID, &rec.Model, &rec.Version, &rec.RS485, &rec.RealtimeParameters, &rec.FaultMap, &rec.CommandDictionary, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func (r *PostgresVFDRepo) UpdateModelArtifacts(ctx context.Context, projectID, modelID string, rs485 map[string]any, realtime, faults, commands []map[string]any) error {
	if r == nil || r.pool == nil {
		return nil
	}
	tag, err := r.pool.Exec(ctx, `
        UPDATE vfd_models
        SET rs485 = $3, realtime_parameters = $4, fault_map = $5, command_dictionary = $6, updated_at = NOW()
        WHERE project_id = $1 AND id = $2
    `, projectID, modelID, rs485, realtime, faults, commands)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("vfd model not found for project")
	}
	return nil
}

func (r *PostgresVFDRepo) UpdateModel(ctx context.Context, m models.VFDModel) error {
	if r == nil || r.pool == nil {
		return nil
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE vfd_models
		SET model = $3,
			version = $4,
			rs485 = $5,
			realtime_parameters = $6,
			fault_map = $7,
			command_dictionary = $8,
			metadata = $9,
			updated_at = NOW()
		WHERE project_id = $1 AND id = $2
	`, m.ProjectID, m.ID, m.Model, m.Version, m.RS485, m.RealtimeParameters, m.FaultMap, m.CommandDictionary, m.Metadata)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("vfd model not found for project")
	}
	return nil
}

func (r *PostgresVFDRepo) CreateAssignment(ctx context.Context, a models.ProtocolVFDAssignment) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
        INSERT INTO protocol_vfd_assignments (id, project_id, protocol_id, vfd_model_id, assigned_by, assigned_at, metadata)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
    `, a.ID, a.ProjectID, a.ProtocolID, a.VFDModelID, a.AssignedBy, a.AssignedAt, a.Metadata)
	return err
}

func (r *PostgresVFDRepo) ListAssignments(ctx context.Context, projectID, protocolID string) ([]models.ProtocolVFDAssignment, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
        SELECT id, project_id, protocol_id, vfd_model_id, assigned_by, assigned_at, revoked_at, revoked_by, revocation_reason, metadata
        FROM protocol_vfd_assignments
        WHERE project_id = $1 AND protocol_id = $2
        ORDER BY assigned_at DESC
    `, projectID, protocolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ProtocolVFDAssignment
	for rows.Next() {
		var rec models.ProtocolVFDAssignment
		if err := rows.Scan(&rec.ID, &rec.ProjectID, &rec.ProtocolID, &rec.VFDModelID, &rec.AssignedBy, &rec.AssignedAt, &rec.RevokedAt, &rec.RevokedBy, &rec.Reason, &rec.Metadata); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresVFDRepo) GetAssignmentFor(ctx context.Context, projectID, protocolID, vfdModelID string) (*models.ProtocolVFDAssignment, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	row := r.pool.QueryRow(ctx, `
		SELECT id, project_id, protocol_id, vfd_model_id, assigned_by, assigned_at, revoked_at, revoked_by, revocation_reason, metadata
		FROM protocol_vfd_assignments
		WHERE project_id = $1 AND protocol_id = $2 AND vfd_model_id = $3 AND revoked_at IS NULL
		ORDER BY assigned_at DESC
		LIMIT 1
	`, projectID, protocolID, vfdModelID)

	var rec models.ProtocolVFDAssignment
	if err := row.Scan(&rec.ID, &rec.ProjectID, &rec.ProtocolID, &rec.VFDModelID, &rec.AssignedBy, &rec.AssignedAt, &rec.RevokedAt, &rec.RevokedBy, &rec.Reason, &rec.Metadata); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func (r *PostgresVFDRepo) RevokeAssignment(ctx context.Context, projectID, assignmentID, reason, revokedBy string, metadata map[string]any) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE protocol_vfd_assignments
		SET revoked_at = $1, revoked_by = $2, revocation_reason = $3,
			metadata = CASE WHEN $4 IS NULL THEN metadata ELSE COALESCE(metadata, '{}'::jsonb) || $4 END
		WHERE id = $5 AND project_id = $6
	`, time.Now(), revokedBy, reason, metadata, assignmentID, projectID)
	return err
}

func (r *PostgresVFDRepo) CreateCommandImportJob(ctx context.Context, job models.VFDCommandImportJob) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO vfd_command_import_jobs (id, project_id, vfd_model_id, status, error, summary)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, job.ID, job.ProjectID, job.VFDModelID, job.Status, job.Error, job.Summary)
	return err
}

func (r *PostgresVFDRepo) UpdateCommandImportJob(ctx context.Context, id string, status string, errMsg *string, summary map[string]interface{}) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE vfd_command_import_jobs
		SET status = $2, error = $3, summary = $4, updated_at = NOW()
		WHERE id = $1
	`, id, status, errMsg, summary)
	return err
}

func (r *PostgresVFDRepo) ListCommandImportJobs(ctx context.Context, projectID string, status []string, limit int) ([]models.VFDCommandImportJob, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	query := `
		SELECT id, project_id, vfd_model_id, status, error, summary, created_at, updated_at
		FROM vfd_command_import_jobs
		WHERE ($1 = '' OR project_id = $1)
		  AND (array_length($2::text[], 1) IS NULL OR status = ANY($2))
		ORDER BY created_at DESC
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, query, projectID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.VFDCommandImportJob
	for rows.Next() {
		var rec models.VFDCommandImportJob
		if err := rows.Scan(&rec.ID, &rec.ProjectID, &rec.VFDModelID, &rec.Status, &rec.Error, &rec.Summary, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
