package secondary

import (
	"context"
	"encoding/json"
	"time"
)

// --- Alerts ---

func (r *PostgresRepo) CreateAlert(deviceId, projectId, msg, severity string) error {
	return r.CreateAlertWithData(deviceId, projectId, msg, severity, nil)
}

func (r *PostgresRepo) CreateAlertWithData(deviceId, projectId, msg, severity string, data interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        INSERT INTO alerts (device_id, project_id, message, severity, data, status, triggered_at)
        VALUES ($1, $2, $3, $4, $5, 'active', NOW())
    `

	var jsonb []byte
	if data != nil {
		jsonb, _ = json.Marshal(data)
	}
	_, err := r.Pool.Exec(ctx, query, deviceId, projectId, msg, severity, jsonb)
	return err
}

// --- Command Logs ---

// LogCommand moved to postgres_repo.go with richer signature
