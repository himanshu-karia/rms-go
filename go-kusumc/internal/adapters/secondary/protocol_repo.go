package secondary

import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
    "github.com/jackc/pgx/v5/pgxpool"

    "ingestion-go/internal/models"
)

// PostgresProtocolRepo provides CRUD for protocol profiles.
type PostgresProtocolRepo struct {
    pool *pgxpool.Pool
}

func NewPostgresProtocolRepo(pool *pgxpool.Pool) *PostgresProtocolRepo {
    if pool == nil {
        return nil
    }
    return &PostgresProtocolRepo{pool: pool}
}

func (r *PostgresProtocolRepo) Upsert(ctx context.Context, p models.ProtocolProfile) error {
    if r == nil || r.pool == nil {
        return nil
    }

    _, err := r.pool.Exec(ctx, `
        INSERT INTO protocols (id, project_id, server_vendor_org_id, kind, protocol, host, port, publish_topics, subscribe_topics, metadata)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT (id) DO UPDATE SET
            project_id = EXCLUDED.project_id,
            server_vendor_org_id = EXCLUDED.server_vendor_org_id,
            kind = EXCLUDED.kind,
            protocol = EXCLUDED.protocol,
            host = EXCLUDED.host,
            port = EXCLUDED.port,
            publish_topics = EXCLUDED.publish_topics,
            subscribe_topics = EXCLUDED.subscribe_topics,
            metadata = EXCLUDED.metadata,
            updated_at = NOW()
    `,
        p.ID, p.ProjectID, nullableUUID(p.ServerVendor), p.Kind, p.Protocol, p.Host, p.Port,
        toJSONBArray(p.PublishTopics), toJSONBArray(p.SubscribeTopics), p.Metadata,
    )
    return err
}

func (r *PostgresProtocolRepo) GetByProject(ctx context.Context, projectID string) ([]models.ProtocolProfile, error) {
    if r == nil || r.pool == nil {
        return nil, nil
    }

    rows, err := r.pool.Query(ctx, `
        SELECT id, project_id, server_vendor_org_id, kind, protocol, host, port, publish_topics, subscribe_topics, metadata, created_at, updated_at
        FROM protocols
        WHERE project_id = $1
    `, projectID)
    if err != nil {
        if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
            return nil, nil
        }
        return nil, err
    }
    defer rows.Close()

    var out []models.ProtocolProfile
    for rows.Next() {
        var rec models.ProtocolProfile
        var serverVendor *string
        var pub, sub []string
        if err := rows.Scan(&rec.ID, &rec.ProjectID, &serverVendor, &rec.Kind, &rec.Protocol, &rec.Host, &rec.Port, &pub, &sub, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
            return nil, err
        }
        if serverVendor != nil {
            rec.ServerVendor = *serverVendor
        }
        rec.PublishTopics = pub
        rec.SubscribeTopics = sub
        out = append(out, rec)
    }

    if err := rows.Err(); err != nil {
        return nil, err
    }

    return out, nil
}

func (r *PostgresProtocolRepo) GetByID(ctx context.Context, id string) (*models.ProtocolProfile, error) {
    if r == nil || r.pool == nil {
        return nil, nil
    }

    row := r.pool.QueryRow(ctx, `
        SELECT id, project_id, server_vendor_org_id, kind, protocol, host, port, publish_topics, subscribe_topics, metadata, created_at, updated_at
        FROM protocols
        WHERE id = $1
    `, id)

    var rec models.ProtocolProfile
    var serverVendor *string
    var pub, sub []string
    if err := row.Scan(&rec.ID, &rec.ProjectID, &serverVendor, &rec.Kind, &rec.Protocol, &rec.Host, &rec.Port, &pub, &sub, &rec.Metadata, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, nil
        }
        if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
            return nil, nil
        }
        return nil, err
    }
    if serverVendor != nil {
        rec.ServerVendor = *serverVendor
    }
    rec.PublishTopics = pub
    rec.SubscribeTopics = sub
    return &rec, nil
}

func (r *PostgresProtocolRepo) Delete(ctx context.Context, id string) error {
    if r == nil || r.pool == nil {
        return nil
    }
    _, err := r.pool.Exec(ctx, `DELETE FROM protocols WHERE id = $1`, id)
    return err
}

// toJSONBArray is a helper for string slices.
func toJSONBArray(items []string) []string {
    if items == nil {
        return []string{}
    }
    return items
}

func nullableUUID(id string) interface{} {
    if id == "" {
        return nil
    }
    return id
}
