-- V1.1 Schema Initialization Script
-- Target: TimescaleDB (PostgreSQL 14+)

-- 1. Enable Extensions
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 2. Organizations (Hierarchy)
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- 'govt', 'private', 'vendor' (Dynamic)
    path ltree,         -- 'India.Maha.Pune' for fast hierarchy search
    parent_id UUID REFERENCES organizations(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB DEFAULT '{}',
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS path_idx ON organizations USING GIST (path);

-- 3. Link Roles (Governance)
CREATE TABLE IF NOT EXISTS link_roles (
    slug TEXT PRIMARY KEY,
    description TEXT,
    is_unique_per_device BOOLEAN DEFAULT false
);

-- Seed Default Roles
INSERT INTO link_roles (slug, description, is_unique_per_device) VALUES 
('owner', 'Legal Owner of the Asset', true),
('maintainer', 'Technical Support / Field Ops', false),
('auditor', 'Third Party Auditor', false),
('manufacturer', 'Hardware OEM', true)
ON CONFLICT (slug) DO NOTHING;

-- Project DNA sensor CSV version snapshots (added 2026-01-02)
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

-- 3b. Users (Auth)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE,
    display_name TEXT,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer',
    org_id UUID REFERENCES organizations(id),
    
    -- ACL: List of Project IDs this user can access. 
    -- If role='admin', this might be ignored (Super Admin).
    -- If role='operator', restricted to these.
    allowed_projects JSONB DEFAULT '[]', 
    
    active BOOLEAN DEFAULT true,
    must_rotate_password BOOLEAN DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Auth sessions (refresh token storage)
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    ip_address TEXT,
    user_agent TEXT
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_refresh_hash ON user_sessions(refresh_token_hash);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);

-- Mobile bridge baseline tables (added 2026-03-04)
CREATE TABLE IF NOT EXISTS mobile_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token_hash TEXT NOT NULL,
    refresh_token_hash TEXT NOT NULL,
    device_fingerprint TEXT NOT NULL,
    device_name TEXT,
    platform TEXT NOT NULL DEFAULT 'android',
    status TEXT NOT NULL DEFAULT 'active', -- active|revoked|expired
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mobile_sessions_token_hash ON mobile_sessions(session_token_hash);
CREATE INDEX IF NOT EXISTS idx_mobile_sessions_user_id ON mobile_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_mobile_sessions_status_expires ON mobile_sessions(status, expires_at);

