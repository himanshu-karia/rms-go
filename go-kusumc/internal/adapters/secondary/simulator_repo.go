package secondary

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type SimulatorSessionRecord struct {
	ID                 string
	Token              string
	Status             string
	DeviceID           *string
	DeviceUUID         *string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	EndedAt            *time.Time
	RequestedBy        *string
	RevokedBy          *string
	LastActivityAt     time.Time
	CredentialSnapshot map[string]interface{}
	CommandQuota       map[string]interface{}
}

func (r *PostgresRepo) CreateSimulatorSession(record SimulatorSessionRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	credRaw, err := json.Marshal(record.CredentialSnapshot)
	if err != nil {
		return err
	}
	quotaRaw, err := json.Marshal(record.CommandQuota)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO simulator_sessions (
			id, token, status, device_id, device_uuid, created_at, expires_at, ended_at,
			requested_by, revoked_by, last_activity_at, credential_snapshot, command_quota
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13
		)
	`

	_, err = r.Pool.Exec(ctx, query,
		record.ID,
		record.Token,
		record.Status,
		record.DeviceID,
		record.DeviceUUID,
		record.CreatedAt,
		record.ExpiresAt,
		record.EndedAt,
		record.RequestedBy,
		record.RevokedBy,
		record.LastActivityAt,
		credRaw,
		quotaRaw,
	)
	return err
}

func (r *PostgresRepo) ListSimulatorSessions(limit int, cursor string, status string) ([]SimulatorSessionRecord, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	var cursorTime *time.Time
	if cursor != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, cursor); err == nil {
			cursorTime = &parsed
		}
	}

	query := `
		SELECT id, token, status, device_id, device_uuid, created_at, expires_at, ended_at,
		       requested_by, revoked_by, last_activity_at, credential_snapshot, command_quota
		FROM simulator_sessions
		WHERE ($1 = '' OR status = $1)
		  AND ($2::timestamptz IS NULL OR created_at < $2)
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.Pool.Query(ctx, query, status, cursorTime, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	records := make([]SimulatorSessionRecord, 0, limit)
	for rows.Next() {
		var rec SimulatorSessionRecord
		var credRaw []byte
		var quotaRaw []byte

		if err := rows.Scan(
			&rec.ID,
			&rec.Token,
			&rec.Status,
			&rec.DeviceID,
			&rec.DeviceUUID,
			&rec.CreatedAt,
			&rec.ExpiresAt,
			&rec.EndedAt,
			&rec.RequestedBy,
			&rec.RevokedBy,
			&rec.LastActivityAt,
			&credRaw,
			&quotaRaw,
		); err != nil {
			return nil, "", err
		}

		_ = json.Unmarshal(credRaw, &rec.CredentialSnapshot)
		_ = json.Unmarshal(quotaRaw, &rec.CommandQuota)

		records = append(records, rec)
	}

	nextCursor := ""
	if len(records) > limit {
		nextCursor = records[limit-1].CreatedAt.Format(time.RFC3339Nano)
		records = records[:limit]
	}

	return records, nextCursor, nil
}

func (r *PostgresRepo) GetSimulatorSessionByID(sessionID string) (*SimulatorSessionRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, token, status, device_id, device_uuid, created_at, expires_at, ended_at,
		       requested_by, revoked_by, last_activity_at, credential_snapshot, command_quota
		FROM simulator_sessions
		WHERE id = $1
	`

	var rec SimulatorSessionRecord
	var credRaw []byte
	var quotaRaw []byte

	if err := r.Pool.QueryRow(ctx, query, sessionID).Scan(
		&rec.ID,
		&rec.Token,
		&rec.Status,
		&rec.DeviceID,
		&rec.DeviceUUID,
		&rec.CreatedAt,
		&rec.ExpiresAt,
		&rec.EndedAt,
		&rec.RequestedBy,
		&rec.RevokedBy,
		&rec.LastActivityAt,
		&credRaw,
		&quotaRaw,
	); err != nil {
		return nil, err
	}

	_ = json.Unmarshal(credRaw, &rec.CredentialSnapshot)
	_ = json.Unmarshal(quotaRaw, &rec.CommandQuota)

	return &rec, nil
}

func (r *PostgresRepo) RevokeSimulatorSession(sessionID string, revokedBy *string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE simulator_sessions
		SET status='revoked', ended_at=NOW(), last_activity_at=NOW(), revoked_by=$2
		WHERE id=$1
	`

	cmd, err := r.Pool.Exec(ctx, query, sessionID, revokedBy)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("simulator session not found")
	}
	return nil
}

func (r *PostgresRepo) GetDeviceByIDOrUUID(idOrUUID string) (map[string]interface{}, error) {
	if idOrUUID == "" {
		return nil, fmt.Errorf("device id required")
	}
	return r.GetDeviceByIDOrIMEI(idOrUUID)
}

func (r *PostgresRepo) GetCredentialHistoryByUsername(username string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT ch.id, ch.device_id, ch.bundle, ch.lifecycle, d.imei, d.status
		FROM credential_history ch
		JOIN devices d ON d.id = ch.device_id
		WHERE ch.bundle->>'username' = $1
		ORDER BY ch.created_at DESC
		LIMIT 1
	`

	var id string
	var deviceID string
	var bundleRaw []byte
	var lifecycle string
	var imei string
	var status *string

	if err := r.Pool.QueryRow(ctx, query, username).Scan(&id, &deviceID, &bundleRaw, &lifecycle, &imei, &status); err != nil {
		return nil, err
	}

	var bundle map[string]interface{}
	if err := json.Unmarshal(bundleRaw, &bundle); err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":        id,
		"device_id": deviceID,
		"bundle":    bundle,
		"lifecycle": lifecycle,
		"imei":      imei,
	}
	if status != nil {
		result["status"] = *status
	}
	return result, nil
}
