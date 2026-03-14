-- Credential history and job linkage
CREATE TABLE IF NOT EXISTS credential_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    bundle JSONB NOT NULL,
    lifecycle TEXT NOT NULL DEFAULT 'pending', -- pending, applied, failed
    mqtt_access_applied BOOLEAN NOT NULL DEFAULT false,
    last_error TEXT,
    attempt_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Link provisioning jobs to credential history
ALTER TABLE mqtt_provisioning_jobs
    ADD COLUMN IF NOT EXISTS credential_history_id UUID REFERENCES credential_history(id);

CREATE INDEX IF NOT EXISTS idx_cred_hist_device ON credential_history(device_id);
CREATE INDEX IF NOT EXISTS idx_mqtt_jobs_cred_id ON mqtt_provisioning_jobs(credential_history_id);
