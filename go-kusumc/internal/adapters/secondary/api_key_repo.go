package secondary

import (
	"context"
	"encoding/json"
	"time"
)

type ApiKeyRecord struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	IsActive   bool       `json:"is_active"`
	ProjectID  *string    `json:"project_id,omitempty"`
	OrgID      *string    `json:"org_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (r *PostgresRepo) ListApiKeys() ([]ApiKeyRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, `
		SELECT id, name, prefix, scopes, last_used_at, is_active, project_id, org_id, created_at
		FROM api_keys
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []ApiKeyRecord
	for rows.Next() {
		var rec ApiKeyRecord
		var scopesBytes []byte
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Prefix, &scopesBytes, &rec.LastUsedAt, &rec.IsActive, &rec.ProjectID, &rec.OrgID, &rec.CreatedAt); err != nil {
			return nil, err
		}
		if len(scopesBytes) > 0 {
			_ = json.Unmarshal(scopesBytes, &rec.Scopes)
		}
		keys = append(keys, rec)
	}
	return keys, nil
}

func (r *PostgresRepo) CreateApiKey(name, prefix, hash string, scopes []string, projectID, orgID, createdBy *string) (*ApiKeyRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scopesBytes, err := json.Marshal(scopes)
	if err != nil {
		return nil, err
	}

	var rec ApiKeyRecord
	var scopesOut []byte
	row := r.Pool.QueryRow(ctx, `
		INSERT INTO api_keys (name, key_hash, prefix, project_id, org_id, scopes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, prefix, scopes, last_used_at, is_active, project_id, org_id, created_at
	`, name, hash, prefix, projectID, orgID, scopesBytes, createdBy)
	if err := row.Scan(&rec.ID, &rec.Name, &rec.Prefix, &scopesOut, &rec.LastUsedAt, &rec.IsActive, &rec.ProjectID, &rec.OrgID, &rec.CreatedAt); err != nil {
		return nil, err
	}
	if len(scopesOut) > 0 {
		_ = json.Unmarshal(scopesOut, &rec.Scopes)
	}
	return &rec, nil
}

func (r *PostgresRepo) RevokeApiKey(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Pool.Exec(ctx, `
		UPDATE api_keys
		SET is_active = false,
			deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}
