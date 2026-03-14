-- new-db-schema/sql/verticals/master_data.sql
-- Generic master-data bundle (lookup tables).

CREATE TABLE IF NOT EXISTS master_data (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    project_id TEXT,
    is_active BOOLEAN DEFAULT true,
    UNIQUE(type, code, project_id)
);
