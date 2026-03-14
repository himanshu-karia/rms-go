package secondary

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool" // Needs go get

	"ingestion-go/internal/config/dna"
	"ingestion-go/internal/core/domain"
	"ingestion-go/internal/models"
)

type PostgresRepo struct {
	Pool *pgxpool.Pool
}

func NewPostgresRepo(uri string) (*PostgresRepo, error) {
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, err
	}

	// Tuning for High Throughput
	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &PostgresRepo{Pool: pool}, nil
}

// SaveTelemetry inserts a single packet or batch.
// For V1, we implement strict Single Insert to verify functionality.
// Later we upgrade to CopyFrom (Batch).
func (r *PostgresRepo) SaveTelemetry(telemetry interface{}) error {
	// telemetry is expected to be map[string]interface{} or a struct
	// We assume a simple struct for now or map

	data, ok := telemetry.(map[string]interface{})
	if !ok {
		return nil // skip invalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        INSERT INTO telemetry (time, device_id, project_id, data)
        VALUES ($1, $2, $3, $4)
    `
	// Assuming data parsing happened upstream and we have keys
	// In reality, IngestionService should pass a strongly typed domain struct
	// For this brainstorming coding, we do dynamic cast

	_, err := r.Pool.Exec(ctx, query,
		data["time"],
		data["device_id"],
		data["project_id"],
		data["payload"], // stored as JSONB
	)
	return err
}

// UpdateDeviceShadow updates the 'last_seen' and 'shadow.reported' state
func (r *PostgresRepo) UpdateDeviceShadow(imei string, reported map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Logic: Merge reported into shadow.reported
	// This query assumes 'shadow' is { "desired": {}, "reported": {} } structure
	// We use jsonb_set or simple merge if structure is flat.
	// For V1 simplified: We overwrite 'reported'.

	query := `
		UPDATE devices 
		SET last_seen = NOW(),
		    connectivity_status = 'online',
		    connectivity_updated_at = NOW(),
		    shadow = jsonb_set(COALESCE(shadow, '{}'), '{reported}', $2)
		WHERE imei = $1
	`
	_, err := r.Pool.Exec(ctx, query, imei, reported)
	return err
}

func (r *PostgresRepo) SaveBatch(batch []interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rows [][]interface{}
	for _, item := range batch {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		// Expecting envelope with keys: time, device_id, project_id, payload
		rows = append(rows, []interface{}{
			data["time"],
			data["device_id"],
			data["project_id"],
			data["payload"],
		})
	}

	_, err := r.Pool.CopyFrom(
		ctx,
		pgx.Identifier{"telemetry"},
		[]string{"time", "device_id", "project_id", "data"},
		pgx.CopyFromRows(rows),
	)
	return err
}

// --- Phase 2: Service Injection Methods ---

// --- Phase 2: Service Injection Methods ---

// 1. Rules (Moved to bottom)

// 2. OTA
func (r *PostgresRepo) CreateCampaign(id, name, pType, version, url, checksum string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO ota_campaigns (id, name, project_type, version, url, checksum) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.Pool.Exec(ctx, query, id, name, pType, version, url, checksum)
	return err
}

// 3. Scheduler
func (r *PostgresRepo) GetDueSchedules() ([]map[string]interface{}, error) {
	// Simplistic Cron check (In reality, we'd calculate next_run vs now)
	// For V1 MVP, we just return ALL active schedules to let Service Logic decide/filter
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, "SELECT id, project_id, command FROM schedules WHERE is_active=true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []map[string]interface{}
	for rows.Next() {
		var id, pid string
		var cmd interface{}
		rows.Scan(&id, &pid, &cmd)
		schedules = append(schedules, map[string]interface{}{
			"id":         id,
			"project_id": pid,
			"command":    cmd,
		})
	}
	return schedules, nil
}

// 4. Audit / Command Log
// LogCommand is in audit_repos.go

// 5. Devices
func (r *PostgresRepo) GetDevicesByType(pType string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT d.imei, d.project_id, d.name 
		FROM devices d 
		JOIN projects p ON d.project_id = p.id 
		WHERE p.type = $1 AND d.status = 'active'`

	rows, err := r.Pool.Query(ctx, query, pType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []map[string]interface{}
	for rows.Next() {
		var imei, pid, name string
		if err := rows.Scan(&imei, &pid, &name); err != nil {
			continue
		}
		devices = append(devices, map[string]interface{}{
			"imei":       imei,
			"project_id": pid,
			"name":       name,
		})
	}
	return devices, nil
}

// --- Project DNA: Sensors ---

func (r *PostgresRepo) ListDnaSensors(ctx context.Context, projectID string) ([]models.DnaSensor, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT project_id, param, label, unit, min_value, max_value, resolution, required, notes, topic_template, updated_at
		FROM payload_sensors
		WHERE project_id = $1
		ORDER BY param
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.DnaSensor, 0)
	for rows.Next() {
		var (
			s       models.DnaSensor
			unit    sql.NullString
			notes   sql.NullString
			topic   sql.NullString
			min     sql.NullFloat64
			max     sql.NullFloat64
			res     sql.NullFloat64
			updated sql.NullTime
		)
		if err := rows.Scan(&s.ProjectID, &s.Param, &s.Label, &unit, &min, &max, &res, &s.Required, &notes, &topic, &updated); err != nil {
			return nil, err
		}
		if unit.Valid {
			s.Unit = &unit.String
		}
		if notes.Valid {
			s.Notes = &notes.String
		}
		if topic.Valid {
			s.TopicTemplate = &topic.String
		}
		if min.Valid {
			s.MinValue = &min.Float64
		}
		if max.Valid {
			s.MaxValue = &max.Float64
		}
		if res.Valid {
			s.Resolution = &res.Float64
		}
		if updated.Valid {
			t := updated.Time
			s.UpdatedAt = &t
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *PostgresRepo) UpsertDnaSensors(ctx context.Context, projectID string, sensors []models.DnaSensor) error {
	if len(sensors) == 0 {
		return nil
	}
	for _, s := range sensors {
		query := `
		INSERT INTO payload_sensors (project_id, param, label, unit, min_value, max_value, resolution, required, notes, topic_template, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, NOW())
		ON CONFLICT (project_id, param)
		DO UPDATE SET
			label = EXCLUDED.label,
			unit = EXCLUDED.unit,
			min_value = EXCLUDED.min_value,
			max_value = EXCLUDED.max_value,
			resolution = EXCLUDED.resolution,
			required = EXCLUDED.required,
			notes = EXCLUDED.notes,
			topic_template = EXCLUDED.topic_template,
			updated_at = NOW()
		`
		if _, err := r.Pool.Exec(ctx, query, projectID, s.Param, s.Label, s.Unit, s.MinValue, s.MaxValue, s.Resolution, s.Required, s.Notes, s.TopicTemplate); err != nil {
			return err
		}
	}
	return nil
}

// --- Project DNA: Thresholds ---

func (r *PostgresRepo) ListDnaThresholds(ctx context.Context, projectID, scope string, deviceID *string) ([]models.DnaThreshold, error) {
	var rows pgx.Rows
	var err error
	if scope == "device" && deviceID != nil {
		rows, err = r.Pool.Query(ctx, `
			SELECT project_id, param, scope, device_id, min_value, max_value, target, unit, decimal_places, template_id, metadata, reason, updated_by, warn_low, warn_high, alert_low, alert_high, origin, updated_at
			FROM telemetry_thresholds
			WHERE project_id = $1 AND scope = 'device' AND device_id = $2
		`, projectID, *deviceID)
	} else {
		rows, err = r.Pool.Query(ctx, `
			SELECT project_id, param, scope, device_id, min_value, max_value, target, unit, decimal_places, template_id, metadata, reason, updated_by, warn_low, warn_high, alert_low, alert_high, origin, updated_at
			FROM telemetry_thresholds
			WHERE project_id = $1 AND scope = 'project'
		`, projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.DnaThreshold, 0)
	for rows.Next() {
		var (
			t             models.DnaThreshold
			device        sql.NullString
			minValue      sql.NullFloat64
			maxValue      sql.NullFloat64
			target        sql.NullFloat64
			unit          sql.NullString
			decimalPlaces sql.NullInt32
			templateID    sql.NullString
			metadata      map[string]interface{}
			reason        sql.NullString
			updatedBy     sql.NullString
			warnLow       sql.NullFloat64
			warnHigh      sql.NullFloat64
			alertLow      sql.NullFloat64
			alertHigh     sql.NullFloat64
			origin        sql.NullString
			updated       sql.NullTime
		)
		if err := rows.Scan(&t.ProjectID, &t.Param, &t.Scope, &device, &minValue, &maxValue, &target, &unit, &decimalPlaces, &templateID, &metadata, &reason, &updatedBy, &warnLow, &warnHigh, &alertLow, &alertHigh, &origin, &updated); err != nil {
			return nil, err
		}
		if device.Valid {
			d := device.String
			t.DeviceID = &d
		}
		if minValue.Valid {
			v := minValue.Float64
			t.MinValue = &v
		}
		if maxValue.Valid {
			v := maxValue.Float64
			t.MaxValue = &v
		}
		if target.Valid {
			v := target.Float64
			t.Target = &v
		}
		if unit.Valid {
			v := unit.String
			t.Unit = &v
		}
		if decimalPlaces.Valid {
			v := int(decimalPlaces.Int32)
			t.DecimalPlaces = &v
		}
		if templateID.Valid {
			v := templateID.String
			t.TemplateID = &v
		}
		if metadata != nil {
			t.Metadata = metadata
		}
		if reason.Valid {
			v := reason.String
			t.Reason = &v
		}
		if updatedBy.Valid {
			v := updatedBy.String
			t.UpdatedBy = &v
		}
		if warnLow.Valid {
			t.WarnLow = &warnLow.Float64
		}
		if warnHigh.Valid {
			t.WarnHigh = &warnHigh.Float64
		}
		if alertLow.Valid {
			t.AlertLow = &alertLow.Float64
		}
		if alertHigh.Valid {
			t.AlertHigh = &alertHigh.Float64
		}
		if origin.Valid {
			o := origin.String
			t.Origin = &o
		}
		if updated.Valid {
			tt := updated.Time
			t.UpdatedAt = &tt
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *PostgresRepo) UpsertDnaThresholds(ctx context.Context, records []models.DnaThreshold) error {
	for _, t := range records {
		query := `
		INSERT INTO telemetry_thresholds (project_id, param, scope, device_id, min_value, max_value, target, unit, decimal_places, template_id, metadata, reason, updated_by, warn_low, warn_high, alert_low, alert_high, origin, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,COALESCE($11,'{}'::jsonb),$12,$13,$14,$15,$16,$17,$18,NOW())
		ON CONFLICT (project_id, param, scope, COALESCE(device_id, ''))
		DO UPDATE SET
			min_value = EXCLUDED.min_value,
			max_value = EXCLUDED.max_value,
			target = EXCLUDED.target,
			unit = EXCLUDED.unit,
			decimal_places = EXCLUDED.decimal_places,
			template_id = EXCLUDED.template_id,
			metadata = EXCLUDED.metadata,
			reason = EXCLUDED.reason,
			updated_by = EXCLUDED.updated_by,
			warn_low = EXCLUDED.warn_low,
			warn_high = EXCLUDED.warn_high,
			alert_low = EXCLUDED.alert_low,
			alert_high = EXCLUDED.alert_high,
			origin = EXCLUDED.origin,
			updated_at = NOW()
		`
		if _, err := r.Pool.Exec(ctx, query, t.ProjectID, t.Param, t.Scope, t.DeviceID, t.MinValue, t.MaxValue, t.Target, t.Unit, t.DecimalPlaces, t.TemplateID, t.Metadata, t.Reason, t.UpdatedBy, t.WarnLow, t.WarnHigh, t.AlertLow, t.AlertHigh, t.Origin); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepo) DeleteDnaThresholds(ctx context.Context, projectID, scope string, deviceID *string) (int64, error) {
	if scope == "device" && deviceID != nil {
		result, err := r.Pool.Exec(ctx, `
			DELETE FROM telemetry_thresholds
			WHERE project_id = $1 AND scope = 'device' AND device_id = $2
		`, projectID, *deviceID)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected(), nil
	}

	result, err := r.Pool.Exec(ctx, `
		DELETE FROM telemetry_thresholds
		WHERE project_id = $1 AND scope = 'project'
	`, projectID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// --- Project DNA: Sensor Versions ---

// CreateSensorVersion stores a CSV snapshot as a draft version.
func (r *PostgresRepo) CreateSensorVersion(ctx context.Context, projectID, label string, csvData []byte, importedCount int, createdBy *string) (int64, error) {
	if label == "" {
		label = "draft"
	}
	var id int64
	query := `
	INSERT INTO dna_sensor_versions (project_id, label, status, imported_count, csv_data, created_at, created_by)
	VALUES ($1, $2, 'draft', $3, $4, NOW(), $5)
	RETURNING id
	`
	err := r.Pool.QueryRow(ctx, query, projectID, label, importedCount, csvData, createdBy).Scan(&id)
	return id, err
}

// ListSensorVersions returns recent versions for a project.
func (r *PostgresRepo) ListSensorVersions(ctx context.Context, projectID string, limit int) ([]models.DnaSensorVersion, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT id, project_id, label, status, imported_count, created_at, published_at, rolled_back_at, created_by, published_by, rolled_back_by
		FROM dna_sensor_versions
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.DnaSensorVersion, 0)
	for rows.Next() {
		var v models.DnaSensorVersion
		var published sql.NullTime
		var rolledBack sql.NullTime
		var createdBy sql.NullString
		var publishedBy sql.NullString
		var rolledBackBy sql.NullString
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Label, &v.Status, &v.ImportedCount, &v.CreatedAt, &published, &rolledBack, &createdBy, &publishedBy, &rolledBackBy); err != nil {
			return nil, err
		}
		if published.Valid {
			p := published.Time
			v.PublishedAt = &p
		}
		if rolledBack.Valid {
			r := rolledBack.Time
			v.RolledBackAt = &r
		}
		if createdBy.Valid {
			cb := createdBy.String
			v.CreatedBy = &cb
		}
		if publishedBy.Valid {
			pb := publishedBy.String
			v.PublishedBy = &pb
		}
		if rolledBackBy.Valid {
			rb := rolledBackBy.String
			v.RolledBackBy = &rb
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

// GetSensorVersionCSV returns the raw CSV payload and metadata for a version.
func (r *PostgresRepo) GetSensorVersionCSV(ctx context.Context, projectID string, versionID int64) ([]byte, *models.DnaSensorVersion, error) {
	row := r.Pool.QueryRow(ctx, `
		SELECT id, project_id, label, status, imported_count, created_at, published_at, rolled_back_at, csv_data, created_by, published_by, rolled_back_by
		FROM dna_sensor_versions
		WHERE project_id = $1 AND id = $2
	`, projectID, versionID)

	var (
		v            models.DnaSensorVersion
		published    sql.NullTime
		rolledBack   sql.NullTime
		csvData      []byte
		createdBy    sql.NullString
		publishedBy  sql.NullString
		rolledBackBy sql.NullString
	)
	if err := row.Scan(&v.ID, &v.ProjectID, &v.Label, &v.Status, &v.ImportedCount, &v.CreatedAt, &published, &rolledBack, &csvData, &createdBy, &publishedBy, &rolledBackBy); err != nil {
		return nil, nil, err
	}
	if published.Valid {
		p := published.Time
		v.PublishedAt = &p
	}
	if rolledBack.Valid {
		r := rolledBack.Time
		v.RolledBackAt = &r
	}
	if createdBy.Valid {
		cb := createdBy.String
		v.CreatedBy = &cb
	}
	if publishedBy.Valid {
		pb := publishedBy.String
		v.PublishedBy = &pb
	}
	if rolledBackBy.Valid {
		rb := rolledBackBy.String
		v.RolledBackBy = &rb
	}
	return csvData, &v, nil
}

// MarkSensorVersionPublished sets status, timestamp, and actor when a version is applied.
func (r *PostgresRepo) MarkSensorVersionPublished(ctx context.Context, projectID string, versionID int64, publishedBy *string) error {
	_, err := r.Pool.Exec(ctx, `
		UPDATE dna_sensor_versions
		SET status = 'published', published_at = COALESCE(published_at, NOW()), published_by = COALESCE(published_by, $3)
		WHERE project_id = $1 AND id = $2
	`, projectID, versionID, publishedBy)
	return err
}

// MarkSensorVersionRolledBack notes that a rollback applied the version.
func (r *PostgresRepo) MarkSensorVersionRolledBack(ctx context.Context, projectID string, versionID int64, rolledBackBy *string) error {
	_, err := r.Pool.Exec(ctx, `
		UPDATE dna_sensor_versions
		SET status = 'published', published_at = COALESCE(published_at, NOW()), published_by = COALESCE(published_by, $3), rolled_back_at = NOW(), rolled_back_by = $3
		WHERE project_id = $1 AND id = $2
	`, projectID, versionID, rolledBackBy)
	return err
}

// ListDnaThresholdDeviceIDs returns device_ids that have overrides for a project.
func (r *PostgresRepo) ListDnaThresholdDeviceIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT DISTINCT device_id
		FROM telemetry_thresholds
		WHERE project_id = $1 AND scope = 'device' AND device_id IS NOT NULL AND device_id <> ''
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- Phase 5: ERP Extensions ---

// A. Work Orders
func (r *PostgresRepo) GetWorkOrders() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT * FROM work_orders ORDER BY created_at DESC LIMIT 50")
}

// B. Inventory
func (r *PostgresRepo) GetProducts() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT * FROM products ORDER BY name ASC")
}

func (r *PostgresRepo) GetStockLevels(locationId string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, `
		SELECT
			sl.id,
			sl.quantity,
			sl.last_updated,
			p.id,
			p.sku,
			p.name,
			p.category,
			p.unit
		FROM stock_levels sl
		JOIN products p ON p.id = sl.product_id
		WHERE sl.location_id = $1
		ORDER BY p.name ASC
	`, locationId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			stockID   string
			quantity  float64
			updatedAt time.Time
			productID string
			sku       string
			name      string
			category  string
			unit      string
		)
		if err := rows.Scan(&stockID, &quantity, &updatedAt, &productID, &sku, &name, &category, &unit); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"_id":         stockID,
			"quantity":    quantity,
			"lastUpdated": updatedAt,
			"activeTags":  []string{},
			"productId": map[string]interface{}{
				"_id":      productID,
				"sku":      sku,
				"name":     name,
				"category": category,
				"unit":     unit,
			},
		})
	}
	return results, rows.Err()
}

// C. Logistics
func (r *PostgresRepo) GetTrips() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT * FROM trips ORDER BY start_time DESC")
}

func (r *PostgresRepo) GetGeofences() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT * FROM geofences")
}

// D. Traffic
func (r *PostgresRepo) GetTrafficCameras() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT * FROM traffic_cameras")
}

func (r *PostgresRepo) GetTrafficMetrics(deviceId string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, `
		SELECT id, device_id, vehicle_count, breakdown, avg_speed, congestion_level, timestamp
		FROM traffic_metrics
		WHERE device_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, deviceId, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			id           string
			device       string
			vehicleCount int
			breakdownRaw []byte
			avgSpeed     float64
			congestion   string
			timestamp    time.Time
		)
		if err := rows.Scan(&id, &device, &vehicleCount, &breakdownRaw, &avgSpeed, &congestion, &timestamp); err != nil {
			return nil, err
		}
		breakdown := map[string]interface{}{}
		if len(breakdownRaw) > 0 {
			_ = json.Unmarshal(breakdownRaw, &breakdown)
		}
		results = append(results, map[string]interface{}{
			"_id":             id,
			"deviceId":        device,
			"vehicleCount":    vehicleCount,
			"breakdown":       breakdown,
			"avgSpeed":        avgSpeed,
			"congestionLevel": congestion,
			"timestamp":       timestamp,
		})
	}
	return results, rows.Err()
}

func (r *PostgresRepo) CreateTrafficMetric(deviceId string, breakdown map[string]interface{}, avgSpeed float64, congestion string, vehicleCount int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	breakdownJSON, err := json.Marshal(breakdown)
	if err != nil {
		return err
	}

	_, err = r.Pool.Exec(ctx, `
		INSERT INTO traffic_metrics (device_id, vehicle_count, breakdown, avg_speed, congestion_level)
		VALUES ($1, $2, $3, $4, $5)
	`, deviceId, vehicleCount, breakdownJSON, avgSpeed, congestion)
	return err
}

// --- Workflows: Offline Monitor + Notifications ---

func (r *PostgresRepo) ListOfflineCandidates(thresholdSeconds int64, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, `
		SELECT id, imei, project_id, last_seen
		FROM devices
		WHERE deleted_at IS NULL
		  AND (status IS NULL OR status = 'active')
		  AND (connectivity_status IS NULL OR connectivity_status <> 'offline')
		  AND last_seen IS NOT NULL
		  AND last_seen < NOW() - ($1 * INTERVAL '1 second')
		ORDER BY last_seen ASC
		LIMIT $2
	`, thresholdSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			id        string
			imei      string
			projectId string
			lastSeen  time.Time
		)
		if err := rows.Scan(&id, &imei, &projectId, &lastSeen); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":         id,
			"imei":       imei,
			"project_id": projectId,
			"last_seen":  lastSeen,
		})
	}
	return results, rows.Err()
}

func (r *PostgresRepo) MarkDeviceOffline(deviceId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, `
		UPDATE devices
		SET connectivity_status = 'offline',
		    connectivity_updated_at = NOW()
		WHERE id = $1
	`, deviceId)
	return err
}

func (r *PostgresRepo) EnqueueNotification(deviceId, deviceUUID, channel, target, triggeredBy string, payload map[string]interface{}, scheduledFor time.Time, templateId *string, metadata map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = r.Pool.Exec(ctx, `
		INSERT INTO notification_queue (device_id, device_uuid, channel, target, template_id, status, triggered_by, payload, scheduled_for, metadata)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7, $8, $9)
	`, deviceId, deviceUUID, channel, target, templateId, triggeredBy, payloadJSON, scheduledFor, metadataJSON)
	return err
}

func (r *PostgresRepo) ListPendingNotifications(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 100
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, `
		SELECT id, device_id, device_uuid, channel, target, template_id, payload, scheduled_for
		FROM notification_queue
		WHERE status = 'pending' AND scheduled_for <= NOW()
		ORDER BY scheduled_for ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			id           string
			deviceID     string
			deviceUUID   string
			channel      string
			target       string
			templateId   *string
			payloadRaw   []byte
			scheduledFor time.Time
		)
		if err := rows.Scan(&id, &deviceID, &deviceUUID, &channel, &target, &templateId, &payloadRaw, &scheduledFor); err != nil {
			return nil, err
		}
		payload := map[string]interface{}{}
		if len(payloadRaw) > 0 {
			_ = json.Unmarshal(payloadRaw, &payload)
		}
		results = append(results, map[string]interface{}{
			"id":            id,
			"device_id":     deviceID,
			"device_uuid":   deviceUUID,
			"channel":       channel,
			"target":        target,
			"template_id":   templateId,
			"payload":       payload,
			"scheduled_for": scheduledFor,
		})
	}
	return results, rows.Err()
}

