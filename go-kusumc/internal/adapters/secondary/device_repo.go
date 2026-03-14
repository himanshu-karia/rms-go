package secondary

import (
	"context"
	"encoding/json"
	"time"
)

// Extend existing PostgresRepo
// In a real codebase, this might be in a separate file "device_repo.go"

func (r *PostgresRepo) GetDeviceByIMEI(imei string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fetch device, attributes, and aggregated org links
	query := `
        SELECT 
            d.id, d.imei, d.project_id, d.attributes,
			COALESCE(
				jsonb_object_agg(
					dl.role,
					jsonb_build_object('name', o.name, 'type', o.type)
				) FILTER (WHERE dl.role IS NOT NULL),
				'{}'::jsonb
			) as links
        FROM devices d
        LEFT JOIN device_links dl ON d.id = dl.device_id
        LEFT JOIN organizations o ON dl.org_id = o.id
		WHERE d.imei = $1 AND d.deleted_at IS NULL
        GROUP BY d.id, d.attributes
    `

	var (
		id, pid  string
		attrsRaw []byte
		links    interface{}
	)

	err := r.Pool.QueryRow(ctx, query, imei).Scan(&id, &imei, &pid, &attrsRaw, &links)
	if err != nil {
		return nil, err
	}

	var attrs map[string]interface{}
	if len(attrsRaw) > 0 {
		_ = json.Unmarshal(attrsRaw, &attrs)
	}

	result := map[string]interface{}{
		"id":         id,
		"imei":       imei,
		"project_id": pid,
		"attributes": attrs,
		"links":      links,
	}

	return result, nil
}
