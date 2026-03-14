-- new-db-schema/sql/verticals/traffic.sql
-- Traffic bundle.
-- Depends on: core.sql (uuid-ossp)

CREATE TABLE IF NOT EXISTS traffic_cameras (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    device_id TEXT UNIQUE,
    location TEXT,
    status TEXT DEFAULT 'online',
    stream_url TEXT
);

CREATE TABLE IF NOT EXISTS traffic_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id VARCHAR(50),
    vehicle_count INT,
    breakdown JSONB,
    avg_speed FLOAT,
    congestion_level VARCHAR(20),
    timestamp TIMESTAMPTZ DEFAULT NOW()
);