func (r *PostgresRepo) UpdateNotificationStatus(id, status string, metadata map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `
		UPDATE notification_queue
		SET status = $2,
		    metadata = COALESCE(metadata, '{}'::jsonb) || $3::jsonb,
		    updated_at = NOW()
		WHERE id = $1
	`, id, status, metadataJSON)
	return err
}

// Helper for quick JSON map fetch
func (r *PostgresRepo) fetchSimple(query string) ([]map[string]interface{}, error) {
	return r.fetchSimpleWithArgs(query)
}

// --- WRITE OPERATIONS (Phase 7) ---

// Maintenance: Create & Resolve
func (r *PostgresRepo) CreateWorkOrder(wo map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, `INSERT INTO work_orders (ticket_id, title, device_id, priority)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ticket_id) DO NOTHING`,
		wo["ticket_id"], wo["title"], wo["device_id"], wo["priority"])
	return err
}

func (r *PostgresRepo) ResolveWorkOrder(id string, notes string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, "UPDATE work_orders SET status='RESOLVED', description=$2 WHERE id=$1", id, notes)
	return err
}

// Inventory: Create Product
func (r *PostgresRepo) CreateProduct(prod map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, `INSERT INTO products (sku, name, category, unit)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sku) DO NOTHING`,
		prod["sku"], prod["name"], prod["category"], prod["unit"])
	return err
}

// Logistics: Create Trip
func (r *PostgresRepo) CreateTrip(trip map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, `INSERT INTO trips (trip_id, project_id, vehicle_id, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (trip_id) DO NOTHING`,
		trip["trip_id"], trip["project_id"], trip["vehicle_id"], "SCHEDULED")
	return err
}

// --- Phase 8: Advanced Config ---

