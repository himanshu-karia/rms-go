-- Command & Control schema
-- Adds catalog, capabilities, requests/responses, and response patterns

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
