# Project DNA Unification Plan

_Date: 26-12-2025_

## Objectives
- Replace the CSV-only authoring flow with a unified, database-backed Project DNA store.
- Provide an enterprise-grade admin UI (sidebar-governed) for schema, rules, and automations, with auto-rendered JSON for audit/export.
- Reuse the same APIs in the high-fidelity Studio so power users get richer UX without duplicating business logic.
- Preserve runtime behaviour (Redis payloads, ingestion validation, rules engine, device edge rules) while enabling reverification and version control.

## Architecture Overview

### Canonical Data Model
- Create dedicated tables (or expanded JSON columns) to persist:
  - Packet schemas (packet type, scope, variable metadata, envelope flags).
  - Edge rule definitions (thresholds for device-side enforcement).
  - Server rule graphs (existing ReactFlow payload) and compiled expressions.
  - Virtual sensors and automation flows.
- Maintain CSV import/export tooling for audits, but treat the DB as the authority.

### Config Synchronisation
- Extend `ConfigSyncService` to read the canonical model, derive grouped `payloadSchemas` and companion structures, and push them to Redis unchanged.
- Allow manual/draft publish cycles: operators can stage changes, then commit to Redis.
- Keep existing CSV loader as a bootstrap path but warn when diverging from UI edits.

### Runtime Consumers
- `ingestion_service` and `engine` already read `payloadSchemas`; ensure new fields (edge rules, version tags) have matching structs/tests.
- Provide a reverification background job to reclassify historical telemetry once new keys become part of DNA.
- Edge devices fetch their rules via a unified `/api/device/config` (backed by the canonical store).

## UI / UX Strategy

### Sidebar-Governed Admin Portal
- Add a **Project DNA** section to the main enterprise UI (new-frontend) that includes:
  - **Packet Designer**: scope selector, packet type tabs, variable rows (param, label, unit, min/max, resolution, required, notes, envelope flag). Inline validation ensures `param` aligns with ingestion expectations.
  - **Edge Rules**: simple threshold builder mapped to device-friendly expressions.
  - **Server Rules & Automations**: integrate existing ReactFlow composer; show compiled preview before publish.
  - **Virtual Sensors**: expression editor reusing transformer syntax with live previews.
  - **JSON Preview Panel**: auto-updated read-only card with the exact JSON that will be saved (copy-friendly, version diff support).
  - **Versioning Controls**: draft/publish buttons, change history, rollback options.
  - **Export Actions**: CSV and JSON export for offline planning.

### Studio UX (High Fidelity)
- Studio reuses the same backend APIs but presents higher fidelity interactions:
  - Packet designer embeds blueprint diagrams, quick-add templates, and dependency visualisation.
  - Rules canvas continues as ReactFlow with advanced features (branching, simulation) while still saving through the same REST endpoints.
  - Keep the Studio as the only place for complex automation design, reducing redundancy.

### Redundancy & Navigation
- The sidebar-driven portal remains the single enterprise hub. Sections include Overview, Devices, Project DNA, Automations, Analytics, etc.
- Studio is nested under **Project DNA** (e.g., `Project DNA → Studio`) to signal it is an advanced view of the same data; avoid separate, conflicting forms elsewhere.
- Any legacy forms duplicating sensor config in other modules should be retired or wired to the same APIs to prevent drift.

## Implementation Phases

1. **Data Layer & Migration**
   - Design tables, backfill from `docs/rms-payload-parameters.csv`, and expose read APIs.
   - Run config sync in dual mode (CSV + DB) to confirm identical Redis payloads before switching to DB.
2. **Backend APIs**
   - CRUD endpoints for packet schemas, rules, virtual sensors, versioning, reverify job trigger.
   - Update `ConfigSyncService`, ingestion validators, and unit tests.
3. **Frontend Portal Enhancements**
   - Build Project DNA module with forms, validations, JSON preview, export actions.
   - Integrate versioning UI and publish workflow.
4. **Studio Integration**
   - Point Studio components to the same APIs; enhance UI affordances without altering contracts.
5. **Operational Tooling**
   - Add reverification scheduler, audit logs, change history diffing.
   - Document operator workflows (edit → preview JSON → publish → optional reverification).

