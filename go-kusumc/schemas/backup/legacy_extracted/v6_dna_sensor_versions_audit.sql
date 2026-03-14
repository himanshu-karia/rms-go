-- Add audit actors to dna_sensor_versions
ALTER TABLE IF EXISTS dna_sensor_versions
    ADD COLUMN IF NOT EXISTS created_by TEXT,
    ADD COLUMN IF NOT EXISTS published_by TEXT,
    ADD COLUMN IF NOT EXISTS rolled_back_by TEXT;
