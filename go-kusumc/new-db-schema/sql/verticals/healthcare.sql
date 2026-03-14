-- new-db-schema/sql/verticals/healthcare.sql
-- Healthcare bundle.
-- Depends on: core.sql (projects)

CREATE TABLE IF NOT EXISTS patients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    patient_id TEXT UNIQUE NOT NULL,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    age INT,
    gender TEXT,
    assigned_doctor_id TEXT,
    medical_history TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS medical_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id TEXT UNIQUE NOT NULL,
    patient_id TEXT REFERENCES patients(patient_id) ON DELETE SET NULL,
    device_id TEXT NOT NULL,
    doctor_id TEXT,
    status TEXT DEFAULT 'ACTIVE',
    vitals JSONB,
    notes TEXT,
    start_time TIMESTAMPTZ DEFAULT NOW(),
    end_time TIMESTAMPTZ
);
