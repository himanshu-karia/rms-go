-- new-db-schema/sql/verticals/agri_advisory.sql
-- Agriculture advisory rules bundle.

CREATE TABLE IF NOT EXISTS soil_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id VARCHAR(50),
    crop VARCHAR(50),
    parameter VARCHAR(50),
    operator VARCHAR(10),
    threshold FLOAT,
    threshold_max FLOAT,
    advisory TEXT,
    severity VARCHAR(20)
);
