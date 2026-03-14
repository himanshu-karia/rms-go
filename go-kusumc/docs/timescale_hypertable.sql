-- Timescale hypertable for unified RMS telemetry
-- Run in Postgres/TimescaleDB

CREATE TABLE IF NOT EXISTS telemetry (
    time        TIMESTAMPTZ       NOT NULL,
    project_id  TEXT              NOT NULL,
    device_id   TEXT              NOT NULL,
    imei        TEXT              NOT NULL,
    packet_type TEXT              NOT NULL,
    msg_id      TEXT,
    protocol_id TEXT,
    contractor_id TEXT,
    supplier_id TEXT,
    manufacturer_id TEXT,
    payload     JSONB             NOT NULL
);

SELECT create_hypertable('telemetry', 'time', if_not_exists => TRUE);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_tel_proj_type_time ON telemetry (project_id, packet_type, time DESC);
CREATE INDEX IF NOT EXISTS idx_tel_device_time ON telemetry (device_id, time DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tel_msg_id ON telemetry (msg_id) WHERE msg_id IS NOT NULL;

-- Optional partial indexes if hot per packet_type
-- CREATE INDEX IF NOT EXISTS idx_tel_pump_time ON telemetry (time DESC) WHERE packet_type = 'pump';

-- Retention (example 90 days)
-- SELECT add_retention_policy('telemetry', INTERVAL '90 days');