CREATE TABLE IF NOT EXISTS mobile_assignments (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL,
    device_id TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_mobile_assignments_user_active ON mobile_assignments(user_id, active);
CREATE INDEX IF NOT EXISTS idx_mobile_assignments_scope ON mobile_assignments(project_id, device_id);

CREATE TABLE IF NOT EXISTS mobile_ingest_dedupe (
    id BIGSERIAL PRIMARY KEY,
    project_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT,
    status_code INTEGER,
    response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, device_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_mobile_ingest_dedupe_expires ON mobile_ingest_dedupe(expires_at);

-- Capability-based RBAC
CREATE TABLE IF NOT EXISTS capabilities (
    key TEXT PRIMARY KEY,
    description TEXT
);

CREATE TABLE IF NOT EXISTS user_capabilities (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    capability_key TEXT NOT NULL REFERENCES capabilities(key) ON DELETE CASCADE,
    PRIMARY KEY (user_id, capability_key)
);

-- User role bindings (scoped)
CREATE TABLE IF NOT EXISTS user_role_bindings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_key TEXT NOT NULL,
    role_type TEXT NOT NULL,
    scope JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_role_bindings_unique
    ON user_role_bindings (user_id, role_key, role_type, scope);
CREATE INDEX IF NOT EXISTS idx_user_role_bindings_user_id
    ON user_role_bindings (user_id);

-- User groups (RBAC scopes + memberships)
CREATE TABLE IF NOT EXISTS user_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    scope JSONB NOT NULL DEFAULT '{}'::jsonb,
    default_role_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_group_members (
    group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    added_by UUID REFERENCES users(id),
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

-- Org/Project role scaffolding (RBAC)
CREATE TABLE IF NOT EXISTS org_roles (
    slug TEXT PRIMARY KEY,
    description TEXT NOT NULL
);

INSERT INTO org_roles (slug, description) VALUES
    ('owner','Full org control'),
    ('admin','Org admin'),
    ('operator','Ops user'),
    ('viewer','Read-only')
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS project_roles (
    slug TEXT PRIMARY KEY,
    description TEXT NOT NULL
);

INSERT INTO project_roles (slug, description) VALUES
    ('maintainer','Project maintainer'),
    ('analyst','Read/analytics'),
    ('viewer','Read-only'),
    ('service','Machine key')
ON CONFLICT DO NOTHING;

-- 4. Projects (The DNA)
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY, -- 'health_01'
    name TEXT NOT NULL,
    -- Added columns to satisfy seeder inserts
    type TEXT,
    location TEXT,
    config JSONB NOT NULL DEFAULT '{}', -- Zero-Code Hardware Definition
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
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

-- 5. Devices (Assets)
CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    imei TEXT UNIQUE NOT NULL,
    project_id TEXT REFERENCES projects(id),
    
    -- Dynamic Attributes
    attributes JSONB DEFAULT '{}', -- Location, Firmware Ver, Install Date
    
    -- Shadow State (Last Known)
    shadow JSONB DEFAULT '{}',
    last_seen TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

-- Add missing device metadata columns for seeders
ALTER TABLE devices ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS status TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS connectivity_status TEXT DEFAULT 'unknown';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS connectivity_updated_at TIMESTAMPTZ;

-- 6. Device Links (The Multi-Party Graph)
CREATE TABLE IF NOT EXISTS device_links (
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    org_id UUID REFERENCES organizations(id),
    role TEXT REFERENCES link_roles(slug),
    
    PRIMARY KEY (device_id, org_id, role)
);

-- 7. Telemetry (The Firehose)
CREATE TABLE IF NOT EXISTS telemetry (
    time TIMESTAMPTZ NOT NULL,
    device_id UUID NOT NULL, -- No FK Constraint for speed? Or keep it? Timescale recs: Keep it simple.
    project_id TEXT NOT NULL,
    data JSONB NOT NULL,
    type TEXT DEFAULT 'data'
);

-- 7a. Audit Logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id TEXT,
    action TEXT,
    resource TEXT,
    ip TEXT,
    status TEXT,
    metadata JSONB
);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_logs(ts DESC);

-- 7b. Notification Queue (Workflows)
CREATE TABLE IF NOT EXISTS notification_queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL,
    device_uuid TEXT NOT NULL,
    channel TEXT NOT NULL, -- email | webhook | queue
    target TEXT NOT NULL,
    template_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending', -- pending | sent | failed
    triggered_by TEXT NOT NULL DEFAULT 'offline-monitor',
    payload JSONB NOT NULL DEFAULT '{}',
    scheduled_for TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB
);
CREATE INDEX IF NOT EXISTS idx_notification_queue_status ON notification_queue(status, scheduled_for);
CREATE INDEX IF NOT EXISTS idx_notification_queue_device ON notification_queue(device_id, status);

-- Convert to Hypertable
SELECT create_hypertable('telemetry', 'time', if_not_exists => TRUE);

-- Compression Policy (Save Disk) - Compress chunks older than 7 days
ALTER TABLE telemetry SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'device_id'
);
SELECT add_compression_policy('telemetry', INTERVAL '7 days', if_not_exists => TRUE);

-- 8. Phase 1 Updates (Gap Recovery)
ALTER TABLE telemetry ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'verified';
ALTER TABLE telemetry ADD COLUMN IF NOT EXISTS hops INTEGER DEFAULT 0;

