# Implementation adaptations (go-kusumc + ui-kusumc)

This is the concrete list of adaptations required after you copy code into:
- `go-kusumc/` (from `unified-go/`)
- `ui-kusumc/` (from `old-ui-copy/version-a-frontend/`)

## A) go-kusumc adaptations

### A1) MQTT ingestion must support legacy topics
Current unified-go MQTT handler subscribes to channels/devices patterns.
For KUSUMC it must ingest:
- `+/{heartbeat,pump,data,daq,ondemand}`

Implementation notes:
- Parse topic as `<imei>/<suffix>`.
- Set `packet_type` from suffix when missing.
- Ensure `imei` is always present (topic-derived).
- Normalize message ID fields:
  - accept `msgid`, `msg_id`, `MSGID` (and store as the canonical correlation key).

### A2) Legacy command/response on `<imei>/ondemand`
- Server publishes commands to `<imei>/ondemand` using legacy payload shape:
  - `{ msgid, cmd, payload }`
- Server accepts responses on `<imei>/ondemand`:
  - correlate by `msgid`
  - map legacy statuses into your stored command status set.

### A3) Device credential bundles must advertise legacy topics
When issuing credentials, publish/subscribe topics must match government protocol:
- publish topics: `<imei>/heartbeat`, `<imei>/pump`, `<imei>/data`, `<imei>/daq`
- subscribe topic: `<imei>/ondemand`

### A4) Provisioning / ACL templates in EMQX must allow legacy topics
Device ACL must allow:
- publish: `<imei>/{heartbeat,pump,data,daq,ondemand}` (ondemand used for resp)
- subscribe: `<imei>/ondemand` (for downlink)

### A5) REST compatibility layer for ui-kusumc
Because ui-kusumc is derived from old-ui-copy, ensure that go-kusumc returns:
- the expected response shapes for:
  - device list/get/update
  - commands history/status
  - device configuration queue/pending/ack (if retained)
  - credentials claim/download flows

### A6) Domain and public URL env handling
KUSUMC must support rebuild on a new domain by setting env vars only.
- Mirror the legacy Node behavior:
  - `MQTT_PUBLIC_HOST`, `MQTT_PUBLIC_TLS_PORT`, optional `MQTT_PUBLIC_TCP_PORT`
  - `MQTT_PUBLIC_WS_URL` for web tooling

In unified-go terms, you can also implement:
- `MQTT_PUBLIC_URLS` (comma-separated) as a superset knob.

### A7) Remove/disable non-KUSUMC platform features
Optional but recommended to reduce surface:
- forwarded/routed packet verification paths
- multi-project channels topic logic
- platform-only controllers

Keep only what RMS needs.

---

## B) ui-kusumc adaptations

### B1) Base URL configuration
Ensure the UI can be deployed at `kusumc.hkbase.in` and talk to the same origin:
- Prefer relative `/api/...` where possible.
- If absolute URLs exist, make them env-driven.

### B2) MQTT WebSocket URL (if simulator/features use it)
- Ensure WS/WSS endpoint is env-configurable:
  - `wss://kusumc.hkbase.in/mqtt` (default)

### B3) Payload casing + aliases
If ui-kusumc uses camelCase internally:
- keep conversion at the boundary
- accept that go-kusumc may emit legacy-compatible shapes

### B4) Remove UI pages that depend on platform-only APIs
If any pages reference:
- channels topics
- forwarded packet tooling
- platform-only admin surfaces

Either remove or gate by feature flag.

---

## C) Verification milestones

1) Device can bootstrap/claim credentials and sees correct public broker URL (kusumc.hkbase.in)
2) Device connects to EMQX and publishes `<imei>/heartbeat` → telemetry persists
3) UI sends command → server publishes `<imei>/ondemand` → device response correlates → UI shows ack
4) Credential rotation forces reconnect and continues to work
