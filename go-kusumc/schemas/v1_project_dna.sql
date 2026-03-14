-- Project DNA Schema - provides canonical storage for payload definitions and thresholds
CREATE TABLE IF NOT EXISTS project_dna (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    payload_rows JSONB NOT NULL DEFAULT '[]'::jsonb,
    edge_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    virtual_sensors JSONB NOT NULL DEFAULT '[]'::jsonb,
    automation_flows JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_dna_updated_at ON project_dna (updated_at);

-- Sensors (canonical payload definitions)
CREATE TABLE IF NOT EXISTS payload_sensors (
    project_id       TEXT        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    param            TEXT        NOT NULL,
    label            TEXT        NOT NULL,
    unit             TEXT        NULL,
    min_value        DOUBLE PRECISION NULL,
    max_value        DOUBLE PRECISION NULL,
    resolution       DOUBLE PRECISION NULL,
    required         BOOLEAN     NOT NULL DEFAULT FALSE,
    notes            TEXT        NULL,
    topic_template   TEXT        NULL,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, param)
);

-- Thresholds (project defaults + optional per-device overrides)
CREATE TABLE IF NOT EXISTS telemetry_thresholds (
    project_id  TEXT        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    param       TEXT        NOT NULL,
    scope       TEXT        NOT NULL CHECK (scope IN ('project','device')),
    device_id   TEXT        NOT NULL DEFAULT '',
    min_value   DOUBLE PRECISION NULL,
    max_value   DOUBLE PRECISION NULL,
    target      DOUBLE PRECISION NULL,
    unit        TEXT NULL,
    decimal_places INT NULL,
    template_id TEXT NULL,
    metadata    JSONB DEFAULT '{}'::jsonb,
    reason      TEXT NULL,
    updated_by  TEXT NULL,
    warn_low    DOUBLE PRECISION NULL,
    warn_high   DOUBLE PRECISION NULL,
    alert_low   DOUBLE PRECISION NULL,
    alert_high  DOUBLE PRECISION NULL,
    origin      TEXT        NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, param, scope, device_id),
    CHECK ((scope = 'project' AND device_id = '') OR (scope = 'device' AND device_id <> ''))
);

CREATE INDEX IF NOT EXISTS idx_payload_sensors_project ON payload_sensors(project_id);
CREATE INDEX IF NOT EXISTS idx_thresholds_project_param ON telemetry_thresholds(project_id, param);
CREATE INDEX IF NOT EXISTS idx_thresholds_device ON telemetry_thresholds(project_id, device_id) WHERE scope = 'device';