CREATE TABLE IF NOT EXISTS anomalies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL, -- Logical Link to devices table? Or just UUID to allow loose coupling?
    type TEXT NOT NULL,      -- 'trend_up', 'threshold', 'spike'
    field TEXT NOT NULL,
    value DOUBLE PRECISION,
    severity TEXT,           -- 'critical', 'warning'
    description TEXT,
    detected_at TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS analytics_jobs (
    job_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id TEXT,
    query TEXT,
    params JSONB,
    status TEXT DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    results JSONB,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

-- 9. Phase 2 Updates (Service Injection)
CREATE TABLE IF NOT EXISTS rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL,
    device_id TEXT, -- Optional (Global vs Specific)
    name TEXT NOT NULL,
    trigger JSONB NOT NULL, -- { "metric": "temp", "op": ">", "val": 50 }
    actions JSONB NOT NULL, -- [{ "type": "alert", "msg": "High Temp" }]
    enabled BOOLEAN DEFAULT true,
    execution_location TEXT DEFAULT 'cloud', -- 'cloud' or 'edge'
    created_at TIMESTAMPTZ DEFAULT NOW()
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

CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    cron_expression TEXT,
    time TIMESTAMPTZ, -- For one-off scheduled jobs
    command JSONB NOT NULL, -- { "cmd": "pump_on" }
    is_active BOOLEAN DEFAULT true,
    last_run TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID, -- No FK enforcement for speed
    project_id TEXT,
    message TEXT,
    data JSONB,
    severity TEXT, -- 'info', 'warning', 'critical'
    status TEXT,   -- 'active', 'resolved'
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

-- 10. Phase 5 Updates (ERP Application Layer)

-- A. WORK ORDERS
CREATE TABLE IF NOT EXISTS work_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id TEXT UNIQUE NOT NULL, -- Human Readable "WO-2025-001"
    title TEXT NOT NULL,
    description TEXT,
    device_id TEXT NOT NULL, -- Linked Asset
    priority TEXT DEFAULT 'MEDIUM', -- LOW, MEDIUM, HIGH, CRITICAL
    status TEXT DEFAULT 'OPEN', -- OPEN, IN_PROGRESS, RESOLVED, CLOSED
    assigned_to TEXT, -- User UUID
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- B. INVENTORY
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sku TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    category TEXT DEFAULT 'GRAIN',
    unit TEXT DEFAULT 'kg',
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stock_levels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID REFERENCES products(id),
    location_id TEXT NOT NULL, -- Warehouse/Geofence ID
    quantity NUMERIC DEFAULT 0,
    last_updated TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(product_id, location_id)
);

-- C. LOGISTICS (Trips & Geofences)
CREATE TABLE IF NOT EXISTS trips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id TEXT UNIQUE NOT NULL, -- "TRIP-2025-001"
    project_id TEXT NOT NULL,
    vehicle_id TEXT,
    status TEXT DEFAULT 'SCHEDULED',
    route JSONB, -- Array of Checkpoints
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS geofences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    type TEXT, -- 'warehouse', 'depot', 'customer'
    polygon JSONB, -- GeoJSON or Point Array
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- D. MASTER DATA & STATES
CREATE TABLE IF NOT EXISTS master_data (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type TEXT NOT NULL, -- 'crop_types', 'soil_types'
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    project_id TEXT,
    is_active BOOLEAN DEFAULT true,
    UNIQUE(type, code, project_id)
);

CREATE TABLE IF NOT EXISTS states (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    iso_code TEXT,
    metadata JSONB
);

-- E. TRAFFIC
CREATE TABLE IF NOT EXISTS traffic_cameras (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    device_id TEXT UNIQUE, -- Link to R-Node
    location TEXT, -- Lat,Lng string or JSON
    status TEXT DEFAULT 'online',
    stream_url TEXT
);

-- 11. Phase 8 Updates (Advanced Configuration)

