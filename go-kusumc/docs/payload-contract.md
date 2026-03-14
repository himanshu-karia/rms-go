# Payload contract + taxonomy

This is the strict contract enforced by `internal/core/services/ingestion_service.go` (the code path wired in `cmd/server/main.go`).

This document also defines a practical **packet taxonomy** you can standardize across firmware projects (heartbeat, DAQ, command/response, project-specific payloads) while staying compatible with the current ingestion code.

## 1) Required envelope fields
- `imei` (string) — required; ingestion rejects packets without it

## 2) Optional but practically required fields
- `project_id` (string)
  - without it, the server cannot load the project DNA (`config:project:{project_id}`), so transforms and strict key verification won’t run.
- `msgid` (string)
  - if present: used for dedup lock key
  - if absent: server synthesizes `${imei}-${unix}`
- `device_id` (string)
  - passed through into the DB row (`telemetry.device_id`) and used by some rule paths

## 3) Sensor/value placement (MOST IMPORTANT)
### What ingestion expects
`IngestionService.ProcessPacket` parses one JSON object and expects sensor values to be present at the **top level** of that JSON object.

Example (DAQ-style):
```json
{
  "imei": "356...",
  "project_id": "proj_123",
  "msgid": "m-001",
  "temp": 25.2,
  "hum": 60,
  "batt": 3.9,
  "timestamp": 1730000000000
}
```

### What is stored
The DB row stores `payload` (post-transform) into `telemetry.data`.

## 4) Packet quality / strict unknown-key behavior
If a project config is available, `validatePacketQuality` rejects packets as `suspicious` when they contain keys not listed in the project’s sensor list.

Caveat: there is an internal schema mismatch today:
- `validatePacketQuality` builds the “allowed set” using sensor field `param`
- `GovaluateTransformer` reads raw values using sensor field `id`

So “strict correctness” depends on whether your project’s `hardware.sensors[]` uses `id` == incoming key, or `param` == incoming key.

If you want strictness:
- pick **one** key (`param` is the intent in `internal/engine/config.go`) and make both transformer + verifier use that same key.

Additional nuance (code truth):
- Unknown-key strictness is evaluated against the **transformed payload** (the transformer output).
- If transform succeeds, extra fields that exist only in the raw packet may be dropped and never checked.
- If transform fails, the code falls back to `processed = raw`, and strict checking evaluates raw keys.

## 5) Command / response packets
### Server → device command topic
- `channels/{project_id}/commands/{imei}`

### Command payload shape
Used by `internal/core/services/commands_service.go` and `shadow_service.go`:
```json
{
  "msgid": "uuid",
  "cmd": "SOME_COMMAND",
  "params": {"k": "v"},
  "ts": 1730000000000
}
```

### Device → server command responses
A dedicated “response” topic is not implemented in the Go server today.
If you need strict request/response, define a response topic (e.g. `channels/{project_id}/responses/{imei}`) and subscribe/ingest it.

## 6) Common packet taxonomy (recommended)

The backend ingests “one JSON object with top-level keys”. It does not currently branch on a `type` field.

To keep strict mode compatible, treat the taxonomy as a **convention** and either:
- ensure taxonomy keys are allowed by the strict checker (reserved/whitelisted), or
- ensure transforms drop taxonomy metadata keys, or
- relax strict checking for these metadata keys.

Below are recommended packet types and example payloads.

### A) Common: Heartbeat (device health / ping)

Goal: low-frequency health + routing context.

Topic (recommended):
- `channels/{project_id}/messages/{imei}`

Example:
```json
{
  "imei": "356000000000001",
  "project_id": "project_04_tank",
  "msgid": "hb-1730000000-0001",
  "timestamp": 1730000000000,

  "uptime_s": 123456,
  "fw": "1.2.3",
  "vbat": 3.92,
  "rssi": -67,
  "ip": "10.0.0.12",
  "boot_reason": "watchdog"
}
```

Notes:
- If strict checking is enabled and your project DNA does not include these keys as sensors, they will be marked `suspicious` unless allowed/whitelisted.

### B) Common: Pump/status (actuator + controller state)

`rms-deploy` treats `pump` as a first-class packet type (separate from DAQ). If you need that same separation in this stack, standardize a `type` discriminator (or move to a topic suffix scheme) and reserve/whitelist it for strict mode.

Topic (recommended):
- `channels/{project_id}/messages/{imei}`

Example:
```json
{
  "imei": "356000000000001",
  "project_id": "project_04_tank",
  "msgid": "pump-1730000000-0009",
  "timestamp": 1730000000000,

  "pump_on": 1,
  "pump_rpm": 1450,
  "vfd_fault": 0
}
```

### C) Common: DAQ (raw I/O status for transforms + control state)

Goal: publish raw analog/digital values (and actuator state) that virtual sensors or transformations can consume.

Topic (recommended):
- `channels/{project_id}/messages/{imei}`

Example:
```json
{
  "imei": "356000000000001",
  "project_id": "project_04_tank",
  "msgid": "daq-1730000001-0042",
  "timestamp": 1730000001000,

  "ai_0": 512,
  "ai_1": 678,
  "di_0": 1,
  "di_1": 0,
  "relay_0": 1,
  "mosfet_0": 0
}
```

