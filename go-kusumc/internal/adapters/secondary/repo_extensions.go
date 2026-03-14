package secondary

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// -- Append to postgres_repo.go --

func (r *PostgresRepo) CreateDeviceStruct(projectId, name, imei string, mqttBundle map[string]interface{}, extraAttrs map[string]interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Base Attributes
	attrs := make(map[string]interface{})
	if extraAttrs != nil {
		for k, v := range extraAttrs {
			attrs[k] = v
		}
	}
	attrs["name"] = name
	if mqttBundle != nil {
		attrs["mqtt"] = mqttBundle
	}

	attrsJson, err := json.Marshal(attrs)
	if err != nil {
		return "", err
	}

	// Upsert by IMEI so repeated creations are idempotent and return the existing row id.
	query := `
		INSERT INTO devices (imei, project_id, attributes, last_seen, name, status)
		VALUES ($1, $2, $3, NOW(), $4, COALESCE($5, 'active'))
		ON CONFLICT (imei)
		DO UPDATE SET
			project_id = EXCLUDED.project_id,
			attributes = EXCLUDED.attributes,
			last_seen = NOW(),
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			deleted_at = NULL
		RETURNING id
	`
	var id string
	err = r.Pool.QueryRow(ctx, query, imei, projectId, attrsJson, name, "active").Scan(&id)
	return id, err
}