-- A. AUTOMATION FLOWS (Visual Builder)
CREATE TABLE IF NOT EXISTS automation_flows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT UNIQUE NOT NULL, -- One main flow per project for V1
    version TEXT DEFAULT '1.0.0',
    nodes JSONB NOT NULL, -- Array of Node-RED style nodes
    edges JSONB NOT NULL, -- Array of connections
    compiled_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    deployed_at TIMESTAMPTZ,
    deployed_by TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- B. DEVICE PROFILES (Hardware Definitions)
CREATE TABLE IF NOT EXISTS device_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL, -- 'Danfoss VLT Micro'
    manufacturer TEXT,
    protocol TEXT NOT NULL, -- 'MODBUS_RTU', 'LORAWAN'
    
    -- Deep Config
    registers JSONB, -- Modbus Map
    decoder_script TEXT, -- JS Payload Decoder
    lora_config JSONB,
    comm_settings JSONB,
    
    version INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 12. Phase 9 Updates (Vertical Domains)

-- A. AGRICULTURE & INSTALLATIONS (Beneficiaries)
CREATE TABLE IF NOT EXISTS beneficiaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    type TEXT DEFAULT 'individual', -- individual, organization
    phone TEXT,
    email TEXT,
    address JSONB, -- { street, village, district, state }
    state_id TEXT, -- Ref to State
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- B. INSTALLATIONS (Device Deployment)
CREATE TABLE IF NOT EXISTS installations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL, -- Logical link to devices
    project_id TEXT NOT NULL,
    beneficiary_id UUID REFERENCES beneficiaries(id),
    
    geo_location JSONB, -- { lat, lng }
    status TEXT DEFAULT 'active', -- active, decommissioned
    activated_at TIMESTAMPTZ DEFAULT NOW(),
    decommissioned_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- C. HEALTHCARE
CREATE TABLE IF NOT EXISTS patients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    patient_id TEXT UNIQUE NOT NULL, -- "P-1001"
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    age INT,
    gender TEXT,
    assigned_doctor_id TEXT,
    medical_history TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS medical_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id TEXT UNIQUE NOT NULL, -- "SESS-123456"
    patient_id TEXT REFERENCES patients(patient_id),
    device_id TEXT NOT NULL,
    doctor_id TEXT,
    status TEXT DEFAULT 'ACTIVE', -- ACTIVE, COMPLETED
    vitals JSONB,
    notes TEXT,
    start_time TIMESTAMPTZ DEFAULT NOW(),
    end_time TIMESTAMPTZ
);

-- D. GIS
CREATE TABLE IF NOT EXISTS gis_layers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT, -- LINE, POLYGON, POINT
    geojson JSONB NOT NULL,
    style JSONB, -- { color, weight }
    created_at TIMESTAMPTZ DEFAULT NOW()
);




-- 13. Round 3 Updates (Utilities)

-- A. MQTT PROVISIONING JOBS
CREATE TABLE IF NOT EXISTS mqtt_provisioning_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    trigger_kind TEXT DEFAULT 'initial', -- 'initial', 'resync', 'manual'
    attempt_count INT DEFAULT 0,
    consecutive_failures INT DEFAULT 0,
    next_attempt_at TIMESTAMPTZ DEFAULT NOW(),
    last_result TEXT,
    last_error TEXT,
    last_error_category TEXT,
    processing_started_at TIMESTAMPTZ,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    last_duration_ms INT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for Worker Polling
CREATE INDEX IF NOT EXISTS idx_mqtt_jobs_status_next ON mqtt_provisioning_jobs (status, next_attempt_at);

-- A2. DEVICE CONFIGURATION QUEUE
CREATE TABLE IF NOT EXISTS device_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    config JSONB NOT NULL DEFAULT '{}',
    status TEXT DEFAULT 'pending', -- pending, acknowledged
    ack_payload JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_device_configurations_device ON device_configurations(device_id, status);