Notes:
- For strictness, make sure the project DNA sensor list maps the incoming DAQ keys consistently (see `param` vs `id` mismatch above).

### D) Common but variable: Command-triggered packets (on-demand) + responses

Goal: support ad-hoc diagnostics and “read now” telemetry bursts.

1) **Server → device command** (already implemented):
- topic: `channels/{project_id}/commands/{imei}`
- example:
```json
{
  "msgid": "cmd-uuid",
  "cmd": "READ_REGISTERS",
  "params": {"addr": 1000, "len": 8},
  "ts": 1730000002000
}
```

2) **Device → server response** (recommended convention; not implemented by server today):
- topic: `channels/{project_id}/responses/{imei}`
- example:
```json
{
  "imei": "356000000000001",
  "project_id": "project_04_tank",
  "msgid": "cmd-uuid",
  "timestamp": 1730000002500,
  "ok": true,
  "result": {"regs": [12, 34, 56, 78]}
}
```

If you do not add a response topic, the device can still publish the “response” as a normal telemetry packet on the ingestion topic; in that case, treat it as a project-specific packet (below).

### E) Project-specific packet(s)

Goal: allow per-vertical/per-project payloads (e.g., sensor blocks acquired over RS485/Modbus/CAN, or composite sensor data).

Topic (recommended):
- `channels/{project_id}/messages/{imei}`

Example (single “data” block):
```json
{
  "imei": "356000000000001",
  "project_id": "project_04_tank",
  "msgid": "data-1730000003-0007",
  "timestamp": 1730000003000,

  "data": {
    "ph": 7.12,
    "tds": 412,
    "flow_lpm": 15.8
  }
}
```

Strictness note:
- The current Go ingestion expects sensor values at the **top level** for transforms and rule evaluation.
- If you nest under `data`, you must either:
  - adjust transforms/rules to look under `data`, or
  - flatten keys at the device (recommended for current code), or
  - introduce a transform step that flattens `data.*` into top-level keys before strict checking/rules.

## 7) Taxonomy strictness recommendations
For strict taxonomy control, encode packet class with either topic suffix (`<IMEI>/{heartbeat|data|daq|ondemand}`) or a reserved payload field.

When staying with `channels/{project_id}/messages/{imei}`:
- introduce a reserved `type` field with values: `heartbeat|data|daq|ondemand`
- whitelist reserved envelope/taxonomy keys (`type`, `timestamp`, `msgid`, etc.) so strict checking doesn’t flag them
- consider moving from “unknown keys only” to also computing `missingKeys` / `unknownKeys` per packet type (for operator diagnostics)

## 8) Heartbeat vs DAQ vs project packets (summary)
The backend does not currently branch on a `type` field for ingestion; it treats all packets as one shape.
If you introduce a `type`/`kind` field, ensure it is allowed by strict checking, otherwise packets may become `suspicious`.

## 9) ChirpStack uplinks (northbound)
`internal/adapters/primary/http/northbound.go` normalizes a ChirpStack uplink into:
- `imei = DevEUI`
- `payload = {"_raw": <base64>}`

This does **not** match the ingestion contract above (sensor values at top level) and may be marked `suspicious` unless the project DNA explicitly expects a key named `payload`.

## 10) What gets stored in Timescale telemetry

In the current Go implementation:
- The Timescale/Postgres table `telemetry.data` stores the **transformed payload map**.
- The ingestion envelope also tracks `status` (`verified`/`suspicious`) for runtime behavior, but `status` is not written into the `telemetry` table by `SaveBatch`.

## 11) Telemetry query/export API contract

These are the HTTP APIs used by the admin UI for history browsing and report exports.

### A) Project history (paged)
`GET /api/telemetry/history/project`

Query params:
- `projectId` (or `project_id`) — required
- `from`, `to` — optional time bounds (RFC3339 or unix ms); default is last 24h
- `page`, `limit` — optional; defaults apply
- `packet_type` (or `packetType`) — optional
- `quality` — optional
- `exclude_quality` (or `excludeQuality`) — optional

Response (JSON):
```json
{
  "data": [{"time":"2026-01-06T00:00:00Z","device_id":"...","project_id":"...","data":{}}],
  "total": 0,
  "page": 1,
  "pages": 1
}
```

### B) Export telemetry (CSV/XLSX/PDF)
`GET /api/telemetry/export`

Query params:
- `format` — `csv` (default) | `xlsx` | `pdf`
- `imei` — export for a specific device (accepts IMEI or `device_id` string)
- `projectId` — export for a project (required if `imei` not provided)
- `from`, `to` — optional time bounds (RFC3339 or unix ms); default is last 24h
- `packet_type` (or `packetType`) — optional
- `quality` — optional
- `exclude_quality` (or `excludeQuality`) — optional

Output:
- `format=csv`: `text/csv`, attachment; columns: `time,device_id,data` (where `data` is JSON)
- `format=xlsx`: XLSX attachment
- `format=pdf`: PDF attachment