## Ensuring System Stability
- Feature-flag the new pipeline until parity with CSV is confirmed.
- Expand automated tests covering schema decoding, ingestion validation, edge rule delivery, and UI snapshots.
- Provide rollback scripts (restore previous schema version, revert Redis payloads).
- Communicate rollout steps to operations and device teams, including any required firmware updates to consume edge rules.

## Next Steps
- Finalise DB schema design and obtain buy-in from ops/data teams.
- Prototype the Packet Designer form with live JSON preview to validate UX.
- Draft API contracts (OpenAPI) for shared typing between frontend and backend.
- Schedule incremental merges behind feature flags to keep production stable.

## Status Update (2026-01-02)
- Done: DB tables `payload_sensors`, `telemetry_thresholds`; Go endpoints for sensors/thresholds (project + device override); ConfigSync caches thresholds to Redis and publishes feature-flagged bundle `config:bundle:{projectId}` (schemas + thresholds + rules + automation) when `CONFIG_BUNDLE_ENABLED=true`; seed data for `pm-kusum-solar-pump-msedcl` (voltage/current/power); frontend DNA page now includes inline Packet Designer table, threshold editor (project/device scopes), quick-apply PM-KUSUM override, JSON copy/download preview; `go test ./...` and `npx vitest run` passing; **new:** backend sensor CSV versioning table + APIs (draft upload, list, publish, rollback) added; **new:** audit actor plumbed controller → service → repo for sensor versions (created/published/rolled_back by) and repository returns actor fields; **new:** frontend sensor versions grid wired for publish/rollback, per-version CSV download, history card, and inline CSV diff vs previous.
- Pending: apply/backfill new DB migration for `dna_sensor_versions` audit columns (`created_by`, `published_by`, `rolled_back_by`) across envs (local applied); harden version diffing (value-level deltas, error states) and add guardrails on publish/rollback; rules/automation tabs in DNA UI; reverification job; feature flags wiring on consumers; audit/history; OpenAPI+typed SDK; operator playbook; rerun `go test ./...` after audit plumbing.
- Infra note: `v1_init.sql` now embeds prior standalone schema files (v1_verticals, v2_credential_history, v3_vfd bundle, v4_protocols, dna_sensor_versions) for fresh installs.

## Approved UI Enhancements (to build)
- Layout: two-column with sticky context (project/device selectors, refresh, status) and main cards; gradient headers and dividers for clarity.
- Sensors: compact table with sortable columns (param/label/unit/min/max/resolution/required/topic), badges for required/units, inline edit toggles.
- Thresholds: tabs/pills for project defaults vs device overrides; device selector with recent devices; highlight overrides with badges; show source chip (default/override-merged); “apply recommended pump profile” quick-fill for PM-KUSUM.
- Forms: numeric steppers for warn/alert bands; validation hints; aligned Save/Refresh CTAs; toast feedback and last-sync note; skeleton loaders.
- Mobile: grids collapse cleanly; sticky action bar for Save/Refresh on narrow screens.

## Device Override Preload (PM-KUSUM exemplar)
- Default project: `pm-kusum-solar-pump-msedcl`.
- Suggested device (example): `pump-dev-001` (or first device in project list).
- Override profile to quick-apply:
   - `voltage`: warn_low 195, warn_high 255, alert_low 185, alert_high 265, origin `field-override`.
   - `current`: warn_low 1.5, warn_high 18, alert_low 1.0, alert_high 22, origin `field-override`.
   - `power`: warn_low 250, warn_high 7200, alert_low 150, alert_high 8200, origin `field-override`.
- UX flow: select device → auto-load overrides via `/thresholds?device=...` → show merged view with override badge → allow quick-apply of the above profile → Save triggers cache sync.

