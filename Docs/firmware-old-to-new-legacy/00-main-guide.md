# Firmware Old -> New (Legacy) Main Guide

Purpose: remove confusion between legacy govt payload, minimal extra fields, and full envelope mode.

Audience: firmware teams migrating from old govt packets to rms-go compatibility.

## 1) Operating Modes (clear and exclusive)

### Mode A (Recommended): Legacy + Minimal Extra
- Keep legacy packet body exactly as govt format defines.
- Add only a small, stable set of extra fields.
- Backend infers and enriches the rest.

### Mode B (Supported): Legacy + Full Envelope
- Keep legacy packet body.
- Also include full envelope fields in each packet.
- Backend still validates and may override with authoritative server-side mapping when needed.

### Mode C (Not recommended for migration): Legacy-only with zero extras
- Works in many paths, but weaker dedupe, tracing, and routing certainty.
- Use only as temporary fallback.

## 2) What is Govt Packet vs Extra

### Govt packet (must preserve)
- Heartbeat/PumpData/Daq payload keys and typing as per legacy docs.
- Topic lanes: `<imei>/heartbeat`, `<imei>/data`, `<imei>/daq`.

### Minimal extra (add in Mode A)
- `msgid` (or `MSGID`) for dedupe/tracing.
- `packet_type` for explicit classification.
- One canonical event-time key: prefer `ts` (epoch ms UTC).
- `imei` if not already present as `IMEI`.

### Full envelope (Mode B)
- `packet_type`, `project_id`, `protocol_id`, `contractor_id`, `supplier_id`, `manufacturer_id`, `device_id`, `imei`, `ts`, `msg_id`.

## 3) Recommended Policy

Use Mode A as default for firmware rollout.
- Lowest firmware complexity.
- High compatibility with old devices.
- Backend enrichment keeps business context authoritative.

Use Mode B only when device provisioning and bootstrap data quality are mature and stable.

## 4) Time Rules (non-negotiable)

- Send one canonical event time key (prefer `ts` epoch ms UTC).
- Avoid sending conflicting time keys (`ts` and different `TIMESTAMP`).
- If multiple time keys are sent, they must represent the same moment.

## 5) Packet Type Rules

- Legacy topics allow backend inference.
- Still send `packet_type` explicitly to avoid ambiguity and improve downstream filtering.

## 6) Source Docs

- Legacy packet structures: `RMS JSON MQTT Topics MDs - Legacy/JSON_FORMATS.md`
- Legacy parameter dictionary: `RMS JSON MQTT Topics MDs - Legacy/JSON_PARAMETERS.md`
- Legacy topics: `RMS JSON MQTT Topics MDs - Legacy/MQTT_TOPICS.md`

See also:
- `01-what-firmware-sends.md`
- `02-minimal-contract.md`
- `03-packet-examples.md`
- `04-strict-implementation-rules.md`

## 7) Compulsory vs Recommended Fields (quick decision)

### Compulsory (minimum for ingest)
- Valid JSON payload.
- IMEI present (`IMEI`/`imei`) or inferable from topic.
- Correct topic lane: `<imei>/heartbeat`, `<imei>/data`, `<imei>/daq`.

### Strongly recommended (production quality)
- `msgid` (unique per packet).
- `ts` (UTC epoch ms).
- `packet_type` (explicit classification).

### Current backend behavior
- Govt Smallest packets are accepted even without `msgid`, `ts`, `project_id`, and `device_id`.
- Backend can enrich identity from IMEI/topic and fallback event time to server receive time when needed.

### Operational outcome
- Legacy-only mode works for migration/fallback.
- Minimal mode is the default production recommendation for stronger dedupe, tracing, and event-time analytics.

## 8) Topic patterns by toggle (legacy vs compact)

Use `MQTT_COMPAT_TOPICS_ENABLED` as the topic-family switch:

### When `MQTT_COMPAT_TOPICS_ENABLED=false` (legacy-only)
- Publish telemetry on legacy topics:
	- `<imei>/heartbeat`
	- `<imei>/data`
	- `<imei>/daq`
	- `<imei>/ondemand` (command lane)
	- `<imei>/errors`

### When `MQTT_COMPAT_TOPICS_ENABLED=true` (legacy + compact compat)
- Legacy topics remain supported.
- Compact topic families are also supported:
	- `channels/{project_id}/messages/{imei}`
	- `channels/{project_id}/messages/{imei}/{packet_suffix}`
	- `devices/{imei}/telemetry`
	- `devices/{imei}/telemetry/{packet_suffix}`
	- `devices/{imei}/errors`
	- `devices/{imei}/errors/{packet_suffix}`

`packet_suffix` examples: `heartbeat`, `data`, `daq`, `pump`, `errors`.

Operational recommendation:
- Keep one canonical publish family per fleet segment to reduce ACL/provisioning complexity.
- If dual-publish is required during migration, keep payload contract identical across both topic families.