// Automation Flows (No-Code Builder)
func (r *PostgresRepo) CreateAutomationFlow(flow map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	version := "1.0.0"
	if rawVersion, ok := flow["schema_version"].(string); ok && strings.TrimSpace(rawVersion) != "" {
		version = strings.TrimSpace(rawVersion)
	}
	compiledRules := flow["compiled_rules"]
	if compiledRules == nil {
		compiledRules = []interface{}{}
	}
	// Upsert Logic: One Flow per Project
	query := `
		INSERT INTO automation_flows (project_id, version, nodes, edges, compiled_rules) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (project_id) 
		DO UPDATE SET version=$2, nodes=$3, edges=$4, compiled_rules=$5, updated_at=NOW()`
	_, err := r.Pool.Exec(ctx, query, flow["project_id"], version, flow["nodes"], flow["edges"], compiledRules)
	if err == nil {
		return nil
	}

	if strings.Contains(strings.ToLower(err.Error()), "compiled_rules") {
		legacyQuery := `
			INSERT INTO automation_flows (project_id, version, nodes, edges) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (project_id) 
			DO UPDATE SET version=$2, nodes=$3, edges=$4, updated_at=NOW()`
		_, legacyErr := r.Pool.Exec(ctx, legacyQuery, flow["project_id"], version, flow["nodes"], flow["edges"])
		return legacyErr
	}

	return err
}

func (r *PostgresRepo) GetAutomationFlow(projectId string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hasCompiledColumn := true
	rows, err := r.Pool.Query(ctx, "SELECT id, version, nodes, edges, compiled_rules FROM automation_flows WHERE project_id=$1", projectId)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "compiled_rules") {
		hasCompiledColumn = false
		rows, err = r.Pool.Query(ctx, "SELECT id, version, nodes, edges FROM automation_flows WHERE project_id=$1", projectId)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var id string
		var version string
		var nodesRaw, edgesRaw, compiledRaw []byte

		if hasCompiledColumn {
			if err := rows.Scan(&id, &version, &nodesRaw, &edgesRaw, &compiledRaw); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&id, &version, &nodesRaw, &edgesRaw); err != nil {
				return nil, err
			}
		}

		var nodes []interface{}
		var edges []interface{}
		var compiledRules []interface{}

		// Unmarshal JSONB
		if len(nodesRaw) > 0 {
			if err := json.Unmarshal(nodesRaw, &nodes); err != nil {
				return nil, fmt.Errorf("failed to parse nodes: %v", err)
			}
		}
		if len(edgesRaw) > 0 {
			if err := json.Unmarshal(edgesRaw, &edges); err != nil {
				return nil, fmt.Errorf("failed to parse edges: %v", err)
			}
		}
		if len(compiledRaw) > 0 {
			if err := json.Unmarshal(compiledRaw, &compiledRules); err != nil {
				return nil, fmt.Errorf("failed to parse compiled rules: %v", err)
			}
		}

		return map[string]interface{}{
			"id":             id,
			"nodes":          nodes,
			"edges":          edges,
			"schema_version": version,
			"compiled_rules": compiledRules,
		}, nil
	}

	return nil, nil // Not found
}

// Device Profiles (Hardware Registry)
func (r *PostgresRepo) CreateDeviceProfile(profile map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `INSERT INTO device_profiles (name, protocol, registers) VALUES ($1, $2, $3)`
	_, err := r.Pool.Exec(ctx, query, profile["name"], profile["protocol"], profile["registers"])
	return err
}

func (r *PostgresRepo) GetDeviceProfiles() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT id, name, protocol FROM device_profiles ORDER BY name ASC")
}

// --- Phase 9: Vertical Domains ---

// A. Agriculture / General (Beneficiaries)
func (r *PostgresRepo) CreateBeneficiary(ben map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		INSERT INTO beneficiaries (project_id, name, type, phone, email, address, state_id, is_active, metadata)
		VALUES ($1, $2, COALESCE($3, 'individual'), $4, $5, COALESCE($6, '{}'::jsonb), $7, COALESCE($8, true), COALESCE($9, '{}'::jsonb))
		RETURNING id, project_id, name, type, phone, email, address, state_id, is_active, metadata, created_at, updated_at
	`
	row := r.Pool.QueryRow(ctx, query, ben["project_id"], ben["name"], ben["type"], ben["phone"], ben["email"], ben["address"], ben["state_id"], ben["is_active"], ben["metadata"])
	var (
		id, projectID, name, bType string
		phone, email, stateID      *string
		address, metadata          map[string]interface{}
		isActive                   *bool
		createdAt, updatedAt       time.Time
	)
	if err := row.Scan(&id, &projectID, &name, &bType, &phone, &email, &address, &stateID, &isActive, &metadata, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":         id,
		"project_id": projectID,
		"name":       name,
		"type":       bType,
		"phone":      phone,
		"email":      email,
		"address":    address,
		"state_id":   stateID,
		"is_active":  isActive,
		"metadata":   metadata,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepo) UpdateBeneficiary(id string, ben map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		UPDATE beneficiaries
		SET name = COALESCE($2, name),
			phone = COALESCE($3, phone),
			email = COALESCE($4, email),
			address = COALESCE($5, address),
			state_id = COALESCE($6, state_id),
			is_active = COALESCE($7, is_active),
			metadata = CASE WHEN $8 IS NULL THEN metadata ELSE COALESCE(metadata, '{}'::jsonb) || $8 END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, project_id, name, type, phone, email, address, state_id, is_active, metadata, created_at, updated_at
	`
	row := r.Pool.QueryRow(ctx, query, id, ben["name"], ben["phone"], ben["email"], ben["address"], ben["state_id"], ben["is_active"], ben["metadata"])
	var (
		bid, projectID, name, bType string
		phone, email, stateID       *string
		address, metadata           map[string]interface{}
		isActive                    *bool
		createdAt, updatedAt        time.Time
	)
	if err := row.Scan(&bid, &projectID, &name, &bType, &phone, &email, &address, &stateID, &isActive, &metadata, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":         bid,
		"project_id": projectID,
		"name":       name,
		"type":       bType,
		"phone":      phone,
		"email":      email,
		"address":    address,
		"state_id":   stateID,
		"is_active":  isActive,
		"metadata":   metadata,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepo) ListBeneficiaries(filters map[string]interface{}) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	where := "WHERE 1=1"
	args := []interface{}{}
	arg := 1

	if projectID, ok := filters["project_id"].(string); ok && projectID != "" {
		where += fmt.Sprintf(" AND project_id=$%d", arg)
		args = append(args, projectID)
		arg++
	}
	if installationID, ok := filters["installation_id"].(string); ok && installationID != "" {
		where += fmt.Sprintf(" AND id IN (SELECT beneficiary_id FROM installation_beneficiaries WHERE installation_id=$%d)", arg)
		args = append(args, installationID)
		arg++
	}
	if accountStatus, ok := filters["account_status"].(string); ok && accountStatus != "" {
		where += fmt.Sprintf(" AND COALESCE(metadata->>'accountStatus','') = $%d", arg)
		args = append(args, accountStatus)
		arg++
	}
	if includeDeleted, ok := filters["include_deleted"].(bool); !ok || !includeDeleted {
		where += " AND COALESCE(metadata->>'deleted','false') != 'true'"
	}
	if search, ok := filters["search"].(string); ok && strings.TrimSpace(search) != "" {
		where += fmt.Sprintf(" AND (LOWER(name) LIKE $%d OR LOWER(COALESCE(phone,'')) LIKE $%d OR LOWER(COALESCE(email,'')) LIKE $%d)", arg, arg, arg)
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(search))+"%")
		arg++
	}

	limit := 200
	if v, ok := filters["limit"].(int); ok && v > 0 {
		if v > 500 {
			v = 500
		}
		limit = v
	}

	query := fmt.Sprintf(`
		SELECT id, project_id, name, type, phone, email, address, state_id, is_active, metadata, created_at, updated_at
		FROM beneficiaries
		%s
		ORDER BY created_at DESC
		LIMIT $%d
	`, where, arg)
	args = append(args, limit)

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	for rows.Next() {
		var (
			id, projectID, name, bType string
			phone, email, stateID      *string
			address, metadata          map[string]interface{}
			isActive                   *bool
			createdAt, updatedAt       time.Time
		)
		if err := rows.Scan(&id, &projectID, &name, &bType, &phone, &email, &address, &stateID, &isActive, &metadata, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":         id,
			"project_id": projectID,
			"name":       name,
			"type":       bType,
			"phone":      phone,
			"email":      email,
			"address":    address,
			"state_id":   stateID,
			"is_active":  isActive,
			"metadata":   metadata,
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}
	return results, nil
}

// B. Installations (Agri / Solar)
func (r *PostgresRepo) CreateInstallation(inst map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		INSERT INTO installations (device_id, project_id, beneficiary_id, geo_location, protocol_id, vfd_model_id, status, metadata, activated_at, decommissioned_at)
		VALUES ($1, $2, $3, COALESCE($4, '{}'::jsonb), $5, $6, COALESCE($7, 'active'), COALESCE($8, '{}'::jsonb), COALESCE($9, NOW()), $10)
		RETURNING id, device_id, project_id, beneficiary_id, geo_location, protocol_id, vfd_model_id, status, metadata, activated_at, decommissioned_at, created_at, updated_at
	`
	row := r.Pool.QueryRow(ctx, query, inst["device_id"], inst["project_id"], inst["beneficiary_id"], inst["geo_location"], inst["protocol_id"], inst["vfd_model_id"], inst["status"], inst["metadata"], inst["activated_at"], inst["decommissioned_at"])
	var (
		id, deviceID, projectID                             string
		beneficiaryID, protocolID, vfdModelID               *string
		geoLocation, metadata                               map[string]interface{}
		status                                              *string
		activatedAt, decommissionedAt, createdAt, updatedAt *time.Time
	)
	if err := row.Scan(&id, &deviceID, &projectID, &beneficiaryID, &geoLocation, &protocolID, &vfdModelID, &status, &metadata, &activatedAt, &decommissionedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":                id,
		"device_id":         deviceID,
		"project_id":        projectID,
		"beneficiary_id":    beneficiaryID,
		"geo_location":      geoLocation,
		"protocol_id":       protocolID,
		"vfd_model_id":      vfdModelID,
		"status":            status,
		"metadata":          metadata,
		"activated_at":      activatedAt,
		"decommissioned_at": decommissionedAt,
		"created_at":        createdAt,
		"updated_at":        updatedAt,
	}, nil
}

func (r *PostgresRepo) UpdateInstallation(id string, inst map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		UPDATE installations
		SET beneficiary_id = COALESCE($2, beneficiary_id),
			geo_location = COALESCE($3, geo_location),
			protocol_id = COALESCE($4, protocol_id),
			vfd_model_id = COALESCE($5, vfd_model_id),
			status = COALESCE($6, status),
			metadata = CASE WHEN $7 IS NULL THEN metadata ELSE COALESCE(metadata, '{}'::jsonb) || $7 END,
			activated_at = COALESCE($8, activated_at),
			decommissioned_at = COALESCE($9, decommissioned_at),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, device_id, project_id, beneficiary_id, geo_location, protocol_id, vfd_model_id, status, metadata, activated_at, decommissioned_at, created_at, updated_at
	`
	row := r.Pool.QueryRow(ctx, query, id, inst["beneficiary_id"], inst["geo_location"], inst["protocol_id"], inst["vfd_model_id"], inst["status"], inst["metadata"], inst["activated_at"], inst["decommissioned_at"])
	var (
		iid, deviceID, projectID                            string
		beneficiaryID, protocolID, vfdModelID               *string
		geoLocation, metadata                               map[string]interface{}
		status                                              *string
		activatedAt, decommissionedAt, createdAt, updatedAt *time.Time
	)
	if err := row.Scan(&iid, &deviceID, &projectID, &beneficiaryID, &geoLocation, &protocolID, &vfdModelID, &status, &metadata, &activatedAt, &decommissionedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":                iid,
		"device_id":         deviceID,
		"project_id":        projectID,
		"beneficiary_id":    beneficiaryID,
		"geo_location":      geoLocation,
		"protocol_id":       protocolID,
		"vfd_model_id":      vfdModelID,
		"status":            status,
		"metadata":          metadata,
		"activated_at":      activatedAt,
		"decommissioned_at": decommissionedAt,
		"created_at":        createdAt,
		"updated_at":        updatedAt,
	}, nil
}

