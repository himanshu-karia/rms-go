-- new-db-schema/sql/verticals/pm_kusum.sql
-- PM-KUSUM / RMS-style deployment bundle.
-- Depends on: core.sql (projects, devices, protocols)

-- States (master data)
CREATE TABLE IF NOT EXISTS states (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    iso_code TEXT,
    metadata JSONB
);

-- State authorities (RMS workflows)
CREATE TABLE IF NOT EXISTS authorities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    state_id UUID REFERENCES states(id),
    type TEXT,
    contact_info JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Beneficiaries (project-scoped)
CREATE TABLE IF NOT EXISTS beneficiaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT DEFAULT 'individual',
    phone TEXT,
    email TEXT,
    address JSONB,
    state_id UUID REFERENCES states(id),
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_beneficiaries_project ON beneficiaries(project_id);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_phone ON beneficiaries(phone);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_email ON beneficiaries(email);

-- VFD catalogue
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

-- Installations (device deployment chain)
CREATE TABLE IF NOT EXISTS installations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    beneficiary_id UUID REFERENCES beneficiaries(id) ON DELETE SET NULL,
    location JSONB NOT NULL DEFAULT '{}'::jsonb,
    protocol_id UUID REFERENCES protocols(id) ON DELETE SET NULL,
    vfd_model_id UUID REFERENCES vfd_models(id) ON DELETE SET NULL,
    status TEXT DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    activated_at TIMESTAMPTZ DEFAULT NOW(),
    decommissioned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_installations_device_unique ON installations(device_id);
CREATE INDEX IF NOT EXISTS idx_installations_project ON installations(project_id);
CREATE INDEX IF NOT EXISTS idx_installations_beneficiary ON installations(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_installations_protocol ON installations(protocol_id);
CREATE INDEX IF NOT EXISTS idx_installations_vfd_model ON installations(vfd_model_id);
