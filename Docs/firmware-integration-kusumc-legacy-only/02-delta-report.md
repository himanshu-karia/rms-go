# Delta Report: Legacy RMS vs New Unified Go Stack

NOTE: This document is **platform/server migration context** and is **not required** for legacy-only firmware development.
Firmware developers should use `for-firmware-agent/`.

## What changed since last refresh (2026-02-18)
- Reconfirmed firmware-impacting deltas are still concentrated in MQTT taxonomy, packet typing conventions, command envelope shape, and import request-style differences.
- Updated compatibility posture to reflect that alias-first compatibility hardening is now broadly applied across list/filter APIs.
- Updated recommendation: keep firmware guidance legacy-only; treat any non-legacy topic support as optional server-side capability.
- Added command-recovery posture: devices now have open-device HTTP fallback for recent command history when MQTT continuity is interrupted.

## Deltas that can affect compatibility

| Delta | What changed | Impact type | Device change required? | Mitigation |
|---|---|---|---|---|
| MQTT taxonomy | KUSUMC uses legacy suffix topics with PumpData standardized on data lane: `<imei>/{heartbeat,data,daq,ondemand}` (historical `<imei>/pump` may still be accepted for compatibility). | Protocol/transport | No | Use legacy topics only (PumpData on `<imei>/data`) |
| Packet typing | Legacy infers from topic suffix; Go detects from `packet_type`, `metadata.packet_type`, or topic template | Payload semantics | Usually no | Always include `packet_type` for deterministic behavior |
| Command envelope | Ondemand is published/consumed in govt legacy shape (`msgid`, `timestamp`, `type`, `cmd`, params at top-level). | Command contract | No | Keep strict govt payload shapes |
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
- Freeze a firmware-facing stable contract in this folder (legacy-only topics + legacy payloads).
- Treat all server-side compatibility/topic parallelism as optional overlays (not required for KUSUMC firmware).
- Keep parity verification (`docs/route-compare.md` + `01-parity-audit-matrix.md`) as the source for any remaining edge-case route deltas.
