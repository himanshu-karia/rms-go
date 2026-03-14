-- Add payload storage for alerts (rule trigger packet / device error packet)
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS data JSONB;