## Immediate TODO: fold per-sensor parameters, units, limits, thresholds, rules, automation
- **Canonical sensor schema**: add `param` (identifier), `label`, `unit`, `min`, `max`, `resolution`, `required`, `notes` columns to the DNA model; enforce `param` as the single key across ingestion strict-list, transformer inputs/outputs, rules, and UI.
- **Thresholds**: add per-parameter thresholds with warn/alert bands, scope (project default, optional per-device override), and source metadata (`origin`, `updated_at`). Publish to Redis (e.g., `config:thresholds:{projectId}`) and surface source in UI.
- **Payload specs**: import existing `rms-payload-parameters.csv` into the canonical store, retaining topic templates and required flags; expose CSV/JSON export for audits.
- **Rules/automation alignment**: ensure rule trigger fields point at the transformed payload keys (`param`), and automation actions are defined (MQTT command/alert) rather than logs only. Document in DNA alongside packet schemas.
- **Config sync**: extend ConfigSync to publish schemas + thresholds + rules + automation in one bundle; keep CSV import as bootstrap but warn on drift.
- **UI (Project DNA)**: Packet Designer shows param/unit/min/max/required; Thresholds tab per parameter; Rules/Automation tabs reuse the same identifiers; JSON preview shows the exact document to be saved/published.

### Proposed schema sketch (DB)
- `payload_sensors` (project_id, param PK, label, unit, min, max, resolution, required, notes, topic_template, updated_at)
- `telemetry_thresholds` (project_id, param, scope ENUM('project','device'), device_id NULLABLE, warn_low, warn_high, alert_low, alert_high, origin, updated_at)
- `automation_flows` / `rules` remain but ensure trigger fields reference `param`
- Keep CSV import/export jobs to map `rms-payload-parameters.csv` into `payload_sensors`
 - Draft DDL example: see `docs/project-dna-schema.sql`

### Proposed API (REST, to wire in @hk/core)
- `GET /api/project-dna/:projectId/sensors` → list sensors with param/unit/min/max/required
- `PUT /api/project-dna/:projectId/sensors` → bulk upsert
- `GET /api/project-dna/:projectId/thresholds?device={id}` → merges project defaults + optional device override
- `PUT /api/project-dna/:projectId/thresholds` → upsert thresholds (project-level)
- `PUT /api/project-dna/:projectId/thresholds/:deviceId` → upsert device override
- Redis cache keys: `config:project:{projectId}` (sensors), `config:thresholds:{projectId}` (project defaults), `config:thresholds:{projectId}:{deviceId}` (device override)
 - OpenAPI draft: see `docs/project-dna-openapi.md`

## Execution Backlog (2026-01-02)
- Packet Designer (FE, M) — Status: In Progress (inline table + JSON preview done). Remaining: validation polish, blueprint/view modes.
- Thresholds UX (FE, S) — Status: In Progress (project/device scopes, origin badges, quick-apply profile). Remaining: autocomplete device list, badges for merged/default vs override, validation hints.
- Rules/Automation tabs (FE, M) — Status: Not Started. Reuse ReactFlow composer; needs canonical rules API wiring and compiled preview.
- CSV import/export + versioning/publish/rollback (FE/BE, L) — Status: Mostly Done. Upload/list/publish/rollback/CSV download + audit actors + prev-diff live in UI; migration added (`schemas/v6_dna_sensor_versions_audit.sql`). Remaining: apply/backfill migration to all envs, add value-level diffs/tests, guardrails/empty-state handling.
- ConfigSync bundle + feature flags (BE, M) — Status: Done. Bundled `config:bundle:{projectId}` published behind `CONFIG_BUNDLE_ENABLED`/`FEATURE_CONFIG_BUNDLE`; legacy per-key caches unchanged.
- Reverification job + audit/history (BE/Ops, M) — Status: Not Started. Background reclassify job; audit logging for publishes/overrides with user attribution.
- OpenAPI + typed SDK (BE/FE, S) — Status: Not Started. Finalise API spec; generate typed client for `@hk/core`/apps.
- Operator docs (Ops, S) — Status: Not Started. Playbook for edit → preview JSON → publish → reverification; rollback steps.

### Near-Term Focus (proposed sprint picks)
1) Migration rollout + version governance hardening: apply/backfill `dna_sensor_versions` audit migration across envs; add value-level diff + publish/rollback guardrails + tests.
2) Packet Designer polish (FE, S): validation hints, blueprint/view modes, topic templates.
3) Thresholds UX polish (FE, S): device autocomplete, merged-source badges, validation helpers.