-- A3. DEVICE IMPORT JOBS
CREATE TABLE IF NOT EXISTS import_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_type TEXT NOT NULL,
    project_id TEXT,
    status TEXT DEFAULT 'completed',
    total_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    error_count INT DEFAULT 0,
    errors JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_import_jobs_type ON import_jobs(job_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_import_jobs_project ON import_jobs(project_id, created_at DESC);

-- B. STATE AUTHORITIES (Agri)
CREATE TABLE IF NOT EXISTS authorities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    state_id UUID REFERENCES states(id),
    type TEXT, -- 'mseb', 'irrigation', 'pwd'
    contact_info JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- C. SCHEDULES (Missing Relation Fix)


CREATE INDEX IF NOT EXISTS idx_schedules_active ON schedules(is_active);
CREATE INDEX IF NOT EXISTS idx_schedules_time ON schedules(time);

-- 14. Phase 10: API Keys (Machine-to-Machine Auth)
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL, -- "Zapier Prod"
    key_hash TEXT NOT NULL, -- BCrypt Hash of the key
    prefix TEXT NOT NULL, -- "ak_1234" (first 7 chars for ID)
    project_id TEXT, -- Optional (Global or Project Scoped)
    scopes JSONB DEFAULT '[]', -- ["read:telemetry", "write:command"]
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID, -- User ID
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);

-- Project/User governance columns that depend on the projects table
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS visibility TEXT DEFAULT 'internal',
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS created_by UUID,
    ADD COLUMN IF NOT EXISTS updated_by UUID;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS default_org_id UUID REFERENCES organizations(id),
    ADD COLUMN IF NOT EXISTS default_project_id TEXT REFERENCES projects(id);

CREATE TABLE IF NOT EXISTS user_org_memberships (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    role TEXT REFERENCES org_roles(slug),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, org_id)
);

CREATE TABLE IF NOT EXISTS user_project_memberships (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    role TEXT REFERENCES project_roles(slug),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, project_id)
);

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id),
    ADD COLUMN IF NOT EXISTS role_scope TEXT;
CREATE INDEX IF NOT EXISTS idx_api_keys_org ON api_keys(org_id);

INSERT INTO capabilities (key, description) VALUES
    ('telemetry:read', 'View telemetry data'),
    ('telemetry:live:device', 'Stream live telemetry for assigned devices'),
    ('telemetry:live:all', 'Stream live telemetry for all devices'),
    ('telemetry:export', 'Export telemetry data'),
    ('alerts:manage', 'Manage alerts and thresholds'),
    ('reports:manage', 'Manage reports'),
    ('devices:read', 'View devices'),
    ('devices:write', 'Create and update devices'),
    ('devices:credentials', 'Rotate device credentials'),
    ('devices:commands', 'Send device commands'),
    ('devices:bulk_import', 'Bulk import devices'),
    ('simulator:launch', 'Launch simulator'),
    ('simulator:commands', 'Send simulator commands'),
    ('diagnostics:read', 'View diagnostics'),
    ('diagnostics:commands', 'Run diagnostics commands'),
    ('catalog:protocols', 'Manage protocols catalog'),
    ('catalog:drives', 'Manage VFD catalog'),
    ('catalog:rs485', 'Manage RS485 maps'),
    ('hierarchy:manage', 'Manage hierarchy'),
    ('vendors:manage', 'Manage vendors'),
    ('installations:manage', 'Manage installations'),
    ('beneficiaries:manage', 'Manage beneficiaries'),
    ('users:manage', 'Manage users'),
    ('audit:read', 'View audit logs'),
    ('support:manage', 'Manage support tooling'),
    ('knowledge_base:manage', 'Manage knowledge base'),
    ('admin:all', 'Administrative overrides')
ON CONFLICT (key) DO NOTHING;


