# Delta Report: Legacy RMS vs New Unified Go Stack

## What changed since last refresh (2026-02-18)
- Reconfirmed firmware-impacting deltas are still concentrated in MQTT taxonomy, packet typing conventions, command envelope shape, and import request-style differences.
- Updated compatibility posture to reflect that alias-first compatibility hardening is now broadly applied across list/filter APIs.
- Retained recommendation that compatibility mode and bridge mapping remain default for RMS projects.
- Added command-recovery posture: devices now have open-device HTTP fallback for recent command history when MQTT continuity is interrupted.

## Deltas that can affect compatibility

| Delta | What changed | Impact type | Device change required? | Mitigation |
|---|---|---|---|---|
| MQTT taxonomy | Selected for KUSUMC: active ingest accepts `<imei>/{heartbeat,data,daq,ondemand,errors}`. Legacy `/pump` must be mapped to `/data` before ingest. `channels/{project_id}/messages/{imei}` remains optional for bridged deployments when compatibility subscriptions are enabled. | Protocol/transport | Not if bridge exists | Publish canonical suffixes (`data` for pump telemetry) or enforce bridge mapping |
| Packet typing | Legacy infers from topic suffix; Go detects from `packet_type`, `metadata.packet_type`, or topic template | Payload semantics | Usually no | Always include `packet_type` for deterministic behavior |
| Command envelope | Legacy ondemand style uses `msgid/cmd/payload`; Go command dispatch commonly uses `command_id/correlation_id/payload/ts` internally | Command contract | Usually no (if API adapter retained) | Keep command API adapters and response correlation mapping |
| Command recovery path | Device-open family now includes `/commands/history` fallback in addition to MQTT subscribe flow | Operational resilience | No | Use HTTP fallback when MQTT reconnect window causes potential command miss |
| Import API contract | Legacy import endpoints accept JSON wrapping CSV text; Go now supports raw CSV and legacy JSON-wrapped CSV on key import routes | Admin/tooling | No for device | Keep compatibility adapters and document accepted formats |

## Value-add features added in Go/new frontend

| Feature | Legacy | New stack | Hardware impact |
|---|---|---|---|
| Studio and Builder logic authoring | No | Yes | None |
| Compiled-rules runtime + graph fallback | No | Yes | None |
| Project DNA versioning and thresholds workflow | Limited | Expanded | None |
| Digital Twin and simulation UX | No | Yes | None |
| ERP/vertical modules (logistics, healthcare, traffic, GIS, etc.) | No | Yes | None |
| Extended command catalog/admin flows | Basic | Expanded | None (unless consuming new command types) |

## Recommendation
- Keep compatibility mode as default for RMS projects:
  - preserve legacy ingress paths and command aliases
  - enforce bridge mapping for legacy MQTT topic publishers
  - freeze a firmware-facing stable contract in this folder and treat all new features as optional overlays.
- Keep parity verification (`docs/route-compare.md` + `01-parity-audit-matrix.md`) as the source for any remaining edge-case route deltas.
