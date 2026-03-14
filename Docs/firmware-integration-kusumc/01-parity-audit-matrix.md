# Phase 1: Parity Audit Matrix

## What changed since last refresh (2026-02-18)
- Added a live parity snapshot section with explicit currently listed non-parity route items from `docs/route-compare.md`.
- Recorded query/filter compatibility hardening status (camel/snake aliases, cursor aliases, pagination aliases).
- Kept the high-impact API/Payload matrix aligned with current Go compatibility behavior for telemetry, commands, and imports.
- Included open-device command-history HTTP fallback in the device-side parity surface.

## Compatibility target
- Goal: keep existing RMS hardware behavior working with near-zero firmware change.
- Success bar: preserve legacy endpoints/topic behavior via exact parity, aliases, or bridge compatibility.

## Endpoint parity (legacy Node vs Go)
- Full endpoint-by-endpoint table is maintained in `docs/route-compare.md`.
- Summary from current extraction/census:
  - Legacy extracted routes: 107
  - Go extracted routes: 382
  - Method diffs: 0 in current compare output
  - Remaining compare “missing” entries are mostly template-placeholder artifacts in legacy route definitions.

## Live parity snapshot (2026-02-18)
- Baseline source: `docs/route-compare.md`.
- Current posture: high route parity with compatibility aliases on firmware-critical APIs (telemetry, commands, imports, admin/audit/user-group scopes).

<!-- PARITY_METRICS_TABLE_START -->
| Metric | Count | Basis |
|---|---:|---|
| Legacy routes (extracted) | 107 | `docs/route-compare.md` |
| Exact matches | 103 | `Status=exact` |
| Missing routes | 4 | `Status=missing` |
| Aliased route entries | 0 | Route-compare rows (route-level aliases tracked separately in REST contract notes) |
| Extra Go routes vs legacy | 275 | `Go extracted (382) - Legacy extracted (107)` |
<!-- PARITY_METRICS_TABLE_END -->

- Explicit non-parity items still shown in route compare (classified):
  - `PATCH /admin/${basePath}/:entityId` — **non-actionable** (legacy template placeholder, not a concrete endpoint)
  - `DELETE /admin/${basePath}/:entityId` — **non-actionable** (legacy template placeholder, not a concrete endpoint)
  - `GET /devices/local` — **actionable only if legacy clients still call it** (current Go path is generic `/devices` list + filters)
  - `GET /devices/government` — **actionable only if legacy clients still call it** (current Go path is generic `/devices` list + filters)
- Query/filter compatibility hardening status:
  - camelCase/snake_case alias support is broadly implemented across list/search endpoints,
  - cursor aliases (`cursor`/`afterId`/`after_id`) are standardized,
  - pagination aliases (`page`/`pageNumber`, `limit`/`pageSize`) are implemented in high-traffic list endpoints.

## API/Payload matrix (high-impact)

| Domain | Legacy Node (refer-rms-deploy) | New Go (unified-go) | Parity Status | Hardware/Firmware Impact |
|---|---|---|---|---|
| Device-open bootstrap/creds/vfd/history | `/api/devices/open/*` | Canonical: `/api/device-open/*` and `/api/v1/device-open/*` (legacy aliases supported) | Mirrored | None when using canonical paths |
| Device CRUD and status | Present | Present with aliases/trailing-slash compatibility | Mirrored | None |
| Device commands API | Present | Present (`/api/commands/send`, `/api/devices/:device_uuid/commands`) | Mirrored with payload differences | Low |
| Telemetry HTTPS ingest | Legacy telemetry ingest + mirror conventions | `/api/telemetry/:topic_suffix`, `/api/telemetry/mirror/:topic_suffix` | Mirrored | Low |
| Telemetry MQTT ingest | Topic suffix-centric (`<imei>/<suffix>`) | Canonical channels + payload packet type detection | Bridged / Different | Medium if no bridge |
| Device CSV import | JSON payload containing CSV text + import jobs | Implemented; some paths parse raw CSV body and job APIs available | Partial behavioral delta | Medium for admin tooling, not device |
| Govt credentials import | CSV import + dedicated import job listing | Bulk upsert import endpoint present with import job tracking and dedicated listing route aliases | Mirrored | Low device impact |

## Current status check answers

1) Minimal hardware change requirement understood?
- Yes. This documentation assumes legacy device contracts should keep working through aliases/bridge/config before requiring firmware rewrites.

2) Major deltas identified?
- Yes. Main deltas are MQTT topic taxonomy, command payload envelope shape, and CSV import request-style differences.

3) Are CSV bulk import features fully implemented?
- Largely yes for RMS parity-critical paths. Go now supports legacy-style JSON-wrapped CSV and raw CSV for device/config/government credential imports, plus dedicated government import job listing.
