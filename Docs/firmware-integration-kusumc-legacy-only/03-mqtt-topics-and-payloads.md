# MQTT Topics and Payloads (RMS Compatibility)

## What changed since last refresh (2026-02-18)
- Confirmed forwarded telemetry compatibility normalization remains active for legacy forwarders.
- Revalidated suspicious-payload behavior when origin identity cannot be recovered.
- Kept canonical packet typing precedence and topic mapping aligned with current ingestion behavior.
- Added command continuity note for device-side HTTP fallback (`/device-open/commands/history`) when MQTT delivery continuity is uncertain.

## Scope
This document explains:
- legacy topic behavior and payload contract
- payload contract for direct self telemetry
- payload contract for forwarded telemetry from other nodes with routing metadata.

## 1) Topic model (selected for KUSUMC)

### Legacy Node model (frozen govt firmware)
- Historical device publish pattern used `<imei>/<suffix>` with suffixes including `pump`.
- Canonical firmware publish suffix set is:
  - `heartbeat`, `data`, `daq`, `ondemand`
- Packet type is strongly tied to topic suffix.

Compatibility note:
- Backend may still accept legacy `<imei>/pump` traffic for backward compatibility, but firmware contract remains PumpData on `<imei>/data`.

### go-kusumc model
- Ingest topic patterns (legacy-only for firmware):
  - `<imei>/heartbeat`
  - `<imei>/data`
  - `<imei>/daq`
  - `<imei>/ondemand`
  - `<imei>/errors` (device errors / offline-rule alerts)
- Packet type detection order:
  1. payload `packet_type`
  2. payload `metadata.packet_type`
  3. packet schema topic-template match (TopicTemplate like `<IMEI>/data`)

## 2) Mandatory envelope fields (device-facing)
- Device identity:
  - Prefer sending `IMEI` (legacy) or `imei` (canonical).
  - If omitted, go-kusumc will derive IMEI from the topic prefix (`<imei>/...`).
- Correlation/dedup:
  - `msgid` is strongly recommended (and required for any command/response correlation).
- Classification:
  - `packet_type` is recommended when topic suffix does not uniquely encode intent (especially `ondemand`).
- `project_id` is optional for legacy firmware; go-kusumc will resolve project via device lookup.

## 3) Self data (direct telemetry from same device)

Diagram:

![](diagrams/03-telemetry-self.flowchart.svg)

### Self telemetry definition
Self data means the payload values represent the publishing device itself (same IMEI as source and measurement origin).

### Canonical topics
- Heartbeat: `<imei>/heartbeat`
- PumpData: `<imei>/data`
- DAQ: `<imei>/daq`

Note:
- Govt protocol documents `/data` as PumpData. Use `/data` as the canonical publish topic for pump telemetry.

### Example: self heartbeat (govt legacy shape)
```json
{
  "IMEI": "356000000000001",
  "TIMESTAMP": "2026-02-24T00:00:00Z",
  "ASN": "ASN-001",
  "RSSI": "-67",
  "TEMP": "31.2"
}
```

## 4) Forwarded data (RMS gateway picking from other nodes and forwarding)

Diagram:

![](diagrams/04-telemetry-forwarded.flowchart.svg)

### Forwarded telemetry definition
Forwarded data means a gateway/RMS node publishes readings that originated from other nodes.

### Is this clear in current code?
- Yes. Forwarded telemetry is now explicitly validated in ingestion.
- For forwarded packets (`packet_type=forwarded_data` or `metadata.forwarded=true`), the following are required:
  - `metadata.forwarded=true`
  - `metadata.origin_imei` **or** `metadata.origin_node_id`
  - `metadata.route.path` (non-empty string array)
  - `metadata.route.hops` (non-negative integer)
  - `metadata.route.ingress` (non-empty string)

### Compatibility normalization (implemented)
- Legacy forwarded packets that are missing some route metadata are now auto-normalized before strict validation.
- Important: "strict validation" in this stack primarily determines whether a packet is marked `verified` vs `suspicious` (persisted with diagnostics). Hard rejects are reserved for cases like invalid JSON or missing device identity (`imei`) that prevent correlation/persistence.
- Normalization behavior:
  - copies `packet_type`/`metadata` from raw payload when transform output drops them,
  - enforces `metadata.forwarded=true` for forwarded packets,
  - derives `metadata.origin_imei` or `metadata.origin_node_id` from fallback keys when possible,
  - defaults `metadata.route.path`, `metadata.route.hops`, and `metadata.route.ingress` when absent.
- This keeps legacy forwarders compatible while still enforcing a strict canonical shape after normalization.

### Suspicious fallback behavior (implemented)
- If normalization still cannot recover required forwarded identity fields (especially `metadata.origin_imei` or `metadata.origin_node_id`), the payload is ingested but marked `suspicious`.
- In that case, validation includes missing-field diagnostics (for example `metadata.origin_imei`) for operator troubleshooting.

Example that becomes `suspicious` (origin cannot be recovered):
```json
{
  "imei": "356000000000999",
  "project_id": "proj_alpha",
  "msgid": "fwd-broken-001",
  "packet_type": "forwarded_data",
  "metadata": {
    "forwarded": true,
    "route": {
      "path": ["gateway-99"],
      "hops": 0,
      "ingress": "mesh/lora"
    }
  }
}
```

### Troubleshooting quick map

