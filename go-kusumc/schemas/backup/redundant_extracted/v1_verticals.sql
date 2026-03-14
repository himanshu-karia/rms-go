-- Vertical Business Logic Schemas (V1.2 Lite)
-- Note: Healthcare, Logistics, and OTA moved to v1_init.sql

-- 1. Traffic
CREATE TABLE IF NOT EXISTS traffic_metrics (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    device_id VARCHAR(50),
    vehicle_count INT,
    breakdown JSONB, -- { "car": 5, "bike": 2 }
    avg_speed FLOAT,
    congestion_level VARCHAR(20), -- LOW, MODERATE, HIGH
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Agriculture
CREATE TABLE IF NOT EXISTS soil_rules (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    project_id VARCHAR(50),
    crop VARCHAR(50),
    parameter VARCHAR(50), -- pH, N, P, K
    operator VARCHAR(10), -- <, >, BETWEEN
    threshold FLOAT,
    threshold_max FLOAT,
    advisory TEXT,
    severity VARCHAR(20)
);