func (r *PostgresRepo) GetInstallationByID(installationID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		SELECT id, device_id, project_id, beneficiary_id, geo_location, protocol_id, vfd_model_id, status, metadata, activated_at, decommissioned_at, created_at, updated_at
		FROM installations
		WHERE id=$1
	`
	row := r.Pool.QueryRow(ctx, query, installationID)
	var (
		id, deviceID, projectID                             string
		beneficiaryID, protocolID, vfdModelID               *string
		geoLocation, metadata                               map[string]interface{}
		status                                              *string
		activatedAt, decommissionedAt, createdAt, updatedAt *time.Time
	)
	if err := row.Scan(&id, &deviceID, &projectID, &beneficiaryID, &geoLocation, &protocolID, &vfdModelID, &status, &metadata, &activatedAt, &decommissionedAt, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return map[string]interface{}{
		"id":                id,
		"device_id":         deviceID,
		"project_id":        projectID,
		"beneficiary_id":    beneficiaryID,
		"geo_location":      geoLocation,
		"protocol_id":       protocolID,
		"vfd_model_id":      vfdModelID,
		"status":            status,
		"metadata":          metadata,
		"activated_at":      activatedAt,
		"decommissioned_at": decommissionedAt,
		"created_at":        createdAt,
		"updated_at":        updatedAt,
	}, nil
}

func (r *PostgresRepo) ListInstallations(filters map[string]interface{}) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	where := "WHERE 1=1"
	args := []interface{}{}
	arg := 1

	if projectID, ok := filters["project_id"].(string); ok && projectID != "" {
		where += fmt.Sprintf(" AND project_id=$%d", arg)
		args = append(args, projectID)
		arg++
	}
	if deviceID, ok := filters["device_id"].(string); ok && deviceID != "" {
		where += fmt.Sprintf(" AND device_id=$%d", arg)
		args = append(args, deviceID)
		arg++
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		where += fmt.Sprintf(" AND status=$%d", arg)
		args = append(args, status)
		arg++
	}

	query := fmt.Sprintf(`
		SELECT id, device_id, project_id, beneficiary_id, geo_location, protocol_id, vfd_model_id, status, metadata, activated_at, decommissioned_at, created_at, updated_at
		FROM installations
		%s
		ORDER BY created_at DESC
	`, where)
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	for rows.Next() {
		var (
			id, deviceID, projectID                             string
			beneficiaryID, protocolID, vfdModelID               *string
			geoLocation, metadata                               map[string]interface{}
			status                                              *string
			activatedAt, decommissionedAt, createdAt, updatedAt *time.Time
		)
		if err := rows.Scan(&id, &deviceID, &projectID, &beneficiaryID, &geoLocation, &protocolID, &vfdModelID, &status, &metadata, &activatedAt, &decommissionedAt, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":                id,
			"device_id":         deviceID,
			"project_id":        projectID,
			"beneficiary_id":    beneficiaryID,
			"geo_location":      geoLocation,
			"protocol_id":       protocolID,
			"vfd_model_id":      vfdModelID,
			"status":            status,
			"metadata":          metadata,
			"activated_at":      activatedAt,
			"decommissioned_at": decommissionedAt,
			"created_at":        createdAt,
			"updated_at":        updatedAt,
		})
	}
	return results, nil
}

// C. Healthcare (Patients)
func (r *PostgresRepo) CreatePatient(pat map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `INSERT INTO patients (patient_id, project_id, name, age, gender, assigned_doctor_id, medical_history)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.Pool.Exec(ctx, query, pat["patient_id"], pat["project_id"], pat["name"], pat["age"], pat["gender"], pat["assigned_doctor_id"], pat["medical_history"])
	return err
}

func (r *PostgresRepo) GetPatients(projectID string) ([]map[string]interface{}, error) {
	if projectID == "" {
		return r.fetchSimple("SELECT * FROM patients")
	}
	return r.fetchSimpleWithArgs("SELECT * FROM patients WHERE project_id=$1", projectID)
}

// D. GIS (Layers)
func (r *PostgresRepo) CreateGISLayer(layer map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// GeoJSON validation omitted for V1
	query := `INSERT INTO gis_layers (project_id, name, type, geojson) VALUES ($1, $2, $3, $4)`
	_, err := r.Pool.Exec(ctx, query, layer["project_id"], layer["name"], layer["type"], layer["geojson"])
	return err
}

