package secondary

import (
	"context"
	"fmt"
	"time"
)

// ExportTelemetryByIMEI returns raw telemetry rows for a single device (by IMEI) in a time range.
func (r *PostgresRepo) ExportTelemetryByIMEI(start, end time.Time, imei string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	query := `
		SELECT t.time, t.device_id, t.data
		FROM telemetry t
		JOIN devices d ON t.device_id = d.id
		WHERE (d.imei = $1 OR d.id::text = $1)
		AND t.time >= $2 AND t.time <= $3
		ORDER BY t.time ASC
	`

	rows, err := r.Pool.Query(ctx, query, imei, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var t time.Time
		var deviceID string
		var data map[string]interface{}
		if err := rows.Scan(&t, &deviceID, &data); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"time":      t,
			"device_id": deviceID,
			"data":      data,
		})
	}

	return out, nil
}

// ExportTelemetryByIMEIFiltered returns raw telemetry rows for a single device (by IMEI) in a time range with optional JSON filters.
// Filters:
// - packetType: matches data->>'packet_type'
// - quality: matches data->'meta'->>'quality'
// - excludeQuality: excludes rows where data->'meta'->>'quality' equals this value
func (r *PostgresRepo) ExportTelemetryByIMEIFiltered(start, end time.Time, imei, packetType, quality, excludeQuality string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	base := `
		SELECT t.time, t.device_id, t.data
		FROM telemetry t
		JOIN devices d ON t.device_id = d.id
		WHERE (d.imei = $1 OR d.id::text = $1)
		AND t.time >= $2 AND t.time <= $3
	`
	args := []interface{}{imei, start, end}
	idx := 4
	if packetType != "" {
		base += fmt.Sprintf(" AND t.data->>'packet_type' = $%d\n", idx)
		args = append(args, packetType)
		idx++
	}
	if quality != "" {
		base += fmt.Sprintf(" AND t.data->'meta'->>'quality' = $%d\n", idx)
		args = append(args, quality)
		idx++
	}
	if excludeQuality != "" {
		base += fmt.Sprintf(" AND COALESCE(t.data->'meta'->>'quality','') <> $%d\n", idx)
		args = append(args, excludeQuality)
		idx++
	}
	base += " ORDER BY t.time ASC"

	rows, err := r.Pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var t time.Time
		var deviceID string
		var data map[string]interface{}
		if err := rows.Scan(&t, &deviceID, &data); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"time":      t,
			"device_id": deviceID,
			"data":      data,
		})
	}

	return out, nil
}