// GetDeviceByIDOrIMEI tolerates either UUID id or plain IMEI string.
func (r *PostgresRepo) GetDeviceByIDOrIMEI(identifier string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, imei, project_id, name, status, attributes, shadow, last_seen, connectivity_status, connectivity_updated_at
		FROM devices
		WHERE (id::text = $1 OR imei = $1) AND deleted_at IS NULL
		LIMIT 1
	`

	var (
		did, imei, pid string
		name           sql.NullString
		status         sql.NullString
		attrsRaw       []byte
		shadowRaw      []byte
		lastSeen       *time.Time
		connStatus     sql.NullString
		connUpdatedAt  *time.Time
	)

	err := r.Pool.QueryRow(ctx, query, identifier).Scan(&did, &imei, &pid, &name, &status, &attrsRaw, &shadowRaw, &lastSeen, &connStatus, &connUpdatedAt)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id": did, "imei": imei, "project_id": pid,
	}

	if name.Valid {
		result["name"] = name.String
	}
	if status.Valid {
		result["status"] = status.String
	}
	if lastSeen != nil {
		result["last_seen"] = *lastSeen
	}
	if connStatus.Valid {
		result["connectivity_status"] = connStatus.String
	}
	if connUpdatedAt != nil {
		result["connectivity_updated_at"] = *connUpdatedAt
	}

	if len(shadowRaw) > 0 {
		var shadow map[string]interface{}
		_ = json.Unmarshal(shadowRaw, &shadow)
		result["shadow"] = shadow
	}

	if len(attrsRaw) > 0 {
		var attrs map[string]interface{}
		_ = json.Unmarshal(attrsRaw, &attrs)
		result["attributes"] = attrs
		if auth, ok := attrs["mqtt"].(map[string]interface{}); ok {
			result["auth"] = auth
		}
	}

	return result, nil
}

func (r *PostgresRepo) ListCapabilities() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT key, description FROM capabilities ORDER BY key ASC")
}

func (r *PostgresRepo) ListOrgRoles() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT slug, description FROM org_roles ORDER BY slug ASC")
}

func (r *PostgresRepo) ListProjectRoles() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT slug, description FROM project_roles ORDER BY slug ASC")
}

func (r *PostgresRepo) ListLinkRoles() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT slug, description, is_unique_per_device FROM link_roles ORDER BY slug ASC")
}

func (r *PostgresRepo) GetMqttProvisioningSummary() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, "SELECT status, count(*) FROM mqtt_provisioning_jobs GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	total := 0
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
		total += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	triggerRows, err := r.Pool.Query(ctx, "SELECT COALESCE(trigger_kind,'initial') AS trigger_kind, count(*) FROM mqtt_provisioning_jobs GROUP BY COALESCE(trigger_kind,'initial')")
	if err != nil {
		return nil, err
	}
	defer triggerRows.Close()

	triggerCounts := map[string]int{}
	for triggerRows.Next() {
		var trigger string
		var count int
		if err := triggerRows.Scan(&trigger, &count); err != nil {
			return nil, err
		}
		triggerCounts[trigger] = count
	}
	if err := triggerRows.Err(); err != nil {
		return nil, err
	}

	var retryReady int
	if err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM mqtt_provisioning_jobs WHERE status='failed' AND next_attempt_at <= NOW() AND attempt_count > 0").Scan(&retryReady); err != nil {
		return nil, err
	}

	var avgDurationMs float64
	if err := r.Pool.QueryRow(ctx, "SELECT COALESCE(AVG(last_duration_ms),0) FROM mqtt_provisioning_jobs WHERE last_duration_ms IS NOT NULL AND status='completed'").Scan(&avgDurationMs); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":           total,
		"pending":         counts["pending"],
		"processing":      counts["processing"],
		"completed":       counts["completed"],
		"failed":          counts["failed"],
		"retry_ready":     retryReady,
		"avg_duration_ms": avgDurationMs,
		"by_status":       counts,
		"by_trigger":      triggerCounts,
	}, nil
}

func (r *PostgresRepo) GetCredentialHealthSummary() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, "SELECT lifecycle, count(*) FROM credential_history GROUP BY lifecycle")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	total := 0
	for rows.Next() {
		var lifecycle string
		var count int
		if err := rows.Scan(&lifecycle, &count); err != nil {
			return nil, err
		}
		counts[lifecycle] = count
		total += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":    total,
		"pending":  counts["pending"],
		"active":   counts["active"],
		"revoked":  counts["revoked"],
		"expired":  counts["expired"],
		"by_state": counts,
	}, nil
}

func (r *PostgresRepo) GetOrgRole(slug string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var key, desc string
	if err := r.Pool.QueryRow(ctx, "SELECT slug, description FROM org_roles WHERE slug = $1", slug).Scan(&key, &desc); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return map[string]interface{}{"slug": key, "description": desc}, nil
}

func (r *PostgresRepo) GetProjectRole(slug string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var key, desc string
	if err := r.Pool.QueryRow(ctx, "SELECT slug, description FROM project_roles WHERE slug = $1", slug).Scan(&key, &desc); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return map[string]interface{}{"slug": key, "description": desc}, nil
}

func (r *PostgresRepo) GetLinkRole(slug string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var key, desc string
	var unique sql.NullBool
	if err := r.Pool.QueryRow(ctx, "SELECT slug, description, is_unique_per_device FROM link_roles WHERE slug = $1", slug).Scan(&key, &desc, &unique); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	resp := map[string]interface{}{"slug": key, "description": desc}
	if unique.Valid {
		resp["is_unique_per_device"] = unique.Bool
	}
	return resp, nil
}

// ListDevices returns devices with a total count for pagination.
// - projectId: optional filter; empty string means all
// - search: optional text search against imei or name
func (r *PostgresRepo) ListDevices(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	where := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	arg := 1

	if projectId != "" {
		where += fmt.Sprintf(" AND project_id = $%d", arg)
		args = append(args, projectId)
		arg++
	}
	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", arg)
		args = append(args, status)
		arg++
	} else if !includeInactive {
		where += " AND (status IS NULL OR status <> 'inactive')"
	}
	if search != "" {
		where += fmt.Sprintf(" AND (imei ILIKE $%d OR COALESCE(name,'') ILIKE $%d)", arg, arg)
		args = append(args, "%"+search+"%")
		arg++
	}

	countQuery := "SELECT count(*) FROM devices " + where
	var total int
	if err := r.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery := fmt.Sprintf(`
		SELECT id, imei, project_id, name, status, attributes, shadow, last_seen, connectivity_status, connectivity_updated_at
		FROM devices
		%s
		ORDER BY last_seen DESC NULLS LAST, imei ASC
		LIMIT $%d OFFSET $%d
	`, where, arg, arg+1)

	args = append(args, limit, offset)
	rows, err := r.Pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	devices := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			did, imei, pid string
			name           sql.NullString
			status         sql.NullString
			attrsRaw       []byte
			shadowRaw      []byte
			lastSeen       *time.Time
			connStatus     sql.NullString
			connUpdatedAt  *time.Time
		)

		if err := rows.Scan(&did, &imei, &pid, &name, &status, &attrsRaw, &shadowRaw, &lastSeen, &connStatus, &connUpdatedAt); err != nil {
			return nil, 0, err
		}

		row := map[string]interface{}{
			"id":         did,
			"imei":       imei,
			"project_id": pid,
		}
		if name.Valid {
			row["name"] = name.String
		}
		if status.Valid {
			row["status"] = status.String
		}
		if lastSeen != nil {
			row["last_seen"] = *lastSeen
		}
		if connStatus.Valid {
			row["connectivity_status"] = connStatus.String
		}
		if connUpdatedAt != nil {
			row["connectivity_updated_at"] = *connUpdatedAt
		}

		if len(attrsRaw) > 0 {
			var attrs map[string]interface{}
			_ = json.Unmarshal(attrsRaw, &attrs)
			row["attributes"] = attrs
			if auth, ok := attrs["mqtt"].(map[string]interface{}); ok {
				row["auth"] = auth
			}
		}
		if len(shadowRaw) > 0 {
			var shadow map[string]interface{}
			_ = json.Unmarshal(shadowRaw, &shadow)
			row["shadow"] = shadow
		}

		devices = append(devices, row)
	}

	return devices, total, nil
}

// UpdateDeviceByIDOrIMEI performs a shallow merge into attributes and optionally updates name/status/project_id.
func (r *PostgresRepo) UpdateDeviceByIDOrIMEI(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var patchJSON []byte
	if attrsPatch != nil {
		b, err := json.Marshal(attrsPatch)
		if err != nil {
			return nil, err
		}
		patchJSON = b
	}

	query := `
		UPDATE devices
		SET
			project_id = COALESCE($2, project_id),
			name = COALESCE($3, name),
			status = COALESCE($4, status),
			attributes = COALESCE(attributes, '{}'::jsonb) || COALESCE($5::jsonb, '{}'::jsonb)
		WHERE (id::text = $1 OR imei = $1) AND deleted_at IS NULL
		RETURNING id, imei, project_id, name, status, attributes, shadow
	`

	var (
		did, imei, pid string
		nameOut        sql.NullString
		statusOut      sql.NullString
		attrsRaw       []byte
		shadowRaw      []byte
	)

	var pidIn interface{}
	if projectId != nil {
		pidIn = *projectId
	}
	var nameIn interface{}
	if name != nil {
		nameIn = *name
	}
	var statusIn interface{}
	if status != nil {
		statusIn = *status
	}

	err := r.Pool.QueryRow(ctx, query, idOrIMEI, pidIn, nameIn, statusIn, patchJSON).Scan(
		&did, &imei, &pid, &nameOut, &statusOut, &attrsRaw, &shadowRaw,
	)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":         did,
		"imei":       imei,
		"project_id": pid,
	}
	if nameOut.Valid {
		result["name"] = nameOut.String
	}
	if statusOut.Valid {
		result["status"] = statusOut.String
	}
	if len(attrsRaw) > 0 {
		var attrs map[string]interface{}
		_ = json.Unmarshal(attrsRaw, &attrs)
		result["attributes"] = attrs
		if auth, ok := attrs["mqtt"].(map[string]interface{}); ok {
			result["auth"] = auth
		}
	}
	if len(shadowRaw) > 0 {
		var shadow map[string]interface{}
		_ = json.Unmarshal(shadowRaw, &shadow)
		result["shadow"] = shadow
	}

	return result, nil
}

func (r *PostgresRepo) SoftDeleteDevice(idOrIMEI string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Pool.Exec(ctx, "UPDATE devices SET deleted_at = NOW(), status = COALESCE(status, 'deleted') WHERE (id::text = $1 OR imei = $1) AND deleted_at IS NULL", idOrIMEI)
	return err
}

func (r *PostgresRepo) QueryTelemetryByIMEI(imei string, start, end time.Time, limit int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        SELECT t.time, t.data
        FROM telemetry t
        JOIN devices d ON t.device_id = d.id
        WHERE d.imei = $1
        AND t.time BETWEEN $2 AND $3
        ORDER BY t.time DESC
        LIMIT $4
    `

	rows, err := r.Pool.Query(ctx, query, imei, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []map[string]interface{}{}

	for rows.Next() {
		var t time.Time
		var data map[string]interface{} // jsonb
		rows.Scan(&t, &data)

		// Flatten
		data["timestamp"] = t
		results = append(results, data)
	}
	return results, nil
}

func (r *PostgresRepo) CreateMedicalSession(sess map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO medical_sessions (session_id, patient_id, device_id, doctor_id, start_time, status) 
	          VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.Pool.Exec(ctx, query, sess["session_id"], sess["patient_id"], sess["device_id"], sess["doctor_id"], sess["start_time"], sess["status"])
	return err
}

func (r *PostgresRepo) EndMedicalSession(sessionId string, vitals map[string]interface{}, notes string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE medical_sessions
		SET status = 'COMPLETED',
			vitals = COALESCE($2, vitals),
			notes = COALESCE($3, notes),
			end_time = NOW()
		WHERE session_id = $1`
	_, err := r.Pool.Exec(ctx, query, sessionId, vitals, notes)
	return err
}

func (r *PostgresRepo) CreateDeviceConfiguration(deviceID string, config map[string]any) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.Pool.QueryRow(ctx, `
		INSERT INTO device_configurations (device_id, config)
		VALUES ($1, $2)
		RETURNING id, device_id, config, status, created_at
	`, deviceID, config)

	var id, devID, status string
	var createdAt time.Time
	var cfg map[string]any
	if err := row.Scan(&id, &devID, &cfg, &status, &createdAt); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":         id,
		"device_id":  devID,
		"config":     cfg,
		"status":     status,
		"created_at": createdAt,
	}, nil
}

func (r *PostgresRepo) GetPendingDeviceConfiguration(deviceID string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.Pool.QueryRow(ctx, `
		SELECT id, device_id, config, status, created_at, acknowledged_at, ack_payload
		FROM device_configurations
		WHERE device_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
		LIMIT 1
	`, deviceID)

	var id, devID, status string
	var createdAt time.Time
	var acknowledgedAt *time.Time
	var cfg, ack map[string]any
	if err := row.Scan(&id, &devID, &cfg, &status, &createdAt, &acknowledgedAt, &ack); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return map[string]any{
		"id":              id,
		"device_id":       devID,
		"config":          cfg,
		"status":          status,
		"created_at":      createdAt,
		"acknowledged_at": acknowledgedAt,
		"ack_payload":     ack,
	}, nil
}

func (r *PostgresRepo) AcknowledgeDeviceConfiguration(configID string, ack map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, `
		UPDATE device_configurations
		SET status = 'acknowledged',
			ack_payload = COALESCE($2, ack_payload),
			acknowledged_at = NOW()
		WHERE id = $1
	`, configID, ack)
	return err
}

func (r *PostgresRepo) FinalizeDeviceConfiguration(configID string, status string, ack map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	st := strings.ToLower(strings.TrimSpace(status))
	if st == "" {
		st = "acknowledged"
	}
	_, err := r.Pool.Exec(ctx, `
		UPDATE device_configurations
		SET status = $2,
			ack_payload = COALESCE($3, ack_payload),
			acknowledged_at = NOW()
		WHERE id = $1
	`, configID, st, ack)
	return err
}

func (r *PostgresRepo) GetDeviceConfigurationByID(configID string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.Pool.QueryRow(ctx, `
		SELECT id, device_id, config, status, created_at, acknowledged_at, ack_payload
		FROM device_configurations
		WHERE id = $1
	`, configID)

	var id, devID, status string
	var createdAt time.Time
	var acknowledgedAt *time.Time
	var cfg, ack map[string]any
	if err := row.Scan(&id, &devID, &cfg, &status, &createdAt, &acknowledgedAt, &ack); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return map[string]any{
		"id":              id,
		"device_id":       devID,
		"config":          cfg,
		"status":          status,
		"created_at":      createdAt,
		"acknowledged_at": acknowledgedAt,
		"ack_payload":     ack,
	}, nil
}

func (r *PostgresRepo) GetCoreCommandIDByName(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("command name required")
	}

	var id string
	err := r.Pool.QueryRow(ctx, `
		SELECT id::text
		FROM command_catalog
		WHERE scope = 'core' AND project_id IS NULL AND name = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, trimmed).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

func (r *PostgresRepo) CreateImportJob(jobType, projectID string, total, success, errorCount int, errors []map[string]any) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload, err := json.Marshal(errors)
	if err != nil {
		return nil, err
	}

	status := "completed"
	if errorCount > 0 {
		status = "completed_with_errors"
	}

	row := r.Pool.QueryRow(ctx, `
		INSERT INTO import_jobs (job_type, project_id, status, total_count, success_count, error_count, errors)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, job_type, project_id, status, total_count, success_count, error_count, errors, created_at, updated_at
	`, jobType, projectID, status, total, success, errorCount, payload)

	var id, jt, pid, st string
	var totalCount, successCount, errorCountOut int
	var createdAt, updatedAt time.Time
	var errorsBytes []byte
	if err := row.Scan(&id, &jt, &pid, &st, &totalCount, &successCount, &errorCountOut, &errorsBytes, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var errs []map[string]any
	if len(errorsBytes) > 0 {
		_ = json.Unmarshal(errorsBytes, &errs)
	}
	return map[string]any{
		"id":            id,
		"job_type":      jt,
		"project_id":    pid,
		"status":        st,
		"total_count":   totalCount,
		"success_count": successCount,
		"error_count":   errorCountOut,
		"errors":        errs,
		"created_at":    createdAt,
		"updated_at":    updatedAt,
	}, nil
}

func (r *PostgresRepo) ListImportJobs(jobType string) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, job_type, project_id, status, total_count, success_count, error_count, errors, created_at, updated_at FROM import_jobs`
	args := []interface{}{}
	if jobType != "" {
		query += " WHERE job_type = $1"
		args = append(args, jobType)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, jt, pid, st string
		var totalCount, successCount, errorCountOut int
		var createdAt, updatedAt time.Time
		var errorsBytes []byte
		if err := rows.Scan(&id, &jt, &pid, &st, &totalCount, &successCount, &errorCountOut, &errorsBytes, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		var errs []map[string]any
		if len(errorsBytes) > 0 {
			_ = json.Unmarshal(errorsBytes, &errs)
		}
		out = append(out, map[string]any{
			"id":            id,
			"job_type":      jt,
			"project_id":    pid,
			"status":        st,
			"total_count":   totalCount,
			"success_count": successCount,
			"error_count":   errorCountOut,
			"errors":        errs,
			"created_at":    createdAt,
			"updated_at":    updatedAt,
		})
	}
	return out, nil
}

func (r *PostgresRepo) GetImportJob(jobID string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.Pool.QueryRow(ctx, `
		SELECT id, job_type, project_id, status, total_count, success_count, error_count, errors, created_at, updated_at
		FROM import_jobs
		WHERE id = $1
	`, jobID)

	var id, jt, pid, st string
	var totalCount, successCount, errorCountOut int
	var createdAt, updatedAt time.Time
	var errorsBytes []byte
	if err := row.Scan(&id, &jt, &pid, &st, &totalCount, &successCount, &errorCountOut, &errorsBytes, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var errs []map[string]any
	if len(errorsBytes) > 0 {
		_ = json.Unmarshal(errorsBytes, &errs)
	}
	return map[string]any{
		"id":            id,
		"job_type":      jt,
		"project_id":    pid,
		"status":        st,
		"total_count":   totalCount,
		"success_count": successCount,
		"error_count":   errorCountOut,
		"errors":        errs,
		"created_at":    createdAt,
		"updated_at":    updatedAt,
	}, nil
}

func (r *PostgresRepo) UpdateImportJobStatus(jobID, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, `UPDATE import_jobs SET status = $2, updated_at = NOW() WHERE id = $1`, jobID, status)
	return err
}
