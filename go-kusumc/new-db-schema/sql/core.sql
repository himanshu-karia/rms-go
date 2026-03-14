-- new-db-schema/sql/core.sql
-- Core schema (platform fundamentals). Intended to be applied before any vertical bundle.

-- Extensions
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Organizations (Hierarchy)
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    path ltree,
    parent_id UUID REFERENCES organizations(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS path_idx ON organizations USING GIST (path);

-- Link Roles (Governance)
CREATE TABLE IF NOT EXISTS link_roles (
    slug TEXT PRIMARY KEY,
    description TEXT,
    is_unique_per_device BOOLEAN DEFAULT false
);

INSERT INTO link_roles (slug, description, is_unique_per_device) VALUES 
('owner', 'Legal Owner of the Asset', true),
('maintainer', 'Technical Support / Field Ops', false),
('auditor', 'Third Party Auditor', false),
('manufacturer', 'Hardware OEM', true)
ON CONFLICT (slug) DO NOTHING;

-- Users (Auth)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer',
    org_id UUID REFERENCES organizations(id),
    allowed_projects JSONB DEFAULT '[]',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- API Keys (Machine-to-Machine Auth)
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    prefix TEXT NOT NULL,
    project_id TEXT,
    scopes JSONB DEFAULT '[]',
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID
);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);

-- Projects (The DNA)
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT,
    location TEXT,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

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

CREATE TABLE IF NOT EXISTS telemetry_thresholds (
    project_id  TEXT        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    param       TEXT        NOT NULL,
    scope       TEXT        NOT NULL CHECK (scope IN ('project','device')),
    device_id   TEXT        NOT NULL DEFAULT '',
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

-- Project DNA sensor CSV version snapshots
CREATE TABLE IF NOT EXISTS dna_sensor_versions (
    id BIGSERIAL PRIMARY KEY,
    project_id TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT 'draft',
    status TEXT NOT NULL DEFAULT 'draft',
    imported_count INTEGER NOT NULL DEFAULT 0,
    csv_data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    rolled_back_at TIMESTAMPTZ,
    created_by TEXT,
    published_by TEXT,
    rolled_back_by TEXT
);
CREATE INDEX IF NOT EXISTS idx_dna_sensor_versions_project_created
    ON dna_sensor_versions (project_id, created_at DESC);

-- Devices (Assets)
CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    imei TEXT UNIQUE NOT NULL,
    project_id TEXT REFERENCES projects(id),
    name TEXT,
    status TEXT,
    attributes JSONB DEFAULT '{}',
    shadow JSONB DEFAULT '{}',
    last_seen TIMESTAMPTZ
);

-- Device Links (Multi-party graph)
CREATE TABLE IF NOT EXISTS device_links (
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    org_id UUID REFERENCES organizations(id),
    role TEXT REFERENCES link_roles(slug),
    PRIMARY KEY (device_id, org_id, role)
);

-- Telemetry (Timescale hypertable)
CREATE TABLE IF NOT EXISTS telemetry (
    time TIMESTAMPTZ NOT NULL,
    device_id UUID NOT NULL,
    project_id TEXT NOT NULL,
    data JSONB NOT NULL,
    type TEXT DEFAULT 'data',
    status TEXT DEFAULT 'verified',
    hops INTEGER DEFAULT 0
);

SELECT create_hypertable('telemetry', 'time', if_not_exists => TRUE);

ALTER TABLE telemetry SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'device_id'
);
SELECT add_compression_policy('telemetry', INTERVAL '7 days', if_not_exists => TRUE);

-- Platform services (cross-vertical)
CREATE TABLE IF NOT EXISTS anomalies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL,
    type TEXT NOT NULL,
    field TEXT NOT NULL,
    value DOUBLE PRECISION,
    severity TEXT,
    description TEXT,
    detected_at TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS analytics_jobs (
    job_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id TEXT,
    query TEXT,
    params JSONB,
    status TEXT DEFAULT 'pending',
    results JSONB,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL,
    device_id TEXT,
    name TEXT NOT NULL,
    trigger JSONB NOT NULL,
    actions JSONB NOT NULL,
    enabled BOOLEAN DEFAULT true,
    execution_location TEXT DEFAULT 'cloud',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    cron_expression TEXT,
    time TIMESTAMPTZ,
    command JSONB NOT NULL,
    is_active BOOLEAN DEFAULT true,
    last_run TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_schedules_active ON schedules(is_active);
CREATE INDEX IF NOT EXISTS idx_schedules_time ON schedules(time);

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID,
    project_id TEXT,
    message TEXT,
    severity TEXT,
    status TEXT,
    triggered_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS command_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID,
    command TEXT,
    payload JSONB,
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS ota_campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    project_type TEXT,
    version TEXT NOT NULL,
    url TEXT NOT NULL,
    checksum TEXT,
    status TEXT DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS automation_flows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT UNIQUE NOT NULL,
    version TEXT DEFAULT '1.0.0',
    nodes JSONB NOT NULL,
    edges JSONB NOT NULL,
    deployed_at TIMESTAMPTZ,
    deployed_by TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS device_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    manufacturer TEXT,
    protocol TEXT NOT NULL,
    registers JSONB,
    decoder_script TEXT,
    lora_config JSONB,
    comm_settings JSONB,
    version INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Connectivity (protocol profiles + govt creds)
CREATE TABLE IF NOT EXISTS protocols (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    server_vendor_org_id UUID NULL REFERENCES organizations(id) ON DELETE SET NULL,
    kind TEXT NOT NULL CHECK (kind IN ('primary','govt')),
    protocol TEXT NOT NULL CHECK (protocol IN ('mqtt','mqtts','https')),
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    publish_topics JSONB NOT NULL DEFAULT '[]',
    subscribe_topics JSONB NOT NULL DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, kind, host, port)
);
CREATE INDEX IF NOT EXISTS idx_protocols_project ON protocols(project_id);
CREATE INDEX IF NOT EXISTS idx_protocols_vendor ON protocols(server_vendor_org_id);

CREATE TABLE IF NOT EXISTS device_govt_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    protocol_id UUID NOT NULL REFERENCES protocols(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL,
    username TEXT NOT NULL,
    password_enc TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(device_id, protocol_id)
);
CREATE INDEX IF NOT EXISTS idx_device_govt_credentials_device ON device_govt_credentials(device_id);
CREATE INDEX IF NOT EXISTS idx_device_govt_credentials_protocol ON device_govt_credentials(protocol_id);

-- Provisioning jobs + credential history
CREATE TABLE IF NOT EXISTS mqtt_provisioning_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'pending',
    attempt_count INT DEFAULT 0,
    next_attempt_at TIMESTAMPTZ DEFAULT NOW(),
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_mqtt_jobs_status_next ON mqtt_provisioning_jobs (status, next_attempt_at);

CREATE TABLE IF NOT EXISTS credential_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    bundle JSONB NOT NULL,
    lifecycle TEXT NOT NULL DEFAULT 'pending',
    mqtt_access_applied BOOLEAN NOT NULL DEFAULT false,
    last_error TEXT,
    attempt_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cred_hist_device ON credential_history(device_id);

ALTER TABLE mqtt_provisioning_jobs
    ADD COLUMN IF NOT EXISTS credential_history_id UUID REFERENCES credential_history(id);
CREATE INDEX IF NOT EXISTS idx_mqtt_jobs_cred_id ON mqtt_provisioning_jobs(credential_history_id);
