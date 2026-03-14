-- Mesh nodes + gateway forwarding associations

CREATE TABLE IF NOT EXISTS mesh_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    label TEXT NULL,
    kind TEXT NOT NULL DEFAULT 'mesh',
    attributes JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, node_id)
);

CREATE TABLE IF NOT EXISTS mesh_gateway_nodes (
    gateway_device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES mesh_nodes(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    discovered BOOLEAN NOT NULL DEFAULT FALSE,
    last_seen TIMESTAMPTZ NULL,
    metadata JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (gateway_device_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_mesh_nodes_project ON mesh_nodes(project_id);
CREATE INDEX IF NOT EXISTS idx_mesh_gateway_nodes_gateway ON mesh_gateway_nodes(gateway_device_id);
CREATE INDEX IF NOT EXISTS idx_mesh_gateway_nodes_node ON mesh_gateway_nodes(node_id);