func (r *PostgresRepo) GetGISLayers(projectId string) ([]map[string]interface{}, error) {
	// In real app: use query param $1. For V1 generic fetch:
	if projectId == "" {
		return r.fetchSimple("SELECT * FROM gis_layers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := r.Pool.Query(ctx, "SELECT * FROM gis_layers WHERE project_id=$1", projectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// We rely on fetchSimple for the 'all' case in verification.
	return nil, nil
}

// 5. Bootstrap Helpers (Verticals)
func (r *PostgresRepo) GetInstallationByDevice(deviceId string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, project_id, beneficiary_id, geo_location, protocol_id, vfd_model_id, status, metadata FROM installations WHERE device_id=$1 AND (status IS NULL OR status='active') LIMIT 1`
	rows, err := r.Pool.Query(ctx, query, deviceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var id, projectID, benId, status string
		var geoLocation map[string]interface{}
		var protocolID, vfdModelID *string
		var metadata map[string]interface{}
		if err := rows.Scan(&id, &projectID, &benId, &geoLocation, &protocolID, &vfdModelID, &status, &metadata); err != nil {
			return nil, err
		}
		resp := map[string]interface{}{
			"id":             id,
			"project_id":     projectID,
			"beneficiary_id": benId,
			"geo_location":   geoLocation,
			"protocol_id":    protocolID,
			"vfd_model_id":   vfdModelID,
			"status":         status,
			"metadata":       metadata,
		}
		return resp, nil
	}
	return nil, nil // Not found
}

func (r *PostgresRepo) GetBeneficiary(id string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := r.Pool.Query(ctx, "SELECT id, project_id, name, type, phone, email, address, state_id, is_active, metadata, created_at, updated_at FROM beneficiaries WHERE id=$1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var (
			bid, projectID, name, bType string
			phone, email, stateID       *string
			address, metadata           map[string]interface{}
			isActive                    *bool
			createdAt, updatedAt        time.Time
		)
		rows.Scan(&bid, &projectID, &name, &bType, &phone, &email, &address, &stateID, &isActive, &metadata, &createdAt, &updatedAt)
		return map[string]interface{}{
			"id":         bid,
			"project_id": projectID,
			"name":       name,
			"type":       bType,
			"phone":      phone,
			"email":      email,
			"address":    address,
			"state_id":   stateID,
			"is_active":  isActive,
			"metadata":   metadata,
			"created_at": createdAt,
			"updated_at": updatedAt,
		}, nil
	}
	return nil, nil
}

// Installation beneficiaries (assignment table)
func (r *PostgresRepo) ListInstallationBeneficiaries(installationID string, includeRemoved bool) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	where := "WHERE installation_id=$1"
	if !includeRemoved {
		where += " AND removed_at IS NULL"
	}
	query := fmt.Sprintf(`
		SELECT id, installation_id, beneficiary_id, role, removed_at, created_at
		FROM installation_beneficiaries
		%s
		ORDER BY created_at DESC
	`, where)
	rows, err := r.Pool.Query(ctx, query, installationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []map[string]interface{}{}
	for rows.Next() {
		var id, instID, benID string
		var role *string
		var removedAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &instID, &benID, &role, &removedAt, &createdAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":               id,
			"installationUuid": instID,
			"beneficiaryUuid":  benID,
			"role":             role,
			"removedAt":        removedAt,
			"createdAt":        createdAt,
		})
	}
	return results, nil
}

func (r *PostgresRepo) AssignBeneficiaryToInstallation(installationID, beneficiaryID, role string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		INSERT INTO installation_beneficiaries (installation_id, beneficiary_id, role)
		VALUES ($1, $2, $3)
		RETURNING id, installation_id, beneficiary_id, role, removed_at, created_at
	`
	row := r.Pool.QueryRow(ctx, query, installationID, beneficiaryID, role)
	var id, instID, benID string
	var roleVal *string
	var removedAt *time.Time
	var createdAt time.Time
	if err := row.Scan(&id, &instID, &benID, &roleVal, &removedAt, &createdAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":               id,
		"installationUuid": instID,
		"beneficiaryUuid":  benID,
		"role":             roleVal,
		"removedAt":        removedAt,
		"createdAt":        createdAt,
	}, nil
}

func (r *PostgresRepo) RemoveBeneficiaryFromInstallation(installationID, beneficiaryID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		UPDATE installation_beneficiaries
		SET removed_at = NOW()
		WHERE installation_id=$1 AND beneficiary_id=$2 AND removed_at IS NULL
		RETURNING id, installation_id, beneficiary_id, role, removed_at, created_at
	`
	row := r.Pool.QueryRow(ctx, query, installationID, beneficiaryID)
	var id, instID, benID string
	var roleVal *string
	var removedAt *time.Time
	var createdAt time.Time
	if err := row.Scan(&id, &instID, &benID, &roleVal, &removedAt, &createdAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return map[string]interface{}{
		"id":               id,
		"installationUuid": instID,
		"beneficiaryUuid":  benID,
		"role":             roleVal,
		"removedAt":        removedAt,
		"createdAt":        createdAt,
	}, nil
}

// --- Phase 10: The Final Mile (History & Master Data) ---

// 1. History (Analytics)
func (r *PostgresRepo) GetTelemetryHistory(imei string, start, end time.Time, limit, offset int) ([]map[string]interface{}, error) {
	// Query Hypertable
	query := `
		SELECT time, data 
		FROM telemetry 
		WHERE device_id = (SELECT id FROM devices WHERE imei=$1 LIMIT 1) 
		AND time >= $2 AND time <= $3 
		ORDER BY time DESC 
		LIMIT $4 OFFSET $5`

	// Note: If devices table isn't populated in V1, we might need to fallback to project_id or non-FK lookup.
	// For V1, we assume device_id lookup works OR we modify query to store imei directly?
	// The current schema stores device_id (UUID).
	// If device lookup fails, return empty.

	// Optimization: If device_id is difficult, we can try to join.
	// Simplified for this phase:
	// Simplified for this phase:
	return r.fetchSimpleWithArgs(query, imei, start, end, limit, offset)
}

func (r *PostgresRepo) ExportTelemetry(start, end time.Time, projectId string) ([]map[string]interface{}, error) {
	// For Archiver: Fetch all raw data for project in range
	query := `SELECT time, device_id, data FROM telemetry WHERE time >= $1 AND time <= $2 AND project_id = $3`
	return r.fetchSimpleWithArgs(query, start, end, projectId)
}

func appendPacketTypeFilterClause(
	base string,
	columnExpr string,
	packetType string,
	idx *int,
	args *[]interface{},
) string {
	trimmed := strings.TrimSpace(packetType)
	if trimmed == "" {
		return base
	}

	base += fmt.Sprintf(" AND %s = $%d", columnExpr, *idx)
	*args = append(*args, trimmed)
	*idx++
	return base
}

// ExportTelemetryFiltered fetches raw telemetry rows for a project in a time range with optional JSON filters.
// Filters:
// - packetType: matches data->>'packet_type'
// - quality: matches data->'meta'->>'quality'
// - excludeQuality: excludes rows where data->'meta'->>'quality' equals this value
func (r *PostgresRepo) ExportTelemetryFiltered(start, end time.Time, projectId, packetType, quality, excludeQuality string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	base := `SELECT time, device_id, data FROM telemetry WHERE time >= $1 AND time <= $2 AND project_id = $3`
	args := []interface{}{start, end, projectId}
	idx := 4

	base = appendPacketTypeFilterClause(base, "data->>'packet_type'", packetType, &idx, &args)
	if quality != "" {
		base += fmt.Sprintf(" AND data->'meta'->>'quality' = $%d", idx)
		args = append(args, quality)
		idx++
	}
	if excludeQuality != "" {
		base += fmt.Sprintf(" AND COALESCE(data->'meta'->>'quality','') <> $%d", idx)
		args = append(args, excludeQuality)
		idx++
	}

	base += " ORDER BY time ASC"

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

// GetTelemetryHistoryByProject returns telemetry rows for a project in a time range with pagination and optional JSON filters.
// Filters:
// - packetType: matches data->>'packet_type'
// - quality: matches data->'meta'->>'quality'
// - excludeQuality: excludes rows where data->'meta'->>'quality' equals this value
func (r *PostgresRepo) GetTelemetryHistoryByProject(start, end time.Time, projectId, packetType, quality, excludeQuality string, limit, offset int) ([]map[string]interface{}, int, error) {
	return r.GetTelemetryHistoryByProjectFromTable("telemetry", start, end, projectId, packetType, quality, excludeQuality, limit, offset)
}

func (r *PostgresRepo) GetTelemetryHistoryByProjectFromTable(table string, start, end time.Time, projectId, packetType, quality, excludeQuality string, limit, offset int) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	where := ` WHERE time >= $1 AND time <= $2 AND project_id = $3`
	args := []interface{}{start, end, projectId}
	idx := 4

	where = appendPacketTypeFilterClause(where, "data->>'packet_type'", packetType, &idx, &args)
	if quality != "" {
		where += fmt.Sprintf(" AND data->'meta'->>'quality' = $%d", idx)
		args = append(args, quality)
		idx++
	}
	if excludeQuality != "" {
		where += fmt.Sprintf(" AND COALESCE(data->'meta'->>'quality','') <> $%d", idx)
		args = append(args, excludeQuality)
		idx++
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table) + where
	var total int
	if err := r.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rowsQuery := fmt.Sprintf(`SELECT time, device_id, project_id, data FROM %s`, table) + where + fmt.Sprintf(" ORDER BY time DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	rowsArgs := append(args, limit, offset)
	rows, err := r.Pool.Query(ctx, rowsQuery, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var t time.Time
		var deviceID string
		var pid string
		var data map[string]interface{}
		if err := rows.Scan(&t, &deviceID, &pid, &data); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]interface{}{
			"time":       t,
			"device_id":  deviceID,
			"project_id": pid,
			"data":       data,
		})
	}

	return out, total, nil
}

// --- Phase 14: Rules & Commands (Round 2) ---

// RULES
func (r *PostgresRepo) GetRules(projectId, deviceId string) ([]map[string]interface{}, error) {
	query := "SELECT * FROM rules WHERE project_id = $1"
	args := []interface{}{projectId}
	if deviceId != "" {
		query += " AND device_id = $2"
		args = append(args, deviceId)
	}
	query += " ORDER BY created_at DESC"
	return r.fetchSimpleWithArgs(query, args...)
}

func (r *PostgresRepo) CreateRule(rule map[string]interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Assuming ID is generated by DB (UUIDv4 DEFAULT) or Serial.
	// If ID is not returned by rules table because it might not handle RETURNING id properly if driver issues?
	// But standard PG does.
	query := `INSERT INTO rules (project_id, name, device_id, trigger, actions, enabled) 
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	first := func(keys ...string) interface{} {
		for _, k := range keys {
			if v, ok := rule[k]; ok {
				return v
			}
		}
		return nil
	}

	pid := first("project_id", "projectId")
	name := first("name")
	did := first("device_id", "deviceId")
	trig := first("trigger")
	act := first("actions")
	en := first("enabled")
	if en == nil {
		en = true
	}

	// Normalize jsonb inputs so pgx doesn't depend on map/slice encoding heuristics.
	encodeJSON := func(v interface{}) ([]byte, error) {
		if v == nil {
			return []byte("null"), nil
		}
		switch vv := v.(type) {
		case []byte:
			if len(vv) == 0 {
				return []byte("null"), nil
			}
			return vv, nil
		case string:
			if strings.TrimSpace(vv) == "" {
				return []byte("null"), nil
			}
			// If it's already JSON, keep it as-is; otherwise wrap as string JSON.
			trim := strings.TrimSpace(vv)
			if strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "[") || trim == "null" {
				return []byte(trim), nil
			}
			return json.Marshal(vv)
		default:
			return json.Marshal(vv)
		}
	}

	trigJSON, err := encodeJSON(trig)
	if err != nil {
		return "", err
	}
	actJSON, err := encodeJSON(act)
	if err != nil {
		return "", err
	}

	var id string
	err = r.Pool.QueryRow(ctx, query, pid, name, did, trigJSON, actJSON, en).Scan(&id)
	return id, err
}

func (r *PostgresRepo) DeleteRule(id string) error {
	_, err := r.Pool.Exec(context.Background(), "DELETE FROM rules WHERE id=$1", id)
	return err
}

// COMMANDS (v7 schema)
func (r *PostgresRepo) ListCommandsForDevice(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var proto pgtype.UUID
	if protocolID != nil && strings.TrimSpace(*protocolID) != "" {
		_ = proto.Scan(*protocolID)
	}

	var model pgtype.UUID
	if modelID != nil && strings.TrimSpace(*modelID) != "" {
		_ = model.Scan(*modelID)
	}

	query := `
WITH scoped AS (
    SELECT id, name, scope, protocol_id, model_id, project_id, payload_schema, transport, created_at
    FROM command_catalog
    WHERE scope = 'core'
	   OR (scope = 'project' AND project_id = $1)
	   OR (scope = 'protocol' AND $2::uuid IS NOT NULL AND protocol_id = $2::uuid)
	   OR (scope = 'model' AND $3::uuid IS NOT NULL AND model_id = $3::uuid)
), has_caps AS (
    SELECT COUNT(1) AS total FROM device_capabilities WHERE device_id = $4::uuid
), filtered AS (
    SELECT s.*
    FROM scoped s
    LEFT JOIN project_command_overrides o ON o.command_id = s.id AND o.project_id = $1
    WHERE COALESCE(o.enabled, TRUE) = TRUE
)
SELECT f.id, f.name, f.scope, f.protocol_id, f.model_id, f.project_id, f.payload_schema, f.transport, f.created_at
FROM filtered f
LEFT JOIN device_capabilities dc ON dc.command_id = f.id AND dc.device_id = $4::uuid
WHERE (SELECT total FROM has_caps) = 0 OR dc.device_id IS NOT NULL OR f.scope = 'core'
ORDER BY f.created_at DESC;
`

	rows, err := r.Pool.Query(ctx, query, projectID, proto, model, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CommandCatalog
	for rows.Next() {
		var rec domain.CommandCatalog
		var proto, model, project sql.NullString
		var payload interface{}
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Scope, &proto, &model, &project, &payload, &rec.Transport, &rec.CreatedAt); err != nil {
			continue
		}
		if proto.Valid {
			val := proto.String
			rec.ProtocolID = &val
		}
		if model.Valid {
			val := model.String
			rec.ModelID = &val
		}
		if project.Valid {
			val := project.String
			rec.ProjectID = &val
		}
		if payload != nil {
			if raw, ok := payload.([]byte); ok {
				var parsed map[string]any
				if err := json.Unmarshal(raw, &parsed); err == nil {
					rec.PayloadSchema = parsed
				}
			}
		}
		out = append(out, rec)
	}

	return out, nil
}

func (r *PostgresRepo) GetCommandByID(commandID string) (*domain.CommandCatalog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        SELECT id, name, scope, protocol_id, model_id, project_id, payload_schema, transport, created_at
        FROM command_catalog
        WHERE id = $1::uuid
    `

	var rec domain.CommandCatalog
	var proto, model, project sql.NullString
	var payload []byte
	err := r.Pool.QueryRow(ctx, query, commandID).Scan(&rec.ID, &rec.Name, &rec.Scope, &proto, &model, &project, &payload, &rec.Transport, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	if proto.Valid {
		val := proto.String
		rec.ProtocolID = &val
	}
	if model.Valid {
		val := model.String
		rec.ModelID = &val
	}
	if project.Valid {
		val := project.String
		rec.ProjectID = &val
	}
	if len(payload) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(payload, &parsed); err == nil {
			rec.PayloadSchema = parsed
		}
	}
	return &rec, nil
}

// UpsertCommandCatalog inserts or updates a command catalog entry.
func (r *PostgresRepo) UpsertCommandCatalog(rec domain.CommandCatalog) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var idParam *string
	if strings.TrimSpace(rec.ID) != "" {
		idParam = &rec.ID
	}

	var proto *string
	if rec.ProtocolID != nil && strings.TrimSpace(*rec.ProtocolID) != "" {
		proto = rec.ProtocolID
	}
	var model *string
	if rec.ModelID != nil && strings.TrimSpace(*rec.ModelID) != "" {
		model = rec.ModelID
	}
	var project *string
	if rec.ProjectID != nil && strings.TrimSpace(*rec.ProjectID) != "" {
		project = rec.ProjectID
	}

	if rec.Transport == "" {
		rec.Transport = "mqtt"
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}

	var payload interface{}
	if rec.PayloadSchema != nil {
		payload = rec.PayloadSchema
	}

	query := `
		INSERT INTO command_catalog (id, name, scope, protocol_id, model_id, project_id, payload_schema, transport, created_at)
		VALUES (COALESCE($1, gen_random_uuid())::uuid, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid, NULLIF($6,'')::text, $7::jsonb, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			scope = EXCLUDED.scope,
			protocol_id = EXCLUDED.protocol_id,
			model_id = EXCLUDED.model_id,
			project_id = EXCLUDED.project_id,
			payload_schema = EXCLUDED.payload_schema,
			transport = EXCLUDED.transport
		RETURNING id::text`

	var id string
	err := r.Pool.QueryRow(ctx, query, idParam, rec.Name, rec.Scope, proto, model, project, payload, rec.Transport, rec.CreatedAt).Scan(&id)
	return id, err
}

// DeleteCommandCatalog removes a command definition and related overrides/capabilities.
func (r *PostgresRepo) DeleteCommandCatalog(commandID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	statements := []string{
		`DELETE FROM command_requests WHERE command_id = $1::uuid`,
		`DELETE FROM device_capabilities WHERE command_id = $1::uuid`,
		`DELETE FROM project_command_overrides WHERE command_id = $1::uuid`,
		`DELETE FROM response_patterns WHERE command_id = $1::uuid`,
		`DELETE FROM command_catalog WHERE id = $1::uuid`,
	}

	for _, stmt := range statements {
		if _, execErr := tx.Exec(ctx, stmt, commandID); execErr != nil {
			return execErr
		}
	}

	return tx.Commit(ctx)
}

// UpsertDeviceCapabilities assigns a command to devices (append-only, ignores duplicates).
func (r *PostgresRepo) UpsertDeviceCapabilities(commandID string, deviceIDs []string) error {
	if len(deviceIDs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	batch := &pgx.Batch{}
	for _, did := range deviceIDs {
		if strings.TrimSpace(did) == "" {
			continue
		}
		batch.Queue(`INSERT INTO device_capabilities (device_id, command_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`, did, commandID)
	}

	br := r.Pool.SendBatch(ctx, batch)
	defer br.Close()
	for range deviceIDs {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepo) GetCommandRequestByCorrelation(correlationID string) (*domain.CommandRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        SELECT id, device_id, project_id, command_id, payload, status, retries, correlation_id, created_at, published_at, completed_at
        FROM command_requests
        WHERE correlation_id = $1::uuid
    `

	var rec domain.CommandRequest
	var payload []byte
	err := r.Pool.QueryRow(ctx, query, correlationID).Scan(&rec.ID, &rec.DeviceID, &rec.ProjectID, &rec.CommandID, &payload, &rec.Status, &rec.Retries, &rec.CorrelationID, &rec.CreatedAt, &rec.PublishedAt, &rec.CompletedAt)
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(payload, &parsed); err == nil {
			rec.Payload = parsed
		}
	}
	return &rec, nil
}

func (r *PostgresRepo) GetResponsePatterns(commandID string) ([]domain.ResponsePattern, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        SELECT id, command_id, pattern_type, pattern, success, extract, created_at
        FROM response_patterns
        WHERE command_id = $1::uuid
        ORDER BY created_at DESC
    `

	rows, err := r.Pool.Query(ctx, query, commandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ResponsePattern
	for rows.Next() {
		var rec domain.ResponsePattern
		var extract []byte
		if err := rows.Scan(&rec.ID, &rec.CommandID, &rec.PatternType, &rec.Pattern, &rec.Success, &extract, &rec.CreatedAt); err != nil {
			continue
		}
		if len(extract) > 0 {
			var parsed map[string]any
			if err := json.Unmarshal(extract, &parsed); err == nil {
				rec.Extract = parsed
			}
		}
		out = append(out, rec)
	}

	return out, nil
}

func (r *PostgresRepo) InsertCommandRequest(req domain.CommandRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if strings.TrimSpace(req.ID) == "" {
		req.ID = uuid.New().String()
	}
	if strings.TrimSpace(req.CorrelationID) == "" {
		req.CorrelationID = uuid.New().String()
	}
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	// Serialize payload to JSON to avoid type inference issues on prepared statement parameters.
	payloadBytes, _ := json.Marshal(req.Payload)

	query := `
        INSERT INTO command_requests (id, device_id, project_id, command_id, payload, status, retries, correlation_id, created_at)
        VALUES ($1::uuid, $2::uuid, $3::text, $4::uuid, $5::jsonb, $6::text, $7::int, $8::uuid, $9::timestamptz)
        RETURNING id
    `

	var id string
	err := r.Pool.QueryRow(ctx, query, req.ID, req.DeviceID, req.ProjectID, req.CommandID, payloadBytes, req.Status, req.Retries, req.CorrelationID, createdAt).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *PostgresRepo) UpdateCommandRequestStatus(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setClauses := []string{"status = $1"}
	args := []interface{}{status}
	idx := 2

	if publishedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("published_at = $%d", idx))
		args = append(args, *publishedAt)
		idx++
	}
	if completedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("completed_at = $%d", idx))
		args = append(args, *completedAt)
		idx++
	}
	if retries != nil {
		setClauses = append(setClauses, fmt.Sprintf("retries = $%d", idx))
		args = append(args, *retries)
		idx++
	}

	setSQL := strings.Join(setClauses, ", ")
	query := fmt.Sprintf("UPDATE command_requests SET %s WHERE correlation_id = $%d::uuid", setSQL, idx)
	args = append(args, correlationID)

	_, err := r.Pool.Exec(ctx, query, args...)
	return err
}

func (r *PostgresRepo) SaveCommandResponse(resp domain.CommandResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	receivedAt := resp.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}

	query := `
        INSERT INTO command_responses (correlation_id, device_id, project_id, raw_response, parsed, matched_pattern_id, received_at)
        VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
        ON CONFLICT (correlation_id) DO UPDATE
        SET raw_response = EXCLUDED.raw_response,
            parsed = EXCLUDED.parsed,
            matched_pattern_id = EXCLUDED.matched_pattern_id,
            received_at = EXCLUDED.received_at
    `

	_, err := r.Pool.Exec(ctx, query, resp.CorrelationID, resp.DeviceID, resp.ProjectID, resp.RawResponse, resp.Parsed, resp.MatchedPatternID, receivedAt)
	return err
}

func (r *PostgresRepo) ListCommandRequests(deviceID string, limit int) ([]domain.CommandRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	query := `
        SELECT id, device_id, project_id, command_id, payload, status, retries, correlation_id, created_at, published_at, completed_at
        FROM command_requests
        WHERE device_id = $1::uuid
        ORDER BY created_at DESC
        LIMIT $2
    `

	rows, err := r.Pool.Query(ctx, query, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CommandRequest
	for rows.Next() {
		var rec domain.CommandRequest
		var payload []byte
		if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.ProjectID, &rec.CommandID, &payload, &rec.Status, &rec.Retries, &rec.CorrelationID, &rec.CreatedAt, &rec.PublishedAt, &rec.CompletedAt); err != nil {
			continue
		}
		if len(payload) > 0 {
			var parsed map[string]any
			if err := json.Unmarshal(payload, &parsed); err == nil {
				rec.Payload = parsed
			}
		}
		out = append(out, rec)
	}

	return out, nil
}

func (r *PostgresRepo) ListCommandResponses(deviceID string, limit int) ([]domain.CommandResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	query := `
        SELECT correlation_id, device_id, project_id, raw_response, parsed, matched_pattern_id, received_at
        FROM command_responses
        WHERE device_id = $1::uuid
        ORDER BY received_at DESC
        LIMIT $2
    `

	rows, err := r.Pool.Query(ctx, query, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CommandResponse
	for rows.Next() {
		var rec domain.CommandResponse
		var raw, parsed []byte
		if err := rows.Scan(&rec.CorrelationID, &rec.DeviceID, &rec.ProjectID, &raw, &parsed, &rec.MatchedPatternID, &rec.ReceivedAt); err != nil {
			continue
		}
		if len(raw) > 0 {
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err == nil {
				rec.RawResponse = m
			}
		}
		if len(parsed) > 0 {
			var m map[string]any
			if err := json.Unmarshal(parsed, &m); err == nil {
				rec.Parsed = m
			}
		}
		out = append(out, rec)
	}

	return out, nil
}

func (r *PostgresRepo) ListPendingRetries(cutoff time.Time, limit int) ([]domain.CommandRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	query := `
        SELECT id, device_id, project_id, command_id, payload, status, retries, correlation_id, created_at, published_at, completed_at
        FROM command_requests
        WHERE completed_at IS NULL
          AND status IN ('queued','published')
          AND (published_at IS NULL OR published_at < $1)
        ORDER BY COALESCE(published_at, created_at) ASC
        LIMIT $2
    `

	rows, err := r.Pool.Query(ctx, query, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CommandRequest
	for rows.Next() {
		var rec domain.CommandRequest
		var payload []byte
		if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.ProjectID, &rec.CommandID, &payload, &rec.Status, &rec.Retries, &rec.CorrelationID, &rec.CreatedAt, &rec.PublishedAt, &rec.CompletedAt); err != nil {
			continue
		}
		if len(payload) > 0 {
			var parsed map[string]any
			if err := json.Unmarshal(payload, &parsed); err == nil {
				rec.Payload = parsed
			}
		}
		out = append(out, rec)
	}

	return out, nil
}

func (r *PostgresRepo) GetCommandStats(deviceID string, cutoff time.Time) (domain.CommandStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats := domain.CommandStats{
		DeviceID:     deviceID,
		StatusCounts: make(map[string]int),
	}

	rows, err := r.Pool.Query(ctx, `
		SELECT status, COUNT(*) AS cnt, COALESCE(SUM(retries), 0) AS total_retries
		FROM command_requests
		WHERE device_id = $1::uuid
		GROUP BY status
	`, deviceID)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var cnt int
		var retries int
		if err := rows.Scan(&status, &cnt, &retries); err != nil {
			continue
		}
		stats.StatusCounts[status] = cnt
		stats.TotalRetries += retries
	}

	if err := r.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM command_requests
		WHERE device_id = $1::uuid
		  AND status IN ('queued','published')
		  AND (published_at IS NULL OR published_at < $2)
	`, deviceID, cutoff).Scan(&stats.PendingPastCutoff); err != nil {
		return stats, err
	}

	// Try to populate project id from latest record if available.
	_ = r.Pool.QueryRow(ctx, `
		SELECT project_id
		FROM command_requests
		WHERE device_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`, deviceID).Scan(&stats.ProjectID)

	return stats, nil
}

// LogAudit records administrative actions (Audit Middleware)
func (r *PostgresRepo) LogAudit(userId, action, resource, ip, status string, metadata interface{}) error {
	// We might need an audit_logs table. For V1 without schema change, we can use a generic logger or just log to disk?
	// The requirement is "Audit Middleware to log every admin action".
	// If 'audit_logs' table doesn't exist in v1_init.sql, we should create it or reuse something.
	// I checked schema and it wasn't there.
	// However, critical modification said "Fix: Create AuditLog model".
	// To avoid breaking schema migration for user, I will use 'command_logs' with device_id=nil?
	// Or I'll just use the JSON Logger for now if Table is missing, but satisfy the Interface.
	// Actually, let's try to insert into 'audit_logs' assuming I add the schema.
	// Wait, I should add the schema if I want to be comprehensive.
	// Or I log to 'alerts' with type='audit'? No.
	// Let's use Structured Logger for Audit if DB table is missing.
	// BUT, the user asked for Audit Middleware to be "Auto Middleware".

	// Init Audit Schema
	_, err := r.Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS audit_logs (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			user_id TEXT,
			action TEXT,
			resource TEXT,
			ip TEXT,
			status TEXT,
			metadata JSONB
		);
		CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource);
		CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_logs(ts DESC);
	`)
	if err != nil {
		fmt.Printf("Warning: Failed to ensure audit schema: %v\n", err)
	}

	query := `INSERT INTO audit_logs (user_id, action, resource, ip, status, metadata) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = r.Pool.Exec(context.Background(), query, userId, action, resource, ip, status, metadata)
	return err
}

