package secondary

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresProjectRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresProjectRepo(pool *pgxpool.Pool) *PostgresProjectRepo {
	return &PostgresProjectRepo{pool: pool}
}

type ProjectRecord struct {
	ID       string
	Name     string
	Type     string
	Location string
	Config   interface{} // map
}

func (r *PostgresProjectRepo) CreateProject(id, name, projType, location string, config interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// config to jsonb
	// logic usually handled by pgx if we pass map, or we marshal

	query := `INSERT INTO projects (id, name, type, location, config) VALUES ($1, $2, $3, $4, $5)
			  ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type, location = EXCLUDED.location, config = EXCLUDED.config, deleted_at = NULL`
	_, err := r.pool.Exec(ctx, query, id, name, projType, location, config)
	return err
}

func (r *PostgresProjectRepo) GetProject(id string) (*ProjectRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var p ProjectRecord

	query := `
		SELECT
			id,
			name,
			COALESCE(type, ''),
			COALESCE(location, ''),
			COALESCE(config, '{}'::jsonb)
		FROM projects
		WHERE id = $1 AND deleted_at IS NULL
	`

	// PGX automatically unmarshals JSONB into map[string]interface{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.Name, &p.Type, &p.Location, &p.Config)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostgresProjectRepo) GetAllProjectsWithConfig() ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, "SELECT id, name, config FROM projects WHERE deleted_at IS NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []map[string]interface{}
	for rows.Next() {
		var id, name string
		var config interface{}
		if err := rows.Scan(&id, &name, &config); err != nil {
			continue
		}
		projects = append(projects, map[string]interface{}{
			"id": id, "name": name, "config": config,
		})
	}
	return projects, nil
}

func (r *PostgresProjectRepo) ListProjects() ([]ProjectRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			name,
			COALESCE(type, ''),
			COALESCE(location, ''),
			COALESCE(config, '{}'::jsonb)
		FROM projects
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []ProjectRecord
	for rows.Next() {
		var p ProjectRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Location, &p.Config); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}

	return projects, nil
}

func (r *PostgresProjectRepo) GetProjectWithConfig(id string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		projectID string
		name      string
		config    interface{}
	)

	query := `
		SELECT id, name, COALESCE(config, '{}'::jsonb)
		FROM projects
		WHERE id = $1 AND deleted_at IS NULL
	`
	if err := r.pool.QueryRow(ctx, query, id).Scan(&projectID, &name, &config); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":     projectID,
		"name":   name,
		"config": config,
	}, nil
}

func (r *PostgresProjectRepo) SoftDeleteProject(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.pool.Exec(ctx, "UPDATE projects SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	return err
}
