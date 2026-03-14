-- new-db-schema/sql/verticals/gis.sql
-- GIS bundle.
-- Depends on: core.sql (projects)

CREATE TABLE IF NOT EXISTS gis_layers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT,
    geojson JSONB NOT NULL,
    style JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
