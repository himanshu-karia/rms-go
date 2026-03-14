-- V4 schema additions for protocol profiles and per-device government credentials

BEGIN;

-- Protocol profiles (primary/govt) per project and server vendor
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

-- Per-device government credentials (store/return only, no EMQX sync)
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

COMMIT;
