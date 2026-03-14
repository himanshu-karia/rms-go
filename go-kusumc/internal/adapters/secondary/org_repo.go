package secondary

import (
	"context"
	"encoding/json"
	"time"
)

type OrgRecord struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Path     *string                `json:"path,omitempty"`
	ParentID *string                `json:"parent_id,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (r *PostgresRepo) GetOrgByID(id string) (*OrgRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.Pool.QueryRow(ctx, `
		SELECT id, name, type, path::text, parent_id, metadata
		FROM organizations
		WHERE id = $1 AND deleted_at IS NULL
	`, id)

	var org OrgRecord
	var path *string
	var parentID *string
	var metadataBytes []byte
	if err := row.Scan(&org.ID, &org.Name, &org.Type, &path, &parentID, &metadataBytes); err != nil {
		return nil, err
	}
	org.Path = path
	org.ParentID = parentID
	if len(metadataBytes) > 0 {
		_ = json.Unmarshal(metadataBytes, &org.Metadata)
	}
	return &org, nil
}

func (r *PostgresRepo) ListOrgs() ([]OrgRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, `
		SELECT id, name, type, path::text, parent_id, metadata
		FROM organizations
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []OrgRecord
	for rows.Next() {
		var org OrgRecord
		var path *string
		var parentID *string
		var metadataBytes []byte
		if err := rows.Scan(&org.ID, &org.Name, &org.Type, &path, &parentID, &metadataBytes); err != nil {
			return nil, err
		}
		org.Path = path
		org.ParentID = parentID
		if len(metadataBytes) > 0 {
			_ = json.Unmarshal(metadataBytes, &org.Metadata)
		}
		orgs = append(orgs, org)
	}
	return orgs, nil
}

func (r *PostgresRepo) ListVendors(category string) ([]OrgRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, name, type, path::text, parent_id, metadata
		FROM organizations
		WHERE deleted_at IS NULL AND type = 'vendor'
	`
	args := []interface{}{}
	if category != "" {
		query += " AND (metadata->>'category' = $1 OR metadata->>'vendor_category' = $1)"
		args = append(args, category)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []OrgRecord
	for rows.Next() {
		var org OrgRecord
		var path *string
		var parentID *string
		var metadataBytes []byte
		if err := rows.Scan(&org.ID, &org.Name, &org.Type, &path, &parentID, &metadataBytes); err != nil {
			return nil, err
		}
		org.Path = path
		org.ParentID = parentID
		if len(metadataBytes) > 0 {
			_ = json.Unmarshal(metadataBytes, &org.Metadata)
		}
		orgs = append(orgs, org)
	}
	return orgs, nil
}

func (r *PostgresRepo) CreateOrg(name, orgType, path string, parentID *string, metadata map[string]interface{}) (*OrgRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	parentValue := ""
	if parentID != nil {
		parentValue = *parentID
	}

	var org OrgRecord
	var pathValue *string
	var parentValueOut *string
	var metadataOut []byte
	row := r.Pool.QueryRow(ctx, `
		INSERT INTO organizations (name, type, path, parent_id, metadata)
		VALUES ($1, $2, NULLIF($3,'')::ltree, NULLIF($4,'')::uuid, $5)
		RETURNING id, name, type, path::text, parent_id, metadata
	`, name, orgType, path, parentValue, metadataBytes)
	if err := row.Scan(&org.ID, &org.Name, &org.Type, &pathValue, &parentValueOut, &metadataOut); err != nil {
		return nil, err
	}
	org.Path = pathValue
	org.ParentID = parentValueOut
	if len(metadataOut) > 0 {
		_ = json.Unmarshal(metadataOut, &org.Metadata)
	}
	return &org, nil
}

func (r *PostgresRepo) UpdateOrg(id, name, orgType, path string, parentID *string, metadata map[string]interface{}) (*OrgRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	parentValue := ""
	if parentID != nil {
		parentValue = *parentID
	}

	var org OrgRecord
	var pathValue *string
	var parentValueOut *string
	var metadataOut []byte
	row := r.Pool.QueryRow(ctx, `
		UPDATE organizations
		SET name = $1,
			type = $2,
			path = NULLIF($3,'')::ltree,
			parent_id = NULLIF($4,'')::uuid,
			metadata = $5
		WHERE id = $6 AND deleted_at IS NULL
		RETURNING id, name, type, path::text, parent_id, metadata
	`, name, orgType, path, parentValue, metadataBytes, id)
	if err := row.Scan(&org.ID, &org.Name, &org.Type, &pathValue, &parentValueOut, &metadataOut); err != nil {
		return nil, err
	}
	org.Path = pathValue
	org.ParentID = parentValueOut
	if len(metadataOut) > 0 {
		_ = json.Unmarshal(metadataOut, &org.Metadata)
	}
	return &org, nil
}

func (r *PostgresRepo) UpdateVendor(id string, name *string, metadata map[string]interface{}) (*OrgRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var metadataBytes []byte
	if metadata != nil {
		var err error
		metadataBytes, err = json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
	}

	row := r.Pool.QueryRow(ctx, `
		UPDATE organizations
		SET name = COALESCE($2, name),
			metadata = COALESCE($3, metadata)
		WHERE id = $1 AND deleted_at IS NULL AND type = 'vendor'
		RETURNING id, name, type, path::text, parent_id, metadata
	`, id, name, metadataBytes)

	var org OrgRecord
	var path *string
	var parentID *string
	var metadataOut []byte
	if err := row.Scan(&org.ID, &org.Name, &org.Type, &path, &parentID, &metadataOut); err != nil {
		return nil, err
	}
	org.Path = path
	org.ParentID = parentID
	if len(metadataOut) > 0 {
		_ = json.Unmarshal(metadataOut, &org.Metadata)
	}
	return &org, nil
}

func (r *PostgresRepo) SoftDeleteOrg(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, "UPDATE organizations SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	return err
}
