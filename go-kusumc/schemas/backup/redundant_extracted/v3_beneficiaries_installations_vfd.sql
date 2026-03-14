-- V3 schema additions for beneficiaries, installations, VFDs, and protocol assignments (project-scoped)

BEGIN;

-- Beneficiaries (project-scoped persons)
CREATE TABLE IF NOT EXISTS beneficiaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    phone TEXT,
    email TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_project ON beneficiaries(project_id);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_phone ON beneficiaries(phone);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_email ON beneficiaries(email);

-- VFD Manufacturers
CREATE TABLE IF NOT EXISTS vfd_manufacturers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vfd_manufacturers_name_proj ON vfd_manufacturers(project_id, name);

-- VFD Models (RS485/commands/faults as JSONB)
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

-- Protocol ↔ VFD assignment (per project)
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

-- Installations (bind device + beneficiary + protocol + vfd_model)
CREATE TABLE IF NOT EXISTS installations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    beneficiary_id UUID REFERENCES beneficiaries(id) ON DELETE SET NULL,
    location JSONB DEFAULT '{}'::jsonb,
    protocol_id UUID REFERENCES protocols(id) ON DELETE SET NULL,
    vfd_model_id UUID REFERENCES vfd_models(id) ON DELETE SET NULL,
    status TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(device_id)
);
CREATE INDEX IF NOT EXISTS idx_installations_project ON installations(project_id);
CREATE INDEX IF NOT EXISTS idx_installations_beneficiary ON installations(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_installations_protocol ON installations(protocol_id);
CREATE INDEX IF NOT EXISTS idx_installations_vfd_model ON installations(vfd_model_id);

COMMIT;
