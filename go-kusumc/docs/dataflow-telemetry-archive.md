# Telemetry dataflow (Device → EMQX → Go → Redis → Timescale → Archive → Rehydrate)

This describes the path used by `unified-go/cmd/server`.

## Strict end-to-end sequence (operational view)

0) **Provisioning + ACL (precondition)**
  - Device must exist in Postgres and must be provisioned into EMQX with an ACL that authorizes the runtime topics.
  - See: `provisioning-emqx-acl.md`.

1) **Device → EMQX (MQTT connect + authz)**
  - Device connects with `username=imei`.
  - EMQX authenticates the creds and enforces ACL on publish/subscribe.

2) **EMQX → Go (broker delivers telemetry to subscriber)**
  - The Go server connects with its bridge/service identity and subscribes to ingestion topics.
  - When a device publishes telemetry on an authorized topic, EMQX forwards it to the Go subscriber.

3) **Go ingestion parses + routes + persists**
  - Go parses JSON, requires `imei`, applies dedup lock, optionally loads project DNA by `project_id`, transforms payload, marks packet `verified` or `suspicious`.
  - The packet is enqueued for batch persistence to Timescale/Postgres.

4) **Verified-only side effects**
  - Only `verified` packets:
    - get pushed into Redis “hot” cache
    - update device shadow (`devices.shadow.reported`) and `devices.last_seen`
    - trigger rule evaluation

5) **Timescale/Postgres → Archive (CSV.gz)**
  - Archiver exports time-partitions per project/day into `telemetry_archive_{projectId}_{YYYY-MM-DD}.csv.gz`.

6) **Rehydrate (old queries)**
  - For cold ranges, archive files are restored into a temp table, then queried.

## 1) Device → EMQX
- Devices publish telemetry to MQTT topics.
- The Go server subscribes via `internal/adapters/primary/mqtt_handler.go` to:
  - `channels/+/messages/+`
  - `devices/+/telemetry`

## 2) EMQX → Go ingestion
- MQTT messages are forwarded to `internal/core/services/ingestion_service.go` via `IngestionService.ProcessPacket("mqtt", payload)`.
- HTTP ingestion path also exists:
  - `POST /api/ingest` in `cmd/server/main.go` calls `ProcessPacket("http/debug", body)`
  - it is currently *open* but wrapped with `ApiKeyMiddleware` to capture identity when present.

## 3) Ingestion pipeline (strict behavior)
In `IngestionService.ProcessPacket`:
- Parse JSON into `raw map[string]interface{}`
- Required field:
  - `imei` (string) — hard requirement; missing → error
- Dedup / idempotency:
  - uses Redis lock via `StateStore.AcquireLock("lock:"+msgid, 30s)`
  - `msgid` is taken from payload if present, otherwise generated as `${imei}-${unix}`
- Project config lookup (for verification + transforms):
  - reads `project_id` and looks up `config:project:{project_id}` via `StateStore.GetProjectConfig`
  - config is written by `internal/core/services/config_sync_service.go` (and also by project create/update paths)
- Transformation:
  - `GovaluateTransformer.Apply(raw, sensors)` produces a transformed map used as `payload`
- Quality flag:
  - `validatePacketQuality` marks packet as `verified` or `suspicious` (unknown keys vs expected sensors)

Important nuance (code truth):
- The strict unknown-key check runs against the **transformed payload** (the output of the transformer).
- If transformation succeeds, extra keys present in the raw packet may be dropped by the transformer and therefore never be evaluated by the strict checker.
- If transformation fails, the code falls back to `processed = raw`, and strict checking then evaluates the raw packet keys.

## 4) Hot path (Redis)
If `status == "verified"`:
- latest packets are pushed into Redis list:
  - key: `hot:{imei}`
  - via `internal/adapters/secondary/redis_store.go` (`LPUSH` + `LTRIM 0..49`)

## 5) Persistence (Timescale/Postgres)
The ingestion service buffers envelopes and writes in batches (for both `verified` and `suspicious`):
- buffer channel size: 5000
- flush conditions: every 1s or when batch reaches 1000
- write implementation: `PostgresRepo.SaveBatch` uses `pgx.CopyFrom` into table `telemetry`:
  - columns: `time`, `device_id`, `project_id`, `data` (JSONB)

Important nuance (code truth):
- Timescale/Postgres stores **only the transformed payload** (`telemetry.data = envelope.payload`).
- The envelope `status` is used for runtime decisions (hot cache, rule eval, shadow updates) but is not persisted into the `telemetry` table by `SaveBatch`.

Also on verified packets:
- device shadow is updated in Postgres via `PostgresRepo.UpdateDeviceShadow`:
  - sets `devices.last_seen = NOW()`
  - sets `devices.shadow.reported = <processed payload>`

## 6) Archive (cold storage)
- `internal/core/services/archiver_service.go` runs daily at 2 AM:
  - cron: `0 2 * * *`
  - archives exactly the day that is `days` old (default `ArchiveData(180)`)
- For each project (`PostgresProjectRepo.GetAllProjectsWithConfig`):
  1) query telemetry for that project/day (`PostgresRepo.ExportTelemetry`)
  2) write `telemetry_archive_{projectId}_{YYYY-MM-DD}.csv.gz` with columns:
     - `time` (RFC3339)
     - `device_id`
     - `data` (JSON)
  3) upload via storage provider

Storage provider selection happens in `cmd/server/main.go`:
- `STORAGE_TYPE=S3` → S3
- `STORAGE_TYPE=GCS` → Google Cloud Storage
- otherwise local filesystem: `./cold-storage`

## 7) Rehydrate (timecapsule query)
- `internal/core/services/analytics_service.go` treats any query with `start < now-180d` as “cold”.
- For cold queries:
  1) looks up device → projectId via `DeviceRepo.GetDeviceByIMEI`
  2) calls `Archiver.RestoreData(start,end,projectId)`
  3) restore creates a temp table and loads any matching daily archives into it
  4) query runs against the temp table via `TelemetryRepo.GetTelemetryHistoryFromTable(tempTable, ...)`

For warm/hot queries:
- queries directly from `telemetry` via `GetTelemetryHistoryFromTable("telemetry", ...)`.

## Notes
- Archive and rehydrate behavior here is based on per-project/day CSV.GZ exports and temporary-table restoration for cold-range queries.
