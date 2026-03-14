# `rms-deploy` (old) vs `unified-go` + `new-frontend` (new)

This document compares the *proven* `rms-deploy` behavior (Node/Fastify + Mongo + EMQX) with the current Go/Timescale stack, focusing on:
- strict provisioning → EMQX authN/authZ
- strict MQTT topic + payload taxonomy
- strict telemetry lifecycle (ingest → hot state → history → archive/rehydrate)
- rules/alerts nuances

It also highlights what to port into the new system to regain the same operational strictness.

---

## 1) Provisioning + EMQX ACL

### `rms-deploy` (proven)
- **Device identity**: each device gets its own MQTT username/password.
- **Provisioning mechanism**: backend calls EMQX REST APIs to:
  1) upsert built-in auth user
  2) *replace* the user’s ACL rules
- **ACL safety pattern (important)**:
  - explicit allow rules for the device’s publish topics + subscribe topics
  - **deny-all fallback**: `{ topic: '#', action: 'all', permission: 'deny' }`
  - this prevents “accidental access” if allow-rules drift.
- **Backend/service identity**: a separate MQTT service account is bootstrapped with broad wildcard permissions (subscribe to many devices; publish commands).
- **Operational detail**: provisioning is queued and retried via a job/worker model; results are recorded back onto a credential history record.

References (repo):
- `rms-deploy/backend/src/provisioning/emqx-client.ts`
- `rms-deploy/backend/src/domain/emqx-bootstrap.service.ts`
- `rms-deploy/backend/src/provisioning/mqtt-provisioning.worker.ts`

### `unified-go` (current)
- **Device identity**: per-device creds are generated on `POST /api/devices`.
- **Provisioning mechanism**: a Postgres-backed job is processed by `internal/core/workers/mqtt_worker.go`, using `internal/adapters/secondary/emqx_adapter.go`.
- **Gap**: ACL topics currently granted by the worker do **not** match the runtime topics used by ingestion/commands.

### What to port into `unified-go`
- Keep the **replace ACL** pattern.
- Add the **deny-all fallback rule** to each device ACL.
- Define one canonical topic contract (ideally `channels/...`) and ensure:
  - firmware publishes/subscribes to it
  - Studio LIVE uses it
  - Go subscribes/publishes to it
  - provisioning worker grants *exactly* those topics

---

## 2) MQTT topic contract

### `rms-deploy` (proven)
- Topic scheme is **IMEI-first + suffix**:
  - `<IMEI>/heartbeat`
  - `<IMEI>/data`
  - `<IMEI>/daq`
  - `<IMEI>/ondemand`
- Backend subscription uses wildcards by suffix: `+/{suffix}`.

References:
- `rms-deploy/RMS JSON MQTT Topics MDs/MQTT_TOPICS.md`
- `rms-deploy/backend/src/telemetry/mqtt-worker.ts`

### `unified-go` (current)
- Go ingestion subscribes to:
  - `channels/+/messages/+`
  - `devices/+/telemetry` (legacy/alt)
- Go commands publish to:
  - `channels/{project_id}/commands/{imei}`

### What to port into `unified-go`
- Port the *taxonomy* (heartbeat/data/daq/ondemand) even if you keep `channels/...` topics.
- If you keep a single ingest topic (`channels/{project_id}/messages/{imei}`), you still need a strict discriminator:
  - either a `type` field in the payload (`heartbeat|data|daq|ondemand`)
  - or a topic suffix segment (i.e. `channels/{project_id}/{suffix}/{imei}`)

---

## 3) Payload taxonomy + strict schema

### `rms-deploy` (proven)
- Ingestion validates:
  - `topic_suffix ∈ {heartbeat,data,daq,ondemand}`
  - device exists
  - (per protocol/version metadata) the suffix is allowed
  - expected key presence and unknown keys (missing/unknown sets are computed)
  - `ASN` is required on `heartbeat/data/daq`
- Storage pattern:
  - writes raw telemetry events into `telemetry_raw`
  - maintains an upserted “latest view” per device in `telemetry_views`

References:
- `rms-deploy/backend/src/telemetry/ingest.service.ts`
- `rms-deploy/RMS JSON MQTT Topics MDs/JSON_PARAMETERS.md`
- `rms-deploy/RMS JSON MQTT Topics MDs/JSON_FORMATS.md`

### `unified-go` (current)
- Ingestion expects a single JSON object and requires `imei`.
- “Strictness” is currently implemented as **unknown-key filtering** against a project’s sensor list.
- Known nuance/gap: strict allow-list uses `sensor.param`, while transform uses `sensor.id`.

### What to port into `unified-go`
- Make the taxonomy explicit:
  - reserve/whitelist `type`, `timestamp`, `msgid`, etc (so strict mode doesn’t flag them)
- Consider porting the `rms-deploy` idea of **protocol-version schema**:
  - per `projectId + protocolVersion` define allowed packet types and expected keys
  - compute missing/unknown keys for diagnostics (don’t just label “suspicious”)
- Fix the `param` vs `id` mismatch so strictness and transforms/rules use the same key.

---

## 4) Telemetry lifecycle + archiving

### `rms-deploy` (proven)
- History store: MongoDB collections (raw + views).
- Archiving: nightly `mongodump --archive --gzip` against the replica set.
  - output file: `mongo-backup-YYYYMMDD-HHmmss.archive.gz`

References:
- `rms-deploy/scripts/backup-mongo.ps1`
- `rms-deploy/scripts/linux-mongo-backup.timer`

### `unified-go` (current)
- History store: Timescale/Postgres `telemetry(time, device_id, project_id, data)`.
- Hot/latest: Redis hot list + Postgres device shadow.
- Archiving: daily CSV.GZ export per project/day; rehydrate loads archives into a temp table for cold queries.

### What to port into `unified-go`
- Conceptually, the new archive/rehydrate path is “better” than whole-DB dumps.
- The key `rms-deploy` nuance to preserve is **operational repeatability**:
  - scheduled job + retention policy
  - integrity checks (non-empty output, logs)

---

## 5) Rules / alerts / thresholds

### `rms-deploy` (proven)
- Alerting is strongly tied to:
  - **offline detection** workflow with protocol-specific thresholds and notification queueing
  - **parameter thresholds** stored per device (installation layer + overrides)
- Keys are consistently treated as `parameter` strings.

References:
- `rms-deploy/backend/src/workflows/offline-monitor.ts`
- `rms-deploy/backend/src/api/telemetry-thresholds.routes.ts`
- `rms-deploy/backend/src/telemetry/thresholds.service.ts`

### `unified-go` (current)
- Rules are evaluated server-side on **verified** packets only, against the **transformed payload**.
- Rules/virtual sensors rely on consistent payload key naming (currently impacted by `param` vs `id`).

### What to port into `unified-go`
- Treat `parameter` naming as a first-class contract:
  - rules, virtual sensors, strict verification, and UI must all reference the same key namespace.
- Consider adding the proven “offline monitor” style workflow as a separate, explicit worker (instead of only rules-on-ingest).

---

## 6) Actionable alignment checklist (new system)

1) Choose the canonical device topic family (recommend `channels/...`).
2) Update provisioning worker ACL to grant only the canonical topics + **deny-all fallback**.
3) Decide how taxonomy is expressed (payload `type` vs topic suffix) and document it.
4) Fix `sensor.param` vs `sensor.id` mismatch (strictness + transforms + rules must agree).
5) Reserve/whitelist envelope keys (`imei`, `project_id`, `timestamp`, `msgid`, `type`) so strict mode stays usable.
6) If you need “proven” alerts, port offline monitor + threshold layering as explicit services/workers.