| Symptom | Validation key | Likely cause | Firmware/edge fix |
|---|---|---|---|
| Forwarded packet marked suspicious | `metadata.origin_imei|metadata.origin_node_id` | Origin ID omitted and no fallback key | Populate `metadata.origin_node_id` (preferred) or `metadata.origin_imei` |
| Forwarded packet marked suspicious | `metadata.route.path` | Route path missing/empty | Send non-empty hop path array |
| Forwarded packet marked suspicious | `metadata.route.hops` | Hops missing/non-integer | Send non-negative integer hop count |
| Forwarded packet marked suspicious | `metadata.route.ingress` | Ingress not provided | Provide source transport label (e.g. `mesh/lora`) |
| Packet suspicious due to unknown fields | `unknown` list in validation | Non-whitelisted telemetry keys in strict mode | Move route info under `metadata.*` and align project schema |

### Recommended forward topic
- Publish using gateway IMEI path to keep auth simple:
  - `<gateway_imei>/data`
- Include origin node identity and route metadata inside payload.

### Recommended forwarded payload contract
Required fields:
- `imei`: gateway IMEI (publisher identity)
- `project_id`
- `msgid`
- `packet_type`: use value such as `forwarded_data` (or source packet type if you prefer)
- `metadata.forwarded`: `true`
- `metadata.origin_imei`: source node IMEI
- `metadata.route`: object with routing path details

Recommended optional routing metadata:
- `metadata.route.path`: array of hop IDs
- `metadata.route.hops`: integer hop count
- `metadata.route.ingress`: source protocol/topic label
- `metadata.route.received_at`: time seen by gateway

### Example: forwarded payload with routing info
```json
{
  "imei": "356000000000999",
  "project_id": "proj_alpha",
  "msgid": "fwd-1730000012-41",
  "packet_type": "forwarded_data",
  "timestamp": 1730000012000,
  "TEMP": 29.8,
  "FLOW": 12.1,
  "metadata": {
    "forwarded": true,
    "origin_imei": "356000000000111",
    "origin_node_id": "field-node-111",
    "route": {
      "path": ["field-node-111", "repeater-07", "gateway-999"],
      "hops": 2,
      "ingress": "mesh/lora",
      "received_at": "2026-02-17T11:05:30Z"
    }
  }
}
```

## 5) Compatibility guardrails for forwarded payloads
- Keep measurement keys at top level if strict verification and transforms depend on top-level sensor mappings.
- Reserve all forwarding/routing data under `metadata.*` to avoid colliding with sensor keys.
- Add `packet_type` and whitelist metadata keys in project schema if strict unknown-key checks are enabled.

## 6) Command topics (server to device and response)
- Command publish topic (server -> device):
  - `<imei>/ondemand`
- Response topic (device -> server):
  - `<imei>/ondemand` (same topic)
- Important: because MQTT brokers can echo publishes back to the same client subscription, devices should ignore their own publishes (for example: only treat inbound packets with `type=ondemand_cmd` as commands).
- Correlation keys handled in ingestion flow:
  - `correlation_id`, fallback to `msgid`.
- Command continuity fallback:
  - Device can fetch recent command backlog via `GET /api/device-open/commands/history?imei={imei}&limit={n}` (legacy and `/v1` aliases available).

### Command payload contract (server -> device)
Commands are published on `<imei>/ondemand` as JSON.

Govt legacy OnDemandCommand shape:
```json
{
  "msgid": "a7b9c9f5-8b26-4d4c-9a0f-82d9d0f68f3a",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "set_ping_interval_sec",
  "interval_sec": 60
}
```

Default command set (seeded as `scope=core`):
- `reboot`
- `rebootstrap`
- `set_ping_interval_sec` (payload: `{ "interval_sec": int }`)
- `send_immediate`
- `apply_device_configuration` (payload: `{ "config_id": "<uuid>", "config": { ... } }`)

Configuration apply correlation rules:
- The server uses `config_id` as the command correlation identifier and publishes it as `msgid` on `<imei>/ondemand`.
- Device responses should include `msgid` equal to the command `msgid` (recommended). If supported, also include `correlation_id=config_id`.

### Ack/response payload contract (device -> server)
Govt legacy OnDemandResponse shape uses `status` and may omit correlation fields.

Strong recommendation (for deterministic correlation):
- Include `msgid` in the response and keep it equal to the command `msgid`.

Publish topic:
- Publish the response JSON on `<imei>/ondemand`.

Standard codes:
- `code=0` accepted/acked
- `code=1` failed
- `code=2` wait (used by `send_immediate` when the next periodic publish is ≤30s away)

Example (ack):
```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "DO1": 0,
  "PRUNST1": "1",
  "msgid": "a7b9c9f5-8b26-4d4c-9a0f-82d9d0f68f3a"
}
```

## 7) Device errors / offline-rule alerts (device → server)
Errors MUST NOT be mixed into heartbeat and MUST NOT be overloaded onto `ondemand`.

Topic:
- `<imei>/errors`

Schema:
- `open_id`: device UUID from bootstrap identity (recommended)
- `timestamp`: ms epoch (number) or RFC3339 (string)
- `error_code`: stable enum string (see `for-firmware-agent/10-error-codes.md`)
- `error_data`: dynamic JSON object

Example:
```json
{
  "open_id": "dev-9c6b8de2",
  "timestamp": 1760870400456,
  "error_code": "RS485_CRC_ERROR",
  "error_data": {
    "bus": "rs485",
    "slave_id": 1,
    "count": 12,
    "window_sec": 300
  },
  "severity": "warning"
}
```

## 8) Practical deployment mode (KUSUMC)
- Legacy firmware unchanged (selected): publish on `<imei>/{heartbeat,data,daq,ondemand}` with PumpData on `<imei>/data`, and rely on go-kusumc legacy subscriptions.
