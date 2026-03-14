# Provisioning Upgrade Plan

Bridging legacy refer-rms hierarchy and MQTT provisioning with the new Timescale org tree + Project DNA.

## 0) Legacy RMS hierarchy (Mongo) recap
- Orgs: `states` → `state_authorities` → `projects`.
- Vendors: `server_vendors`, `solar_pump_vendors`, `vfd_drive_manufacturers`, `rms_manufacturers`.
- Protocols: `protocol_versions` keyed by `{stateId, authorityId, projectId, serverVendorId, version}` with topic suffix metadata (`<IMEI>/heartbeat`, etc.).
- Devices: `devices` reference `protocolVersionId`; telemetry keyed by imei/deviceId.
- Installations/beneficiaries: `installations`, `installation_beneficiaries`, `beneficiaries`.
- Govt creds: `government_credentials` per device with assignments: endpoints (protocol, host, port, url), publish/subscribe topics, protocolSelector snapshot, per-device creds.

## 1) Target schema (Timescale/Postgres)
- Org tree: `organizations` table with `path` (ltree) and `type` (`govt|private|vendor|authority|state|project-owner`), `parent_id` to model hierarchies. Use it to mirror state→authority→project ownership for PM-KUSUM.
- Projects: `projects(id text, name, type, location, config jsonb)`; store protocol/profile FKs inside `config`.
- Project DNA: `project_dna(project_id, payload_rows, edge_rules, virtual_sensors, automation_flows, metadata, updated_at)` remains the canonical payload schema.
- Devices: `devices(id uuid, imei unique, project_id, name, status, attributes jsonb, shadow jsonb, last_seen)`.
- Device links: `device_links(device_id, org_id, role)` to express ownership/maintainer/auditor, matching refer-rms multi-party graph.
- Telemetry: `telemetry(time, device_id, project_id, data jsonb, type, status, hops)` hypertable.
- Credentials history (from v2_credential_history.sql): keep per-device MQTT bundle history for traceability.

## 2) Protocol entity in new stack
- Define a reusable broker profile table (new): `protocols(id uuid, project_id text, server_vendor_org_id uuid, kind text, protocol text, host text, port int, publish_topics jsonb, subscribe_topics jsonb, metadata jsonb, created_at, updated_at)`.
  - `kind`: `primary` (our EMQX) or `govt` (external broker); `protocol`: `mqtt|mqtts|https`.
  - For PM-KUSUM, seed a `primary` profile with channels topics; seed a `govt` profile from provided govt details.
- Store the protocol reference in `projects.config`, e.g. `{"primary_protocol_id": "...", "govt_protocol_id": "..."}` to keep per-project defaults. Devices can override by FK if needed.
- Topic derivation: primary uses channels `channels/{project_id}/messages/{imei}` pub, `channels/{project_id}/commands/{imei}` sub. Govt uses whatever topics are defined in its profile.

## 3) Govt broker add-on (store/return only)
- Per-device govt creds: `client_id`, `username`, `password` (hash at rest) plus selected `govt_protocol_id`.
- Storage: add table `device_govt_credentials(device_id uuid, protocol_id uuid, client_id text, username text, password_enc text, created_at, updated_at)`.
- No EMQX sync for govt. We only store and return.

## 4) Provisioning flow (UI + CSV)
- Inputs per device: IMEI, name, project_id, vendor/org links, `protocol_id` (primary), optional `govt_protocol_id`, optional govt creds (`client_id`, `username`, `password`).
- CSV columns: `imei,name,project_id,protocol_id,govt_protocol_id,govt_client_id,govt_username,govt_password` (extend as needed; project_id can default from querystring).
- Steps:
  1. Create device row; link to project; attach org links via `device_links` (owner/maintainer/etc.).
  2. Create EMQX creds + ACL (primary broker) and record in credentials history.
  3. Persist protocol refs on device (or rely on project config defaults).
  4. If govt creds provided, upsert into `device_govt_credentials` with the selected `govt_protocol_id`.

## 5) APIs
- Protocol profiles: `POST /api/projects/{id}/protocols` (primary or govt); `GET /api/projects/{id}/protocols` list.
- Govt creds: `POST /api/devices/{id}/govt-creds` (and bulk variant) to attach per-device govt creds + protocol; `GET /api/devices/{id}/govt-creds` to fetch.
- Bootstrap: `GET /api/devices/{id}/bootstrap` returns:
  - `primary_broker`: {protocol, host, port, publish_topic(s), subscribe_topic(s), client_id, username, password, envelope_required_fields}.
    - Defaults derive from `MQTT_PUBLIC_PROTOCOL|HOST|PORT` (fallback `mqtts://iot.local:8883`).
    - Optional phased rollout: set `MQTT_PUBLIC_URLS` (comma-separated) to advertise both endpoints, e.g. `mqtts://iot.local:8883,mqtt://iot.local:1884`.
  - `govt_broker` (optional): {protocol, host, port, publish_topic(s), subscribe_topic(s), client_id, username, password}.
  - DNA metadata: schema version, payload fields expected (from `project_dna`).

