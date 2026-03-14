# Schema bundles (core + verticals)

## Larger goal

This repo is building a **Unified IoT platform**:
- One engine and UI
- Multiple “project types” (verticals)
- Most per-project variation should live in **Project DNA** (JSON config + payload schema), not in separate DB schemas.

So the DB should be:
- **One shared database** (not one DB per project)
- Core tables always present
- Optional vertical bundles added as needed

## Bundles

### `core`
Always present; platform fundamentals.

Includes:
- Auth/tenancy: `organizations`, `link_roles`, `users`, `api_keys`
- Project definition: `projects`, `project_dna`, `payload_sensors`, `telemetry_thresholds`, `dna_sensor_versions`
- Device registry: `devices`, `device_links`
- Telemetry store: `telemetry` (Timescale hypertable)
- Platform services (cross-vertical): `rules`, `alerts`, `schedules`, `command_logs`, `anomalies`, `analytics_jobs`, `automation_flows`, `device_profiles`
- Connectivity/provisioning primitives: `protocols`, `device_govt_credentials`, `mqtt_provisioning_jobs`, `credential_history`

### `vertical_pm_kusum`
PM-KUSUM / RMS-style deployment model.

Includes:
- People + deployment chain: `beneficiaries`, `installations`
- State/authority master data used by RMS-style workflows: `states`, `authorities`
- VFD catalogue + protocol assignments (currently included here): `vfd_manufacturers`, `vfd_models`, `protocol_vfd_assignments`

### `vertical_healthcare`
Includes:
- `patients`, `medical_sessions`

### `vertical_gis`
Includes:
- `gis_layers`

### `vertical_traffic`
Includes:
- `traffic_cameras`, `traffic_metrics`

### `vertical_logistics`
Includes:
- Inventory: `products`, `stock_levels`
- Logistics: `trips`, `geofences`

### `vertical_maintenance`
Includes:
- `work_orders`

## FK / dependency rules

- Vertical tables may have foreign keys into core tables.
- Core tables must never foreign-key into vertical tables.

If a feature needs to link across verticals:
- Prefer a **core “association/link” table** (generic), or
- Use a soft reference (UUID + type) until it becomes a core concept.

## Suggested init order

1. `core.sql`
2. Any vertical bundle(s), in any order **as long as dependencies are satisfied**.
   - Example: `pm_kusum.sql` depends on `core.sql` because it references `projects`, `devices`, `protocols`.

## Docker compose example

Mount in this order (fresh volume only):

- `./new-db-schema/sql/core.sql:/docker-entrypoint-initdb.d/01_core.sql`
- `./new-db-schema/sql/verticals/pm_kusum.sql:/docker-entrypoint-initdb.d/02_pm_kusum.sql`