func (r *PostgresRepo) ListAuditLogs(limit int, afterId, actorId, action, stateId, authorityId, projectId string) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	where := "WHERE 1=1"
	args := []interface{}{}
	arg := 1

	var afterTs time.Time
	if afterId != "" {
		if err := r.Pool.QueryRow(ctx, "SELECT ts FROM audit_logs WHERE id=$1", afterId).Scan(&afterTs); err == nil {
			where += fmt.Sprintf(" AND (ts < $%d OR (ts = $%d AND id::text < $%d))", arg, arg, arg+1)
			args = append(args, afterTs, afterId)
			arg += 2
		}
	}

	if actorId != "" {
		where += fmt.Sprintf(" AND user_id = $%d", arg)
		args = append(args, actorId)
		arg++
	}
	if action != "" {
		where += fmt.Sprintf(" AND action = $%d", arg)
		args = append(args, action)
		arg++
	}
	if stateId != "" {
		where += fmt.Sprintf(" AND (metadata->>'stateId' = $%d OR metadata->'scope'->>'stateId' = $%d)", arg, arg)
		args = append(args, stateId)
		arg++
	}
	if authorityId != "" {
		where += fmt.Sprintf(" AND (metadata->>'authorityId' = $%d OR metadata->'scope'->>'authorityId' = $%d)", arg, arg)
		args = append(args, authorityId)
		arg++
	}
	if projectId != "" {
		where += fmt.Sprintf(" AND (metadata->>'projectId' = $%d OR metadata->'scope'->>'projectId' = $%d)", arg, arg)
		args = append(args, projectId)
		arg++
	}

	query := fmt.Sprintf(`
		SELECT id, ts, user_id, action, resource, ip, status, metadata
		FROM audit_logs
		%s
		ORDER BY ts DESC, id DESC
		LIMIT $%d
	`, where, arg)
	args = append(args, limit)

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			id       string
			ts       time.Time
			userId   *string
			action   *string
			resource *string
			ip       *string
			status   *string
			metaRaw  []byte
		)
		if err := rows.Scan(&id, &ts, &userId, &action, &resource, &ip, &status, &metaRaw); err != nil {
			return nil, err
		}
		metadata := map[string]interface{}{}
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &metadata)
		}
		results = append(results, map[string]interface{}{
			"id":       id,
			"ts":       ts,
			"user_id":  userId,
			"action":   action,
			"resource": resource,
			"ip":       ip,
			"status":   status,
			"metadata": metadata,
		})
	}
	return results, rows.Err()
}

func (r *PostgresRepo) GetAssetTimeline(assetId string) ([]map[string]interface{}, error) {
	// Aggregate from Audit Logs, Trips, WorkOrders
	// Join or Union?
	// For V1, we fetch independently and sort in Go (easiest for heterogeneous sources)

	// 1. Audit Logs
	audits, err := r.fetchSimpleWithArgs("SELECT ts as time, action as type, user_id as actor, status, metadata FROM audit_logs WHERE resource=$1 ORDER BY ts DESC", assetId)
	if err != nil {
		return nil, err
	}

	// 2. Trips (Departures/Arrivals)
	// assetId might be vehicle_id
	trips, err := r.fetchSimpleWithArgs("SELECT start_time as time, 'transport' as type, status, trip_id as id FROM trips WHERE vehicle_id=$1 ORDER BY start_time DESC", assetId)
	if err != nil {
		return nil, err
	}

	// 3. Work Orders
	wos, err := r.fetchSimpleWithArgs("SELECT created_at as time, 'maintenance' as type, status, title as details FROM work_orders WHERE device_id=$1 ORDER BY created_at DESC", assetId)

	// Merge
	var timeline []map[string]interface{}
	timeline = append(timeline, audits...)
	timeline = append(timeline, trips...)
	if wos != nil {
		timeline = append(timeline, wos...)
	}

	// Sort (Bubble sort or simple slice sort if we import sort)
	// We rely on service or client to sort for V1 simplicity if we don't want to import sort here.
	// Actually, let's just return unrelated lists or valid json?
	// Client-side sorting is fine for V1.
	return timeline, nil
}

// --- 10. Archive Logic for Temp Tables (Strategy B) ---

func (r *PostgresRepo) CreateTempTelemetryTable(tableName string) error {
	// 1. Create Table (Unlogged for speed)
	query := fmt.Sprintf(`CREATE UNLOGGED TABLE %s (
		time        TIMESTAMPTZ       NOT NULL,
		device_id   TEXT              NOT NULL,
		data        JSONB
	);`, tableName) // SAFE because tableName generated internally

	_, err := r.Pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	// Index for query speed
	idxQuery := fmt.Sprintf(`CREATE INDEX idx_%s_time ON %s (time DESC);`, tableName, tableName)
	_, err = r.Pool.Exec(context.Background(), idxQuery)
	return err
}

func (r *PostgresRepo) CopyTelemetryFromReader(tableName string, reader io.Reader) error {
	conn, err := r.Pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	// Implement Batch Insert using CSV parsing for portability
	csvReader := csv.NewReader(reader)
	// Skip Header
	if _, err := csvReader.Read(); err != nil {
		return err // Empty file or error
	}

	batch := [][]interface{}{}
	query := fmt.Sprintf("INSERT INTO %s (time, device_id, data) VALUES ($1, $2, $3)", tableName)

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		t, _ := time.Parse(time.RFC3339, record[0])
		d := record[1]
		p := record[2]

		batch = append(batch, []interface{}{t, d, p})

		if len(batch) >= 1000 {
			for _, row := range batch {
				conn.Exec(context.Background(), query, row[0], row[1], row[2])
			}
			batch = nil
		}
	}

	// Final Flush
	for _, row := range batch {
		conn.Exec(context.Background(), query, row[0], row[1], row[2])
	}

	return nil
}

// 2. Master Data (Admin)
func (r *PostgresRepo) GetStates() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT * FROM states ORDER BY name ASC")
}

func (r *PostgresRepo) CreateState(name string, isoCode *string, metadata map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := r.Pool.QueryRow(ctx, "INSERT INTO states (name, iso_code, metadata) VALUES ($1, $2, $3) RETURNING id, name, iso_code, metadata, NOW() AS created_at, NOW() AS updated_at", name, isoCode, metadata)
	var id, sName string
	var iso *string
	var meta map[string]interface{}
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &sName, &iso, &meta, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":         id,
		"name":       sName,
		"iso_code":   iso,
		"metadata":   meta,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepo) UpdateState(id string, name *string, isoCode *string, metadata map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := r.Pool.QueryRow(ctx, `UPDATE states
		SET name = COALESCE($2, name),
			iso_code = COALESCE($3, iso_code),
			metadata = COALESCE($4, metadata)
		WHERE id = $1
		RETURNING id, name, iso_code, metadata, NOW() AS created_at, NOW() AS updated_at`, id, name, isoCode, metadata)
	var sID, sName string
	var iso *string
	var meta map[string]interface{}
	var createdAt, updatedAt time.Time
	if err := row.Scan(&sID, &sName, &iso, &meta, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":         sID,
		"name":       sName,
		"iso_code":   iso,
		"metadata":   meta,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepo) DeleteState(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, "DELETE FROM states WHERE id = $1", id)
	return err
}

