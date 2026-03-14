# Govt MQTT Protocol Verification (KUSUMC) — rms-go/go-kusumc

## Source of truth
This verification uses the government/legacy protocol definitions under:
- `refer-rms-deploy/RMS JSON MQTT Topics MDs/*`

In particular:
- `MQTT_TOPICS.md` (topic suffixes + PumpData mapping)
- `COMMON_PARAMETERS.md`, `JSON_PARAMETERS.md`, `COMMON_FORMATS.md` (envelope + per-packet keys)

## What go-kusumc supports today (status)

### Topics
From `MQTT_TOPICS.md`, the required topic shapes are:
- `<IMEI>/heartbeat`
- `<IMEI>/pump`
- `<IMEI>/data` (PumpData)
- `<IMEI>/daq`
- `<IMEI>/ondemand` (both command + response)

Status in go-kusumc:
- Subscribes to `+/heartbeat`, `+/pump`, `+/data`, `+/daq`, `+/ondemand`.
- Keeps `channels/+/messages/+` and related topics as compatibility only.

### Identity (IMEI)
- IMEI is accepted from payload key `IMEI` or `imei`.
- IMEI is also inferred from legacy topics (`<IMEI>/...`) when missing.

### Packet type inference (critical for verification + rules)
- If `packet_type` is provided, it is normalized.
- If `packet_type` is missing, it is inferred from the legacy topic suffix:
  - `/heartbeat` → `heartbeat`
  - `/pump` → `pump`
  - `/data` → `pump` (per govt doc: PumpData)
  - `/daq` → `daq`
  - `/ondemand` → inferred as command vs response based on payload shape

### Commands (ondemand)
- Publish target for downlink commands: `<IMEI>/ondemand`.
- The backend ignores its own published ondemand commands when they are echoed back via MQTT subscriptions (prevents telemetry pollution and false response handling).

### Command responses (ondemand)
Government/legacy ondemand responses may not include `msgid`/`correlation_id`.
- When `correlation_id` (or `msgid`) exists, responses are correlated normally.
- When they are missing, go-kusumc falls back to correlating the response to the most recent outstanding command request for that device.

## Rules & automation pipeline integrity

### What rules can be defined
- Rules/automation are evaluated from cached project config (Redis config bundle) and/or DB-backed rules.
- Rules are only evaluated for packets that pass payload verification (`status == verified`) and have a resolved `project_id`.

### What must be true for the pipeline to be “clean/intact”
- Correct `project_id` resolution (from device lookup or ingress scope).
- Correct `packet_type` resolution (topic inference or payload `packet_type`).
- Payload schema must be loaded for the project (or project type) so verification is not blocking rule execution.

## Payload verification vs govt protocol

### Current verifier behavior
- Key-level verification: required keys, unknown keys, max length.
- Numeric value ranges are not enforced by default unless present in the payload schema.

### Payload schemas (CSV)
Default CSV schema path (used when DNA is not present/loaded):
- `rms-go/go-kusumc/docs/rms-payload-parameters.csv`

Important note:
- The CSV uses `scope_id` values like `PM_KUSUM_SolarPump_RMS`.
- Projects often have IDs like `pm-kusum-solar-pump-msedcl`.

Fix applied:
- When `project:<projectId>` payload schemas are missing, go-kusumc now falls back to `project:<projectType>` using the project’s `type` field.

## Project DNA vs hardcoding

### Current state
- Packet key expectations can come from:
  1) Project DNA repository (DB `project_dna` JSONB) OR
  2) CSV fallback (`docs/rms-payload-parameters.csv`)

### Recommendation
- Govt protocol should be represented as “protocol-level” or “project-type-level” schema, not duplicated per project ID.
- In the current implementation, using project `type` as the schema scope is the simplest non-invasive alignment.

## Known gaps (remaining)

1) Ondemand correlation ambiguity (protocol limitation)
- If multiple commands are outstanding for the same device, a response without correlation fields cannot be safely matched.
- Recommended mitigation: enforce “at most one outstanding ondemand command per device” at publish time, or embed `msgid` into device responses if firmware can be updated.

2) Range/threshold validation
- Govt docs define the fields, but not numeric min/max enforcement.
- Current strictness is mostly presence/unknown key checks; numeric validation is mainly driven by DB `telemetry_thresholds` (for alerts) rather than verifier rejection.

3) UI simulator alignment
- The backend now treats `/data` as PumpData (`pump`).
- If the UI simulator sends pump payloads to `/data`, it should set fields consistent with the govt docs and/or the CSV schema keys.

## Fix plan (next steps)

- Enforce single outstanding ondemand request per device (or implement a safer matching window + status filtering).
- Add explicit schema topic templates for `<IMEI>/data` if you want schema matching to be template-driven rather than suffix-driven.
- Decide whether verifier should:
  - remain “schema presence/unknown key” focused (current), or
  - incorporate threshold/range checks as hard failures.

## Code references (where to look)
- MQTT subscriptions: `rms-go/go-kusumc/internal/adapters/primary/mqtt_handler.go`
- Payload schema bundling + scope fallback: `rms-go/go-kusumc/internal/core/services/config_sync_service.go`
- Topic-based packet type inference + ondemand correlation fallback: `rms-go/go-kusumc/internal/core/services/ingestion_service.go`
