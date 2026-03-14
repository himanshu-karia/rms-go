# Rules, automation, and virtual sensors

This document describes rule and transform behavior in the server runtime wired by `cmd/server/main.go`.

## 1) Virtual sensors (wired)
In the active ingestion path (`internal/core/services/ingestion_service.go`):
- transformation runs via `internal/core/services/transformer.go` (`GovaluateTransformer`)
- `transformMode == "virtual"` evaluates `sensor.expression` using `govaluate` with the full raw packet as the variable map
- the transformed payload becomes the stored `telemetry.data` and the basis for rule evaluation

Important: the transformer currently keys off sensor field `id` (not `param`).

Nuances (code truth):
- Rule evaluation and hot-cache/shadow updates only run for packets marked `status == "verified"`.
- Rule evaluation is invoked with the **transformed payload map** (not the full envelope), so rule triggers must reference keys that exist in the transformed payload.
- The transformed payload produced by `GovaluateTransformer` is currently a map of computed sensor values keyed by `sensor.id`.

## 2) Automation flows (wired)
### Storage
Automation flows are stored in Postgres table `automation_flows` (created/queried via `PostgresRepo.CreateAutomationFlow` / `GetAutomationFlow`).
- the flow shape is a ReactFlow-like graph: `{ nodes: [...], edges: [...] }`

### Evaluation
On every verified packet, `IngestionService` calls `DeviceService.EvaluateRules(projectId, processedPayload)`.
- `DeviceService` loads the flow from DB
- it evaluates the graph via `internal/engine/rules_engine.go` (a compiler-like traversal):
  - TRIGGER → CONDITION → ACTION
  - expression is built as: `<field> <operator> <value>` (e.g. `temp > 50`)
  - evaluation runs against the *payload map* (so `field` must match a key in the payload)

Nuance (payload key matching):
- If your project DNA defines sensors with `id != param`, you must be explicit about which one rules should use.
- Today, transforms populate keys using `sensor.id`, while strict verification builds its allow-list using `sensor.param` (see `payload-contract.md`).
- For “strict + predictable” rule behavior, unify on one identifier and use it consistently across:
  - device payload keys
  - strict allow-list
  - transformer raw lookup and output keys
  - rule trigger field names

### Actions
In the `DeviceService` path, actions are currently only logged (`fmt.Printf`).
If you want “real” actions (MQTT commands, DB alerts), this is the seam to implement them.

Nuance (execution location):
- The current Go backend implements **server-side** rule evaluation.
- Any “edge rule” / `executionLocation` concept in the UI is not implemented as a deployed device rule engine in this repo.

## 3) Rules table + alerting (wired)
Separately, `internal/core/services/rules_service.go` supports rules stored in Postgres via a `RulesRepository`:
- rule trigger is a free-form govaluate expression string like: `temp > 30 && batt < 20`
- rules are fetched per project and evaluated against the payload
- when triggered:
  - publishes an alert to: `channels/{project_id}/alerts`
  - if `severity == "critical"`, auto-creates a work order via `CreateWorkOrder`

This is exposed over HTTP in `cmd/server/main.go` under protected routes:
- `GET /api/rules`
- `POST /api/rules`
- `DELETE /api/rules/:id`

Nuance (UI schema drift risk):
- The frontend Rules UI includes optional fields like advanced `trigger.formula` and `executionLocation`.
- Ensure the backend rule repository/controller either persists these fields intentionally or rejects them explicitly, otherwise the UI can drift from actual runtime semantics.

## 4) Engine/pipeline components (reference)
The repo also contains a separate ingestion/engine stack:
- `internal/mqtt/handler.go` subscribes to `telemetry/+`
- `internal/pipeline/worker_pool.go` does:
  - Redis-based dedup
  - project config load via `internal/engine/loader.go` (reads `config:project:{projectId}`)
  - payload verification via `internal/engine/payload_verifier.go`
  - transforms via `internal/engine/virtual_sensor.go`
  - rules via `internal/engine/rules.go` + action execution via `internal/engine/automation.go`

This stack depends on `internal/repository.Rdb` initialization and is separate from the server runtime path documented above.

## 5) MQTT topics used for rule outputs
- Alerts (server → broker): `channels/{project_id}/alerts`
- Commands (server → broker): `channels/{project_id}/commands/{imei}`

Devices must be ACL-authorized to subscribe to the command topic they should receive.

## 5a) Automation action schema (UI → backend contract)
Automation flow actions should emit one of the following shapes:

### Alert action
```json
{
  "type": "ALERT",
  "payload": {
    "message": "High temperature",
    "severity": "warning"
  }
}
```

### MQTT command action
```json
{
  "type": "MQTT_COMMAND",
  "payload": {
    "topic": "channels/{project_id}/commands/{imei}",
    "payload": {
      "cmd": "set_vfd",
      "params": {"speed": 42}
    }
  }
}
```

Notes:
- `type` is case-insensitive in the backend.
- If `topic` is omitted, the backend defaults to `channels/{project_id}/commands/{imei}` when possible.

## 6) How rules relate to the payload taxonomy

To make rules reliable across “Heartbeat / DAQ / project-specific packets”:
- Decide which packet types are expected to trigger rules.
- Ensure the packet’s keys that rules reference exist in the **transformed payload**.
- If you want heartbeat packets to trigger rules, either:
  - model heartbeat fields as sensors (so they appear in transformed payload), or
  - change rule evaluation to run against the raw packet/envelope rather than only transformed sensor outputs.

## Alerts/threshold patterns (reference)
An alternate stack’s “rules” implementation is primarily:
- an explicit **offline monitor** workflow with protocol-specific `thresholdMs` and queued notifications
- **telemetry threshold** configuration keyed by `parameter` strings (installation layer + per-device overrides)

For comparable behavior in this runtime, keep the key-namespace contract tight:
- pick one identifier for sensor keys (the current `param` vs `id` mismatch must be resolved)
- ensure the Rules UI and any threshold/alert config refer to that same identifier

See: `rms-deploy-comparison.md` for architecture reference details.

## 7) Config cache distribution
- `ConfigSyncService` now publishes three Redis documents per project: `config:project:{projectId}` (full DNA slice), `config:rules:{projectId}` (Govaluate rules array), and `config:automation:{projectId}` (ReactFlow graph). Empty sets are written as `[]` or `null` so consumers skip DB round trips.
- The sync service reads canonical data from `PostgresRepo` (automation flow, rules) after it finishes the payload document and writes the cache with no TTL (Redis keeps the latest publish).
- Targeted invalidation runs through `SyncProject(projectId)` and is triggered automatically on DNA writes as well as rule create/update calls (`RulesService` falls back to a full sync if the payload lacks a project id).
- `DeviceService` now resolves automation flows from Redis first, falling back to Postgres only on cache miss, and repopulates the cache when it has to visit the DB.
- `RulesService` now hydrates its evaluations from `config:rules:{projectId}` when present, populating the cache itself when it falls back to Postgres.
- The MQTT/pipeline loaders continue to respect the same keys; they benefit from the populated rule cache (`config:rules:*`) without further changes.