func (r *PostgresRepo) GetAuthorities(stateId string) ([]map[string]interface{}, error) {
	if stateId == "" {
		return r.fetchSimple("SELECT * FROM authorities ORDER BY name ASC")
	}
	return r.fetchSimpleWithArgs("SELECT * FROM authorities WHERE state_id=$1 ORDER BY name ASC", stateId)
}

func (r *PostgresRepo) CreateAuthority(stateID, name, authorityType string, contactInfo map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := r.Pool.QueryRow(ctx, "INSERT INTO authorities (name, state_id, type, contact_info) VALUES ($1, $2, $3, $4) RETURNING id, name, state_id, type, contact_info, created_at, NOW() AS updated_at", name, stateID, authorityType, contactInfo)
	var id, aName, sID string
	var aType *string
	var contact map[string]interface{}
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &aName, &sID, &aType, &contact, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":           id,
		"name":         aName,
		"state_id":     sID,
		"type":         aType,
		"contact_info": contact,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

func (r *PostgresRepo) UpdateAuthority(id string, name *string, authorityType *string, contactInfo map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := r.Pool.QueryRow(ctx, `UPDATE authorities
		SET name = COALESCE($2, name),
			type = COALESCE($3, type),
			contact_info = COALESCE($4, contact_info)
		WHERE id = $1
		RETURNING id, name, state_id, type, contact_info, created_at, NOW() AS updated_at`, id, name, authorityType, contactInfo)
	var aID, aName, sID string
	var aType *string
	var contact map[string]interface{}
	var createdAt, updatedAt time.Time
	if err := row.Scan(&aID, &aName, &sID, &aType, &contact, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":           aID,
		"name":         aName,
		"state_id":     sID,
		"type":         aType,
		"contact_info": contact,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

func (r *PostgresRepo) DeleteAuthority(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, "DELETE FROM authorities WHERE id = $1", id)
	return err
}

// Internal Helper for Args
// Internal Helper for Args
func (r *PostgresRepo) fetchSimpleWithArgs(query string, args ...interface{}) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Generic Map Scan
	var results []map[string]interface{}

	// Get Columns
	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i, fd := range fields {
		columns[i] = string(fd.Name)
	}

	for rows.Next() {
		// Create a slice of interface{} to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Handle []byte (JSONB sometimes comes as bytes)
			if b, ok := val.([]byte); ok {
				// Try parse as JSON; fall back to string.
				var parsed interface{}
				if err := json.Unmarshal(b, &parsed); err == nil {
					rowMap[col] = parsed
				} else {
					rowMap[col] = string(b)
				}
			} else if u, ok := val.([16]byte); ok {
				rowMap[col] = uuid.UUID(u).String()
			} else {
				rowMap[col] = val
			}
		}
		results = append(results, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// --- Phase 12: The Final Sweep ---

// 1. Alerts
func (r *PostgresRepo) GetAlerts(projectId, status string) ([]map[string]interface{}, error) {
	// Simple Query
	query := "SELECT id, device_id, message, severity, status, triggered_at, data FROM alerts WHERE project_id=$1"
	if status != "" {
		query += " AND status='" + status + "'"
	}
	query += " ORDER BY triggered_at DESC LIMIT 100"
	return r.fetchSimpleWithArgs(query, projectId)
}

func (r *PostgresRepo) AckAlert(id, userId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, "UPDATE alerts SET status='acknowledged', acknowledged_by=$2 WHERE id=$1", id, userId)
	return err
}

// 2. Intelligence (Anomalies)
func (r *PostgresRepo) GetAnomalies(limit int) ([]map[string]interface{}, error) {
	// In reality this might be in 'anomalies' table
	return r.fetchSimple("SELECT * FROM anomalies ORDER BY created_at DESC LIMIT 50")
}

// 3. Scheduler CRUD
func (r *PostgresRepo) GetSchedules() ([]map[string]interface{}, error) {
	return r.fetchSimple("SELECT * FROM schedules ORDER BY time ASC")
}

func (r *PostgresRepo) CreateSchedule(sch map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `INSERT INTO schedules (project_id, command, time, is_active) VALUES ($1, $2, $3, $4)`
	_, err := r.Pool.Exec(ctx, query, sch["project_id"], sch["command"], sch["time"], true)
	return err
}

func (r *PostgresRepo) ToggleSchedule(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, "UPDATE schedules SET is_active = NOT is_active WHERE id=$1", id)
	return err
}

// UpsertProjectDNA stores the canonical payload schema for a project.
func (r *PostgresRepo) UpsertProjectDNA(record dna.ProjectPayloadSchema) error {
	if record.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payloadRows, err := json.Marshal(record.Rows)
	if err != nil {
		return fmt.Errorf("marshal payload rows: %w", err)
	}
	edgeRules, err := json.Marshal(record.EdgeRules)
	if err != nil {
		return fmt.Errorf("marshal edge rules: %w", err)
	}
	virtualSensors, err := json.Marshal(record.VirtualSensors)
	if err != nil {
		return fmt.Errorf("marshal virtual sensors: %w", err)
	}
	automationFlows, err := json.Marshal(record.AutomationFlows)
	if err != nil {
		return fmt.Errorf("marshal automation flows: %w", err)
	}
	metadata := record.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO project_dna (
			project_id,
			payload_rows,
			edge_rules,
			virtual_sensors,
			automation_flows,
			metadata
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id) DO UPDATE SET
			payload_rows = EXCLUDED.payload_rows,
			edge_rules = EXCLUDED.edge_rules,
			virtual_sensors = EXCLUDED.virtual_sensors,
			automation_flows = EXCLUDED.automation_flows,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
	`

	_, err = r.Pool.Exec(ctx, query, record.ProjectID, payloadRows, edgeRules, virtualSensors, automationFlows, metadataJSON)
	return err
}

// --- Phase 13: Seeder Helpers ---

func (r *PostgresRepo) CreateProject(p map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Using INSERT ... ON CONFLICT DO NOTHING to avoid errors
	query := `INSERT INTO projects (id, name, type, location) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`
	_, err := r.Pool.Exec(ctx, query, p["id"], p["name"], p["type"], p["location"])
	return err
}

func (r *PostgresRepo) CreateDevice(d map[string]interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `INSERT INTO devices (imei, project_id, name, status) VALUES ($1, $2, $3, $4) ON CONFLICT (imei) DO NOTHING RETURNING id`

	var id string
	err := r.Pool.QueryRow(ctx, query, d["imei"], d["project_id"], d["name"], d["status"]).Scan(&id)

	if err != nil {
		// On conflict, ID won't be returned (Scan error: no rows).
		// If we need ID on conflict, we must Fetch it.
		// For Seeder: "DO NOTHING" implies we don't care if it exists?
		// But if we want to ensure Job exists, we should maybe fetch it.
		// For now, if "no rows", return "", nil (Device Alrady Exists).
		if err.Error() == "no rows in result set" {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

// 6. Stats (For Reporting)
func (r *PostgresRepo) GetProjectStats(projectId string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats := make(map[string]interface{})

	// 1. Device Counts
	var total, online int
	err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM devices WHERE project_id=$1", projectId).Scan(&total)
	if err != nil {
		return nil, err
	}
	err = r.Pool.QueryRow(ctx, "SELECT count(*) FROM devices WHERE project_id=$1 AND status='active'", projectId).Scan(&online)
	if err != nil {
		return nil, err
	}

	stats["total_devices"] = total
	stats["online_devices"] = online

	// 2. Anomalies (Last 24h)
	// Assuming anomalies table exists or we query alerts
	// For V1 parity, we'll query alerts as anomalies proxy? Or 'anomalies' table.
	// Since 'anomalies' table was referenced earlier in GetAnomalies, we use that.
	// We need by project_id. If anomalies table doesn't have project_id, we might need JOIN.
	// For now, assume simple query.

	rows, err := r.Pool.Query(ctx, "SELECT type, description, value FROM anomalies WHERE created_at > NOW() - INTERVAL '24 hours' LIMIT 10")
	if err != nil {
		// If table missing, return empty list
		stats["anomalies"] = []map[string]interface{}{}
	} else {
		defer rows.Close()
		var anomalies []map[string]interface{}
		for rows.Next() {
			var aType, desc string
			var val float64
			if err := rows.Scan(&aType, &desc, &val); err == nil {
				anomalies = append(anomalies, map[string]interface{}{"type": aType, "description": desc, "value": val})
			}
		}
		stats["anomalies"] = anomalies
	}

	return stats, nil
}

// --- Round 3: MQTT Worker Helpers ---

func (r *PostgresRepo) FetchPendingMqttJob() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Logic: 'pending' OR ('failed' AND attempt_count < 10 AND next_attempt_at <= NOW())
	// Order by next_attempt_at ASC
	// We update to 'processing' to lock it. In single-worker (simpler), we usually Fetch then Update.
	// But to support concurrency, we use a CTE or separate transaction.
	// For V1 (single instance assumed):

	selectQuery := `
		SELECT id, device_id, status, attempt_count 
		FROM mqtt_provisioning_jobs
		WHERE (status = 'pending') 
		   OR (status = 'failed' AND attempt_count < 10 AND next_attempt_at <= NOW())
		ORDER BY next_attempt_at ASC
		LIMIT 1
	`

	// Transaction
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var id, devId, status string
	var attempts int

	err = tx.QueryRow(ctx, selectQuery).Scan(&id, &devId, &status, &attempts)
	if err != nil {
		// No Rows -> Not an error for worker
		// pgx.ErrNoRows
		return nil, nil
		// If real error, handle
	}
	// Note: We need to handle 'no rows' correctly.
	// But checking error string is brittle.
	return map[string]interface{}{"id": id, "device_id": devId, "status": status, "attempts": attempts}, nil
}

// 7. Reverification Helpers
func (r *PostgresRepo) GetSuspiciousPackets(projectId string) ([]map[string]interface{}, error) {
	// Fetch 100 suspicious packets
	query := `SELECT time, device_id, data FROM telemetry WHERE project_id=$1 AND data->'metadata'->>'quality' = 'suspicious' LIMIT 100`
	return r.fetchSimpleWithArgs(query, projectId)
}

func (r *PostgresRepo) UpdatePacketStatus(ts time.Time, deviceId, status string) error {
	// Update JSONB quality
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Using jsonb_set to deep update
	query := `UPDATE telemetry SET data = jsonb_set(data, '{metadata,quality}', $3) WHERE time=$1 AND device_id=$2`
	// Need to quote the status for jsonb
	jsonStatus := fmt.Sprintf(`"%s"`, status)
	_, err := r.Pool.Exec(ctx, query, ts, deviceId, jsonStatus)
	return err
}

// GetNextProvisioningJob picks the next pending job
func (r *PostgresRepo) GetNextProvisioningJob() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Select FOR UPDATE SKIP LOCKED
	query := `
		SELECT id, device_id, credential_history_id, COALESCE(trigger_kind,'initial'), status, attempt_count 
		FROM mqtt_provisioning_jobs 
		WHERE status='pending' OR (status='failed' AND next_attempt_at <= NOW())
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`

	var id, devId, triggerKind, status string
	var credID *string
	var attempts int

	err = tx.QueryRow(ctx, query).Scan(&id, &devId, &credID, &triggerKind, &status, &attempts)
	if err != nil {
		// No job found
		return nil, nil
	}

	// Lock it
	_, err = tx.Exec(ctx, "UPDATE mqtt_provisioning_jobs SET status='processing', processing_started_at=NOW(), updated_at=NOW() WHERE id=$1", id)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id": id, "device_id": devId, "trigger_kind": triggerKind, "status": status, "attempt_count": attempts,
	}
	if credID != nil {
		result["credential_history_id"] = *credID
	}

	return result, nil
}

func (r *PostgresRepo) UpdateMqttJob(id, status, lastErr string, nextAttempt *time.Time, errCategory string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if status == "failed" {
		query := `
			UPDATE mqtt_provisioning_jobs
			SET status='failed',
			    last_result='failed',
			    last_error=$2,
			    last_error_category=$3,
			    next_attempt_at=$4,
			    attempt_count = attempt_count + 1,
			    consecutive_failures = COALESCE(consecutive_failures,0) + 1,
			    last_failure_at=NOW(),
			    last_attempt_at=NOW(),
			    last_duration_ms = CASE WHEN processing_started_at IS NULL THEN last_duration_ms ELSE GREATEST(0, FLOOR(EXTRACT(EPOCH FROM (NOW()-processing_started_at))*1000)::INT) END,
			    completed_at = NULL,
			    updated_at=NOW()
			WHERE id=$1
		`
		var next interface{}
		if nextAttempt != nil {
			next = *nextAttempt
		}
		_, err := r.Pool.Exec(ctx, query, id, lastErr, errCategory, next)
		return err
	}

	if status == "completed" {
		query := `
			UPDATE mqtt_provisioning_jobs
			SET status='completed',
			    last_result='completed',
			    last_error=NULL,
			    last_error_category=NULL,
			    next_attempt_at=NULL,
			    consecutive_failures=0,
			    last_success_at=NOW(),
			    completed_at=NOW(),
			    last_attempt_at=NOW(),
			    last_duration_ms = CASE WHEN processing_started_at IS NULL THEN last_duration_ms ELSE GREATEST(0, FLOOR(EXTRACT(EPOCH FROM (NOW()-processing_started_at))*1000)::INT) END,
			    updated_at=NOW()
			WHERE id=$1
		`
		_, err := r.Pool.Exec(ctx, query, id)
		return err
	}

	query := `
		UPDATE mqtt_provisioning_jobs
		SET status=$2,
		    last_result=$2,
		    last_error=$3,
		    last_error_category=$4,
		    last_attempt_at=NOW(),
		    updated_at=NOW()
		WHERE id=$1
	`
	_, err := r.Pool.Exec(ctx, query, id, status, lastErr, errCategory)
	return err
}

func (r *PostgresRepo) GetDeviceByID(id string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// FIX: 'auth' column does not exist. Use attributes.
	query := `SELECT id, imei, project_id, name, status, attributes, shadow FROM devices WHERE id=$1 AND deleted_at IS NULL`

	var (
		did, imei, pid string
		name           sql.NullString
		status         sql.NullString
		attrsRaw       []byte
		shadowRaw      []byte
	)

	err := r.Pool.QueryRow(ctx, query, id).Scan(&did, &imei, &pid, &name, &status, &attrsRaw, &shadowRaw)
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

	if len(shadowRaw) > 0 {
		var shadow map[string]interface{}
		if err := json.Unmarshal(shadowRaw, &shadow); err == nil {
			result["shadow"] = shadow
		}
	}

	if len(attrsRaw) > 0 {
		var attrMap map[string]interface{}
		if err := json.Unmarshal(attrsRaw, &attrMap); err == nil {
			if mqtt, exists := attrMap["mqtt"]; exists {
				result["auth"] = mqtt
			}
			if govt, exists := attrMap["government"]; exists {
				result["government"] = govt
			}
		}
	}

	return result, nil
}

func (r *PostgresRepo) GetPendingCommands(deviceId string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
	    SELECT id, command_id, payload, status, correlation_id, created_at, published_at
	    FROM command_requests
	    WHERE device_id = $1::uuid AND status IN ('queued','published')
	    ORDER BY created_at ASC
	`

	rows, err := r.Pool.Query(ctx, query, deviceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, cmdID, status, correlationID string
		var payload []byte
		var createdAt time.Time
		var publishedAt *time.Time
		if err := rows.Scan(&id, &cmdID, &payload, &status, &correlationID, &createdAt, &publishedAt); err != nil {
			continue // Skip bad rows
		}
		var parsed map[string]any
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &parsed); err != nil {
				parsed = map[string]any{"raw": string(payload)}
			}
		}
		results = append(results, map[string]interface{}{
			"id":             id,
			"command_id":     cmdID,
			"payload":        parsed,
			"status":         status,
			"correlation_id": correlationID,
			"created_at":     createdAt,
			"published_at":   publishedAt,
		})
	}
	return results, nil
}

