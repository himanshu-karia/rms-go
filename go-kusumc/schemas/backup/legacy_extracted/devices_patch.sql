-- Idempotent patch to ensure device metadata columns exist
ALTER TABLE IF NOT EXISTS devices ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE IF NOT EXISTS devices ADD COLUMN IF NOT EXISTS status TEXT;