-- ============================================================================
-- Embedded migrations (v1_verticals, v2_credential_history, v3_vfd_bundle, v4_protocols)
-- Added 2026-01-02 so fresh installs do not require running separate SQL files.
-- ============================================================================

-- v1_verticals.sql
CREATE TABLE IF NOT EXISTS traffic_metrics (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    device_id VARCHAR(50),
    vehicle_count INT,
    breakdown JSONB,
    avg_speed FLOAT,
    congestion_level VARCHAR(20),
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS soil_rules (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    project_id VARCHAR(50),
    crop VARCHAR(50),
    parameter VARCHAR(50),
    operator VARCHAR(10),
    threshold FLOAT,
    threshold_max FLOAT,
    advisory TEXT,
    severity VARCHAR(20)
);

-- v2_credential_history.sql
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

ALTER TABLE mqtt_provisioning_jobs
    ADD COLUMN IF NOT EXISTS credential_history_id UUID REFERENCES credential_history(id);

ALTER TABLE mqtt_provisioning_jobs
    ADD COLUMN IF NOT EXISTS trigger_kind TEXT DEFAULT 'initial',
    ADD COLUMN IF NOT EXISTS consecutive_failures INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_result TEXT,
    ADD COLUMN IF NOT EXISTS last_error_category TEXT,
    ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_success_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_failure_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_duration_ms INT;

CREATE INDEX IF NOT EXISTS idx_cred_hist_device ON credential_history(device_id);
CREATE INDEX IF NOT EXISTS idx_mqtt_jobs_cred_id ON mqtt_provisioning_jobs(credential_history_id);
CREATE INDEX IF NOT EXISTS idx_mqtt_jobs_trigger_kind ON mqtt_provisioning_jobs(trigger_kind);

-- v5_simulator_sessions.sql
CREATE TABLE IF NOT EXISTS simulator_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
    device_uuid TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    requested_by UUID REFERENCES users(id),
    revoked_by UUID REFERENCES users(id),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    credential_snapshot JSONB NOT NULL DEFAULT '{}',
    command_quota JSONB NOT NULL DEFAULT '{"limit":20,"active_count":0}'
);

CREATE INDEX IF NOT EXISTS idx_simulator_sessions_status ON simulator_sessions(status);
CREATE INDEX IF NOT EXISTS idx_simulator_sessions_device ON simulator_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_simulator_sessions_created ON simulator_sessions(created_at DESC);

-- v4_protocols_and_govt_creds.sql (create protocols before columns that reference it)
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

-- v3_beneficiaries_installations_vfd.sql (align beneficiaries/installations and add VFD tables)
ALTER TABLE beneficiaries
    ADD COLUMN IF NOT EXISTS project_id TEXT NOT NULL DEFAULT 'proj_default',
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();
CREATE INDEX IF NOT EXISTS idx_beneficiaries_project ON beneficiaries(project_id);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_phone ON beneficiaries(phone);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_email ON beneficiaries(email);

CREATE TABLE IF NOT EXISTS vfd_manufacturers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS vfd_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    manufacturer_id UUID NOT NULL REFERENCES vfd_manufacturers(id) ON DELETE RESTRICT,
    model TEXT NOT NULL,
    version TEXT NOT NULL,
    rs485 JSONB NOT NULL DEFAULT '{}',
    realtime_parameters JSONB DEFAULT '[]',
    fault_map JSONB DEFAULT '[]',
    command_dictionary JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, manufacturer_id, model, version)
);

CREATE TABLE IF NOT EXISTS protocol_vfd_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    protocol_id UUID NOT NULL REFERENCES protocols(id) ON DELETE CASCADE,
    vfd_model_id UUID NOT NULL REFERENCES vfd_models(id) ON DELETE CASCADE,
    assigned_by TEXT,
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    revoked_by TEXT,
    revocation_reason TEXT,
    metadata JSONB DEFAULT '{}',
    UNIQUE(project_id, protocol_id, vfd_model_id)
);
CREATE INDEX IF NOT EXISTS idx_protocol_vfd_assignments_protocol ON protocol_vfd_assignments(protocol_id);
CREATE INDEX IF NOT EXISTS idx_protocol_vfd_assignments_vfd ON protocol_vfd_assignments(vfd_model_id);