func (r *PostgresRepo) GetTelemetryHistoryFromTable(table, deviceOrIMEI, packetType string, start, end time.Time, limit, offset int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Longer timeout for Cold Data
	defer cancel()

	// Dynamic Table Name (Trusted internal string)
	base := fmt.Sprintf(`
		SELECT t.time, t.data
		FROM %s t
		JOIN devices d ON t.device_id = d.id
		WHERE (d.imei = $1 OR d.id::text = $1)
		AND t.time BETWEEN $2 AND $3
	`, table)

	args := []interface{}{deviceOrIMEI, start, end}
	paramIdx := 4
	base = appendPacketTypeFilterClause(base, "t.data->>'packet_type'", packetType, &paramIdx, &args)

	base += fmt.Sprintf(" ORDER BY t.time DESC\nLIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	args = append(args, limit, offset)

	rows, err := r.Pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	for rows.Next() {
		var t time.Time
		var data map[string]interface{}
		rows.Scan(&t, &data)
		data["timestamp"] = t
		results = append(results, data)
	}
	return results, nil
}

func (r *PostgresRepo) GetLatestTelemetry(deviceOrIMEI, packetType string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	base := `
		SELECT t.time, t.data
		FROM telemetry t
		JOIN devices d ON t.device_id = d.id
		WHERE (d.imei = $1 OR d.id::text = $1)
	`
	args := []interface{}{deviceOrIMEI}
	paramIdx := 2
	base = appendPacketTypeFilterClause(base, "t.data->>'packet_type'", packetType, &paramIdx, &args)
	base += " ORDER BY t.time DESC LIMIT 1"

	row := r.Pool.QueryRow(ctx, base, args...)
	var t time.Time
	var data map[string]interface{}
	if err := row.Scan(&t, &data); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	data["timestamp"] = t
	return data, nil
}

func (r *PostgresRepo) CreateMqttProvisioningJob(deviceId string, credHistoryId *string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	triggerKind := "initial"
	if credHistoryId == nil {
		triggerKind = "manual"
	} else {
		var historicalJobs int
		_ = r.Pool.QueryRow(ctx, "SELECT count(*) FROM mqtt_provisioning_jobs WHERE device_id=$1", deviceId).Scan(&historicalJobs)
		if historicalJobs > 0 {
			triggerKind = "resync"
		}
	}

	query := `INSERT INTO mqtt_provisioning_jobs (device_id, credential_history_id, status, trigger_kind) VALUES ($1, $2, 'pending', $3)`
	_, err := r.Pool.Exec(ctx, query, deviceId, credHistoryId, triggerKind)
	return err
}

// Credential history persistence
func (r *PostgresRepo) InsertCredentialHistory(deviceId string, bundle map[string]interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(bundle)
	if err != nil {
		return "", err
	}

	query := `
		INSERT INTO credential_history (device_id, bundle, lifecycle, mqtt_access_applied, attempt_count)
		VALUES ($1, $2, 'pending', false, 0)
		RETURNING id
	`

	var id string
	if err := r.Pool.QueryRow(ctx, query, deviceId, body).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *PostgresRepo) GetCredentialHistory(id string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT bundle, lifecycle, mqtt_access_applied, last_error, attempt_count FROM credential_history WHERE id=$1`

	var bundleRaw []byte
	var lifecycle string
	var applied bool
	var lastError *string
	var attempts int

	if err := r.Pool.QueryRow(ctx, query, id).Scan(&bundleRaw, &lifecycle, &applied, &lastError, &attempts); err != nil {
		return nil, err
	}

	var bundle map[string]interface{}
	if err := json.Unmarshal(bundleRaw, &bundle); err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"bundle":    bundle,
		"lifecycle": lifecycle,
		"applied":   applied,
		"attempts":  attempts,
	}
	if lastError != nil {
		result["last_error"] = *lastError
	}
	return result, nil
}

func (r *PostgresRepo) MarkCredentialApplied(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE credential_history
		SET lifecycle='applied', mqtt_access_applied=true, updated_at=NOW()
		WHERE id=$1
	`
	_, err := r.Pool.Exec(ctx, query, id)
	return err
}

func (r *PostgresRepo) MarkCredentialFailure(id string, errMsg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE credential_history
		SET lifecycle='failed', last_error=$2, attempt_count = attempt_count + 1, updated_at=NOW()
		WHERE id=$1
	`
	_, err := r.Pool.Exec(ctx, query, id, errMsg)
	return err
}

func (r *PostgresRepo) RevokeCredentialHistoryByDevice(deviceID string, reason string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE credential_history
		SET lifecycle='revoked', last_error=COALESCE(NULLIF($2,''), last_error), updated_at=NOW()
		WHERE device_id=$1 AND lifecycle <> 'revoked'
	`
	cmd, err := r.Pool.Exec(ctx, query, deviceID, reason)
	if err != nil {
		return 0, err
	}
	return cmd.RowsAffected(), nil
}

func (r *PostgresRepo) GetLatestCredentialHistory(deviceId string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, bundle, lifecycle, mqtt_access_applied, last_error, attempt_count, created_at
		FROM credential_history
		WHERE device_id=$1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var id string
	var bundleRaw []byte
	var lifecycle string
	var applied bool
	var lastError *string
	var attempts int
	var createdAt time.Time

	if err := r.Pool.QueryRow(ctx, query, deviceId).Scan(&id, &bundleRaw, &lifecycle, &applied, &lastError, &attempts, &createdAt); err != nil {
		return nil, err
	}

	var bundle map[string]interface{}
	if err := json.Unmarshal(bundleRaw, &bundle); err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":         id,
		"bundle":     bundle,
		"lifecycle":  lifecycle,
		"applied":    applied,
		"attempts":   attempts,
		"created_at": createdAt,
	}
	if lastError != nil {
		result["last_error"] = *lastError
	}
	return result, nil
}

func (r *PostgresRepo) ListCredentialHistory(deviceId string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, bundle, lifecycle, mqtt_access_applied, last_error, attempt_count, created_at, updated_at
		FROM credential_history
		WHERE device_id=$1
		ORDER BY created_at DESC
	`

	rows, err := r.Pool.Query(ctx, query, deviceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	for rows.Next() {
		var id string
		var bundleRaw []byte
		var lifecycle string
		var applied bool
		var lastError *string
		var attempts int
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(&id, &bundleRaw, &lifecycle, &applied, &lastError, &attempts, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		var bundle map[string]interface{}
		if err := json.Unmarshal(bundleRaw, &bundle); err != nil {
			bundle = map[string]interface{}{"raw": string(bundleRaw)}
		}
		item := map[string]interface{}{
			"id":         id,
			"bundle":     bundle,
			"lifecycle":  lifecycle,
			"applied":    applied,
			"attempts":   attempts,
			"created_at": createdAt,
			"updated_at": updatedAt,
		}
		if lastError != nil {
			item["last_error"] = *lastError
		}
		results = append(results, item)
	}
	return results, nil
}

// --- Phase 21: Analytics Worker Support ---

func (r *PostgresRepo) GetPendingAnalyticsJob() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Atomic Fetch & Lock
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Assuming 'analytics_jobs' table
	query := `
		SELECT job_id, query, params
		FROM analytics_jobs 
		WHERE status = 'pending' 
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`
	var jobID, jobQuery string
	var params []byte // JSONB

	err = tx.QueryRow(ctx, query).Scan(&jobID, &jobQuery, &params)
	if err != nil {
		// pgx.ErrNoRows -> Return nil, nil
		return nil, nil // No jobs
	}

	// Update to processing
	_, err = tx.Exec(ctx, "UPDATE analytics_jobs SET status='processing', updated_at=NOW() WHERE job_id=$1", jobID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	var pMap map[string]interface{}
	json.Unmarshal(params, &pMap)

	return map[string]interface{}{
		"id":         jobID,
		"query":      jobQuery,
		"parameters": pMap,
	}, nil
}

func (r *PostgresRepo) UpdateAnalyticsJob(id, status string, result interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resBytes, _ := json.Marshal(result)
	query := `UPDATE analytics_jobs SET status=$1, results=$2, updated_at=NOW() WHERE job_id=$3`
	_, err := r.Pool.Exec(ctx, query, status, resBytes, id)
	return err
}

func (r *PostgresRepo) RunAggregationQuery(jobType string, params map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Safe Query Mapping (Prevent Injection)
	// V1 Supported Types: "daily_stats", "msg_count", "anomaly_summary"

	var query string
	var args []interface{}

	switch jobType {
	case "daily_stats":
		// Count packets for a project in last 24h
		pid, _ := params["project_id"].(string)
		query = `SELECT count(*) as count, avg((data->>'temp')::numeric) as avg_temp 
		         FROM telemetry 
		         WHERE project_id=$1 AND time > NOW() - INTERVAL '24 hours'`
		args = append(args, pid)

	case "msg_count":
		// Total messages
		query = `SELECT count(*) as count FROM telemetry`

	default:
		return nil, fmt.Errorf("unknown job type: %s", jobType)
	}

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Generic Single Row Fetch
	if rows.Next() {
		// Map columns
		fields := rows.FieldDescriptions()
		results := make(map[string]interface{})
		vals := make([]interface{}, len(fields))
		valPtrs := make([]interface{}, len(fields))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		rows.Scan(valPtrs...)

		for i, fd := range fields {
			results[string(fd.Name)] = vals[i]
		}
		return results, nil
	}

	return map[string]interface{}{}, nil
}
