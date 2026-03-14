package secondary

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func normalizeMasterDataCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	code = strings.ToLower(code)
	code = strings.ReplaceAll(code, " ", "-")
	return code
}

func (r *PostgresRepo) ListMasterData(mdType string, projectId string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, type, name, code, project_id
		FROM master_data
		WHERE type = $1
		AND is_active = true
		AND ($2 = '' OR project_id = $2)
		ORDER BY name ASC
	`

	rows, err := r.Pool.Query(ctx, query, mdType, projectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			id, t, name, code string
			pid               sql.NullString
		)
		if err := rows.Scan(&id, &t, &name, &code, &pid); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"_id":   id,
			"id":    id,
			"type":  t,
			"name":  name,
			"code":  code,
			"value": code,
		}
		if pid.Valid {
			row["projectId"] = pid.String
		}
		out = append(out, row)
	}
	return out, nil
}

func (r *PostgresRepo) CreateMasterData(mdType string, name string, code string, projectId string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	code = normalizeMasterDataCode(code)
	if code == "" {
		code = normalizeMasterDataCode(name)
	}

	query := `
		INSERT INTO master_data (type, name, code, project_id, is_active)
		VALUES ($1, $2, $3, NULLIF($4, ''), true)
		ON CONFLICT (type, code, project_id)
		DO UPDATE SET name = EXCLUDED.name, is_active = true
		RETURNING id, type, name, code, project_id
	`

	var (
		id, t, n, c string
		pid         sql.NullString
	)
	if err := r.Pool.QueryRow(ctx, query, mdType, name, code, projectId).Scan(&id, &t, &n, &c, &pid); err != nil {
		return nil, err
	}

	row := map[string]interface{}{
		"_id":   id,
		"id":    id,
		"type":  t,
		"name":  n,
		"code":  c,
		"value": c,
	}
	if pid.Valid {
		row["projectId"] = pid.String
	}
	return row, nil
}

func (r *PostgresRepo) UpdateMasterData(mdType string, id string, name string, code string, projectId string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	code = normalizeMasterDataCode(code)
	if code == "" {
		code = normalizeMasterDataCode(name)
	}

	query := `
		UPDATE master_data
		SET name = $1,
			code = $2,
			project_id = NULLIF($3, ''),
			is_active = true
		WHERE id::text = $4 AND type = $5
		RETURNING id, type, name, code, project_id
	`

	var (
		outId, t, n, c string
		pid            sql.NullString
	)
	if err := r.Pool.QueryRow(ctx, query, name, code, projectId, id, mdType).Scan(&outId, &t, &n, &c, &pid); err != nil {
		return nil, err
	}

	row := map[string]interface{}{
		"_id":   outId,
		"id":    outId,
		"type":  t,
		"name":  n,
		"code":  c,
		"value": c,
	}
	if pid.Valid {
		row["projectId"] = pid.String
	}
	return row, nil
}

func (r *PostgresRepo) DeleteMasterData(mdType string, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Soft-delete: keep unique history, hide from list.
	_, err := r.Pool.Exec(ctx, `UPDATE master_data SET is_active=false WHERE id::text=$1 AND type=$2`, id, mdType)
	return err
}