## 6) Mapping refer-rms hierarchy onto Timescale org tree (PM-KUSUM example)
- Create org nodes: `India` (type state), `Maharashtra` (state), `MSEDCL` (authority/server_vendor), `PM-KUSUM Solar Pump RMS` (project owner). Use `path` ltree like `india.maharashtra.msedcl.pm_kusum`.
- Project row: `projects.id = 'pm-kusum-solar-pump-msedcl'`, `config` includes `primary_protocol_id`, optional `govt_protocol_id`, and any vendor references.
- Device links: owner = project owner org; maintainer = O&M org if any; auditor = authority if needed; manufacturer = OEM org.
- Protocol: seed a `primary` protocol profile with channels topics; seed a `govt` protocol profile per govt spec.

## 6b) Domain entities specific to PM-KUSUM (old vs new)
- Beneficiary (person): in refer-rms Mongo, `beneficiaries` with unique phone/email, linked via `installation_beneficiaries` to installations. In the new stack, model as a project-scoped person table (or JSON in `projects.config`), but since PM-KUSUM is the only user, keep it namespaced: `beneficiaries(project_id, uuid, name, phone, email, metadata)` and link via `installations.beneficiary_id`.
- Installation: refer-rms `installations` links device ↔ project/state/authority with lat/long. New stack: add `installations(id uuid, project_id, device_id, beneficiary_id, location jsonb{lat,lng,address}, protocol_id, vfd_model_id, status, metadata)` and link with `device_links` for org ownership. Mark PM-KUSUM-specific rows by `project_id`.
- VFD drive manufacturer/model: refer-rms has `vfd_drive_manufacturers`, `vfd_drive_models`, `protocol_vfd_assignments` (RS485 config, MODBUS addresses, commands, fault codes) and device configurations referencing them. New stack: add `vfd_manufacturers` + `vfd_models` tables and a join `protocol_vfd_assignments(protocol_id, vfd_model_id, revoked_at)`; store RS485/config/faults/commands as JSONB. Devices/Installations carry `vfd_model_id`; config pushes reuse existing command publishing.
- Scoping: all of the above stay project-scoped via `project_id` (and optionally org tree path). Do not leak into other projects—enforce by FK or by `project_id` column on each table and in APIs.

### Table sketches (new stack, project-scoped)
- `beneficiaries`:
  - `id uuid pk`, `project_id text fk projects(id)`, `name text`, `phone text`, `email text`, `metadata jsonb`, `created_at`, `updated_at`.
- `installations`:
  - `id uuid pk`, `project_id text fk`, `device_id uuid fk devices(id)`, `beneficiary_id uuid fk beneficiaries(id)`, `location jsonb` ({lat,lng,address}), `protocol_id uuid fk protocols(id)`, `vfd_model_id uuid fk vfd_models(id)`, `status text`, `metadata jsonb`, `created_at`, `updated_at`.
- `vfd_manufacturers`:
  - `id uuid pk`, `project_id text fk`, `name text`, `metadata jsonb`, timestamps.
- `vfd_models`:
  - `id uuid pk`, `project_id text fk`, `manufacturer_id uuid fk vfd_manufacturers(id)`, `model text`, `version text`, `rs485 jsonb` (baud_rate, data_bits, stop_bits, parity, flow_control, metadata), `realtime_parameters jsonb[]`, `fault_map jsonb[]`, `command_dictionary jsonb[]`, `metadata jsonb`, timestamps.
- `protocol_vfd_assignments`:
  - `id uuid pk`, `project_id text fk`, `protocol_id uuid fk protocols(id)`, `vfd_model_id uuid fk vfd_models(id)`, `assigned_by text`, `assigned_at timestamptz`, `revoked_at timestamptz`, `revoked_by text`, `revocation_reason text`, `metadata jsonb`.

### SQL migration stub
- Added `schemas/v3_beneficiaries_installations_vfd.sql` creating: `beneficiaries`, `vfd_manufacturers`, `vfd_models`, `protocol_vfd_assignments`, `installations` (project-scoped FKs, JSONB for RS485/commands/faults, unique constraints, indexes).
- Added `schemas/v4_protocols_and_govt_creds.sql` creating: `protocols` (primary/govt profiles per project/vendor) and `device_govt_credentials` (per-device govt bundles, store/return only).