CREATE TABLE IF NOT EXISTS vfd_command_import_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL,
    vfd_model_id UUID REFERENCES vfd_models(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    error TEXT,
    summary JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_vfd_import_jobs_project ON vfd_command_import_jobs(project_id);
CREATE INDEX IF NOT EXISTS idx_vfd_import_jobs_model ON vfd_command_import_jobs(vfd_model_id);

CREATE TABLE IF NOT EXISTS installation_beneficiaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    installation_id UUID NOT NULL REFERENCES installations(id) ON DELETE CASCADE,
    beneficiary_id UUID NOT NULL REFERENCES beneficiaries(id) ON DELETE CASCADE,
    role TEXT,
    removed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_installation_beneficiaries_installation ON installation_beneficiaries(installation_id);
CREATE INDEX IF NOT EXISTS idx_installation_beneficiaries_beneficiary ON installation_beneficiaries(beneficiary_id);

ALTER TABLE installations
    ADD COLUMN IF NOT EXISTS beneficiary_id UUID REFERENCES beneficiaries(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS protocol_id UUID REFERENCES protocols(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS vfd_model_id UUID REFERENCES vfd_models(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();
CREATE UNIQUE INDEX IF NOT EXISTS idx_installations_device_unique ON installations(device_id);
CREATE INDEX IF NOT EXISTS idx_installations_project ON installations(project_id);
CREATE INDEX IF NOT EXISTS idx_installations_beneficiary ON installations(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_installations_protocol ON installations(protocol_id);
CREATE INDEX IF NOT EXISTS idx_installations_vfd_model ON installations(vfd_model_id);

-- ============================================================================
-- Embedded command-control schema (merged from v7_command_control.sql)
-- Adds catalog, capabilities, requests/responses, and response patterns
-- ============================================================================

CREATE TABLE IF NOT EXISTS command_catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('core','protocol','model','project')),
    protocol_id UUID NULL,
    model_id UUID NULL,
    project_id TEXT NULL REFERENCES projects(id) ON DELETE CASCADE,
    payload_schema JSONB NULL,
    transport TEXT NOT NULL CHECK (transport IN ('mqtt','http')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS device_capabilities (
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    command_id UUID NOT NULL REFERENCES command_catalog(id) ON DELETE CASCADE,
    PRIMARY KEY (device_id, command_id)
);

CREATE TABLE IF NOT EXISTS project_command_overrides (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    command_id UUID NOT NULL REFERENCES command_catalog(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (project_id, command_id)
);

CREATE TABLE IF NOT EXISTS response_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    command_id UUID NOT NULL REFERENCES command_catalog(id) ON DELETE CASCADE,
    pattern_type TEXT NOT NULL CHECK (pattern_type IN ('regex','jsonpath')),
    pattern TEXT NOT NULL,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    extract JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS command_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    command_id UUID NOT NULL REFERENCES command_catalog(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued','published','acked','failed','timeout')),
    retries INT NOT NULL DEFAULT 0,
    correlation_id UUID NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS command_responses (
    correlation_id UUID PRIMARY KEY REFERENCES command_requests(correlation_id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    raw_response JSONB NULL,
    parsed JSONB NULL,
    matched_pattern_id UUID NULL REFERENCES response_patterns(id) ON DELETE SET NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_command_requests_device_status ON command_requests(device_id, status);
CREATE INDEX IF NOT EXISTS idx_command_requests_project ON command_requests(project_id);
CREATE INDEX IF NOT EXISTS idx_command_responses_device ON command_responses(device_id);

-- PM-KUSUM-only production posture:
-- demo convenience seeds and legacy RMS sample seeds were removed intentionally.


