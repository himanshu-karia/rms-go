package secondary

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ingestion-go/internal/models"
)

// PostgresGovtCredsRepo stores per-device government credentials.
type PostgresGovtCredsRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresGovtCredsRepo(pool *pgxpool.Pool) *PostgresGovtCredsRepo {
	if pool == nil {
		return nil
	}
	return &PostgresGovtCredsRepo{pool: pool}
}

func (r *PostgresGovtCredsRepo) Upsert(ctx context.Context, b models.GovtCredentialBundle) error {
	if r == nil || r.pool == nil {
		return nil
	}

	_, err := r.pool.Exec(ctx, `
        INSERT INTO device_govt_credentials (id, device_id, protocol_id, client_id, username, password_enc, metadata)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        ON CONFLICT (device_id, protocol_id) DO UPDATE SET
            client_id = EXCLUDED.client_id,
            username = EXCLUDED.username,
            password_enc = EXCLUDED.password_enc,
            metadata = EXCLUDED.metadata,
            updated_at = NOW()
    `,
		b.ID, b.DeviceID, b.ProtocolID, b.ClientID, b.Username, b.Password, b.Metadata,
	)
	return err
}

func (r *PostgresGovtCredsRepo) BulkUpsert(ctx context.Context, bundles []models.GovtCredentialBundle) error {
	if r == nil || r.pool == nil {
		return nil
	}
	if len(bundles) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, b := range bundles {
		batch.Queue(`
            INSERT INTO device_govt_credentials (id, device_id, protocol_id, client_id, username, password_enc, metadata, created_at, updated_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
            ON CONFLICT (device_id, protocol_id) DO UPDATE SET
                client_id = EXCLUDED.client_id,
                username = EXCLUDED.username,
                password_enc = EXCLUDED.password_enc,
                metadata = EXCLUDED.metadata,
                updated_at = EXCLUDED.updated_at
        `, b.ID, b.DeviceID, b.ProtocolID, b.ClientID, b.Username, b.Password, b.Metadata, b.CreatedAt, b.UpdatedAt)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range bundles {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresGovtCredsRepo) GetByDevice(ctx context.Context, deviceID string) ([]models.GovtCredentialBundle, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
        SELECT id, device_id, protocol_id, client_id, username, password_enc, metadata, created_at, updated_at
        FROM device_govt_credentials
        WHERE device_id = $1
    `, deviceID)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []models.GovtCredentialBundle
	for rows.Next() {
		var rec models.GovtCredentialBundle
		if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.ProtocolID, &rec.ClientID, &rec.Username, &rec.Password, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresGovtCredsRepo) GetByDeviceAndProtocol(ctx context.Context, deviceID, protocolID string) (*models.GovtCredentialBundle, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}

	row := r.pool.QueryRow(ctx, `
        SELECT id, device_id, protocol_id, client_id, username, password_enc, metadata, created_at, updated_at
        FROM device_govt_credentials
        WHERE device_id = $1 AND protocol_id = $2
    `, deviceID, protocolID)

	var rec models.GovtCredentialBundle
	if err := row.Scan(&rec.ID, &rec.DeviceID, &rec.ProtocolID, &rec.ClientID, &rec.Username, &rec.Password, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func (r *PostgresGovtCredsRepo) Delete(ctx context.Context, id string) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM device_govt_credentials WHERE id = $1`, id)
	return err
}
