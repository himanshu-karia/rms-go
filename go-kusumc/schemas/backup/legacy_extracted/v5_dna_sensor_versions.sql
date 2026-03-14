-- Project DNA sensor CSV versions
CREATE TABLE IF NOT EXISTS dna_sensor_versions (
    id BIGSERIAL PRIMARY KEY,
    project_id TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT 'draft',
    status TEXT NOT NULL DEFAULT 'draft',
    imported_count INTEGER NOT NULL DEFAULT 0,
    csv_data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    rolled_back_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dna_sensor_versions_project_created
    ON dna_sensor_versions (project_id, created_at DESC);
