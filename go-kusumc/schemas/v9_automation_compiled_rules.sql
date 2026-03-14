-- Add durable storage for compiled automation rules (graph->rules dual representation)
ALTER TABLE IF EXISTS automation_flows
    ADD COLUMN IF NOT EXISTS compiled_rules JSONB NOT NULL DEFAULT '[]'::jsonb;