### API shapes (VFD import/assign)
- Create manufacturer: `POST /api/projects/{projectId}/vfd/manufacturers { name, metadata }`.
- Create model: `POST /api/projects/{projectId}/vfd/models { manufacturer_id, model, version, rs485, realtime_parameters, fault_map, command_dictionary, metadata }`.
- Import command/fault dictionaries: `POST /api/projects/{projectId}/vfd/models/{modelId}/commands:import` accepting CSV/JSON payload with merge strategy (replace/append); reuse refer-rms shapes.
- Assign model to protocol: `POST /api/projects/{projectId}/protocols/{protocolId}/vfd-assignments { vfd_model_id, assigned_by?, metadata? }`.
- List assignments: `GET /api/projects/{projectId}/protocols/{protocolId}/vfd-assignments` (filter active/revoked).
- Revoke assignment: `POST /api/projects/{projectId}/protocols/{protocolId}/vfd-assignments/{assignmentId}/revoke { reason?, revoked_by? }`.
- Device configuration push (reuse): when a device has `vfd_model_id` and protocol assignment exists, issue CONFIG_SYNC with the model’s command dictionary/RS485 payload.

### Mapping old → new for VFD
- Old: `vfd_drive_manufacturers`, `vfd_drive_models`, `protocol_vfd_assignments`, device_configurations referencing assignment, with RS485 + commands + faults as documents.
- New: `vfd_manufacturers`, `vfd_models`, `protocol_vfd_assignments` (JSONB for rs485/commands/faults), `installations.vfd_model_id` and `devices` carry the FK; provisioning/bootstrap can expose assigned vfd_model and protocol assignment id if needed.

## 7) Code touchpoints (planned changes)

## 7) Code touchpoints (planned changes)
- Add PM-KUSUM-scoped tables/APIs for beneficiaries and installations; wire installation create/import to attach beneficiary + location + device + protocol + vfd model.
- Add VFD manufacturer/model tables and protocol assignments; expose import APIs for command/fault dictionaries (reuse refer-rms CSV/JSON shapes where possible).

## 8) Firmware guidance
- Primary broker: publish `channels/{project_id}/messages/{imei}`, subscribe `channels/{project_id}/commands/{imei}`, include envelope fields (`packet_type`, `project_id`, `protocol_id`, `device_id`, `imei`, `msgid`, `timestamp`).
- Govt broker: connect using returned creds and topics; publish same payload. Commands remain on primary unless govt requires otherwise.

## 9) Migration steps
- Create new tables (`protocols`, `device_govt_credentials`) via migration.
- Populate org tree for PM-KUSUM, project row, protocol profiles, and sample devices.
- Wire services/repos/controllers for protocols, govt creds, bootstrap response.
- Update frontend forms/CSV import to include protocol selection and govt creds.
- Backfill existing devices with primary protocol_id via project defaults.

## 10) Risks/notes
- Ensure creds at rest: hash or encrypt govt passwords; avoid logging.
- Keep channels contract as primary; legacy suffix topics only via explicit bridge if ever needed.
- Validate project/org linkage so protocol/profile selection cannot cross projects inadvertently.

## 11) Gap status & next steps (backend)
- Beneficiaries/installations: only basic create/list; add project-scoped filters, updates, and richer location/status validation.
- VFD import: ✅ Added `POST /api/projects/{projectId}/vfd/models/{modelId}/import` supporting JSON or CSV payloads with append/replace merge for command dictionaries and fault maps.
- Govt creds: ✅ Added AES-GCM encryption at rest and `POST /api/devices/govt-creds/bulk` for bulk upsert.
- Provisioning/CSV: ✅ Added `POST /api/devices/import` bulk CSV import accepting columns `imei,name,project_id,protocol_id,govt_protocol_id,govt_client_id,govt_username,govt_password`; attaches govt creds via bulk upsert and stores protocol ids on device attrs.
- Bootstrap payload: ✅ Adds DNA block (rows, virtual sensors, edge rules, metadata) and envelope required fields derived from DNA/metadata; defaults for topics/endpoints now point to the TLS listener `mqtts://iot.local:8883` (configurable via `MQTT_PUBLIC_PROTOCOL|HOST|PORT`).
- Frontend: ✅ Bulk import UI now exposes the new CSV headers (project_id/protocol_id/govt_*), client-side template download, and posts raw CSV to `/api/devices/import` with active project fallback.
- Migrations runner: ensure v3/v4 migrations are wired into the migration execution path if not already.

## 12) Operational validation (EMQX + pipeline)
- Provision: `POST /api/devices/import` with a single-row CSV (or UI bulk import); capture `client_id/username/password` + topics from the response or via `GET /api/bootstrap?imei=...`.
- Connect + publish: use `cmd/simulator/kusum_sim.go` as a smoke test: `go run -tags kusum ./cmd/simulator -mqtt mqtts://iot.local:8883 -project <project_id> -imei <imei> -protocol <protocol_id> -skip-provision` (supply creds from bootstrap/client options; ensure `infra/nginx/certs/ca.crt` is trusted locally). Publish to `channels/{project}/messages/{imei}` with envelope fields (`packet_type, project_id, protocol_id, device_id, imei, ts, msg_id`).
- Subscribe: watch `channels/{project}/commands/{imei}` with the same creds to confirm command delivery path.
- Persistence check: after publish, verify ingestion via Timescale query or reporting endpoint (e.g., telemetry rows for the device/project) to confirm rules/persistence are consuming the primary topics.
