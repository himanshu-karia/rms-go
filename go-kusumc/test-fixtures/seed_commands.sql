-- Seed fixtures for Command Center e2e tests
-- Creates a test project, a device, a project-scoped command, and grants the device the capability to run it.

-- create project if missing
INSERT INTO projects (id, name)
VALUES ('test-project', 'E2E Test Project')
ON CONFLICT (id) DO NOTHING;

-- create device if missing
INSERT INTO devices (imei, project_id, name)
VALUES ('TEST-IMEI-001', 'test-project', 'E2E Device')
ON CONFLICT (imei) DO NOTHING;

-- create project-scoped command if missing
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM command_catalog WHERE name = 'E2E_Set' AND project_id = 'test-project') THEN
    INSERT INTO command_catalog (name, scope, project_id, payload_schema, transport)
    VALUES (
      'E2E_Set',
      'project',
      'test-project',
      ('{"type":"object","required":["mode"],"properties":{"mode":{"type":"string","enum":["on","off"]}}}')::jsonb,
      'mqtt'
    );
  END IF;
END$$;

-- grant the device capability to use the command (if not already granted)
INSERT INTO device_capabilities (device_id, command_id)
SELECT d.id, c.id
FROM devices d
JOIN command_catalog c ON c.name = 'E2E_Set' AND c.project_id = 'test-project'
WHERE d.imei = 'TEST-IMEI-001'
  AND NOT EXISTS (
    SELECT 1 FROM device_capabilities dc WHERE dc.device_id = d.id AND dc.command_id = c.id
  );

-- Done
SELECT 'seed_commands.sql applied' AS info;

-- ============================================================================
-- RMS baseline seeds (org/vendor, project, protocols, command catalog, device)
-- ============================================================================

-- Organizations (stable UUIDs for references)
INSERT INTO organizations (id, name, type, path, metadata) VALUES
  ('11111111-1111-1111-1111-111111111111', 'State Water Agency', 'govt', 'India.Maharashtra', '{}'),
  ('22222222-2222-2222-2222-222222222222', 'RMS Integration Vendor', 'vendor', 'India.Maharashtra.RMSVendor', '{}'),
  ('33333333-3333-3333-3333-333333333333', 'Server Vendor', 'vendor', 'India.Maharashtra.ServerVendor', '{}'),
  ('44444444-4444-4444-4444-444444444444', 'Pump OEM', 'vendor', 'India.Maharashtra.PumpOEM', '{}')
ON CONFLICT (id) DO NOTHING;

-- Project
INSERT INTO projects (id, name, type, location, config)
VALUES ('rms-pump-01', 'RMS Solar Pump', 'rms', 'Maharashtra', '{}')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type, location = EXCLUDED.location;

-- Protocols (primary + govt) linked to server vendor org
INSERT INTO protocols (project_id, server_vendor_org_id, kind, protocol, host, port, publish_topics, subscribe_topics, metadata)
VALUES
  ('rms-pump-01', '33333333-3333-3333-3333-333333333333', 'primary', 'mqtt', 'localhost', 1883, '["channels/rms-pump-01/messages/{imei}"]', '["channels/rms-pump-01/commands/{imei}"]', '{}'),
  ('rms-pump-01', '33333333-3333-3333-3333-333333333333', 'govt', 'mqtts', 'gw.gov.example.com', 8883, '["gov/rms-pump-01/publish/{imei}"]', '["gov/rms-pump-01/commands/{imei}"]', '{"note":"sample govt endpoint"}')
ON CONFLICT (project_id, kind, host, port) DO NOTHING;

-- Command catalog entries (project scope)
INSERT INTO command_catalog (name, scope, project_id, payload_schema, transport)
SELECT * FROM (
    VALUES
      ('pump_start'::text, 'project'::text, 'rms-pump-01'::text, '{"type":"object","properties":{"reason":{"type":"string","enum":["schedule","manual"]}},"required":["reason"]}'::jsonb, 'mqtt'::text),
      ('pump_stop', 'project', 'rms-pump-01', '{"type":"object","properties":{"reason":{"type":"string","enum":["schedule","manual"]}},"required":["reason"]}'::jsonb, 'mqtt'),
      ('read_status', 'project', 'rms-pump-01', '{"type":"object","properties":{"verbosity":{"type":"string","enum":["summary","full"],"default":"summary"}}}'::jsonb, 'mqtt')
) AS v(name, scope, project_id, payload_schema, transport)
WHERE NOT EXISTS (
    SELECT 1 FROM command_catalog c WHERE c.name = v.name AND c.project_id = v.project_id
);

-- Device with MQTT creds encoded in attributes
INSERT INTO devices (imei, project_id, name, status, attributes)
VALUES ('RMS-DEVICE-001', 'rms-pump-01', 'RMS Pump Device', 'active', '{"mqtt_username":"rms-device-001","mqtt_password":"rms-pass-001","mqtt_client_id":"rms-device-001"}')
ON CONFLICT (imei) DO NOTHING;

-- Attach device capabilities to project commands
INSERT INTO device_capabilities (device_id, command_id)
SELECT d.id, c.id
FROM devices d
JOIN command_catalog c ON c.project_id = 'rms-pump-01' AND c.name IN ('pump_start','pump_stop','read_status')
WHERE d.imei = 'RMS-DEVICE-001'
  AND NOT EXISTS (
    SELECT 1 FROM device_capabilities dc WHERE dc.device_id = d.id AND dc.command_id = c.id
  );

-- Optional: enable project overrides (default TRUE) to ensure visibility
INSERT INTO project_command_overrides (project_id, command_id, enabled)
SELECT 'rms-pump-01', id, TRUE FROM command_catalog
WHERE project_id = 'rms-pump-01'
ON CONFLICT (project_id, command_id) DO NOTHING;

SELECT 'seed_commands.sql RMS block applied' AS info;
