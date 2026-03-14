-- Draft DDL for Project DNA canonical store (sensors + thresholds)
-- Ensure param is the single identifier across ingestion, transforms, rules, and UI

-- Sensors
CREATE TABLE IF NOT EXISTS payload_sensors (
    project_id       TEXT        NOT NULL,
    param            TEXT        NOT NULL, -- canonical key used everywhere
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

-- Thresholds (project default + optional device override)
CREATE TABLE IF NOT EXISTS telemetry_thresholds (
    project_id  TEXT        NOT NULL,
    param       TEXT        NOT NULL,
    scope       TEXT        NOT NULL CHECK (scope IN ('project','device')),
    device_id   TEXT        NULL, -- required when scope = 'device'
    warn_low    DOUBLE PRECISION NULL,
    warn_high   DOUBLE PRECISION NULL,
    alert_low   DOUBLE PRECISION NULL,
    alert_high  DOUBLE PRECISION NULL,
    origin      TEXT        NULL, -- e.g., 'protocol-default', 'override-api', 'firmware'
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, param, scope, COALESCE(device_id, ''))
);

-- Helpful indexes for lookup
CREATE INDEX IF NOT EXISTS idx_payload_sensors_project ON payload_sensors(project_id);
CREATE INDEX IF NOT EXISTS idx_thresholds_project_param ON telemetry_thresholds(project_id, param);
CREATE INDEX IF NOT EXISTS idx_thresholds_device ON telemetry_thresholds(project_id, device_id) WHERE scope = 'device';
