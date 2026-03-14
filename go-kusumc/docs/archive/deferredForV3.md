# Deferred For V3 (Multi-Project / Multi-Vertical Enablement)

Date: 2026-01-03

## TL;DR
Yes — the following pages/APIs are not part of the PM-KUSUM (solar pump) vertical deliverable today. They are *cross-vertical demo modules* or *other-vertical modules* (traffic / healthcare / agriculture / supply-chain) and should be treated as deferred until we introduce explicit multi-project + vertical enablement ("V3").

PM-KUSUM needs the shared platform primitives (auth, projects, device inventory, bootstrap, telemetry, rules/DNA, reports) and the pump-specific screens (RMS dashboard/monitoring).

---

## Scope Definitions

### In-scope for PM-KUSUM (current)
- Auth + roles + audit middleware
- Projects + project context selection
- Device lifecycle for pumps: inventory, detail/edit, rotate creds, bootstrap
- Telemetry ingestion + history (`/api/telemetry`)
- DNA + thresholds + versions (`/api/project-dna/*`)
- Rules + automation flow config (`/api/rules`, `/api/config/automation/*`)
- Compliance report generation *only if required by PM-KUSUM rollout* (`/api/reports/:id/compliance`)

### Deferred for V3 (other verticals / demos)
These are the items you listed (plus the UI modules that imply them):

1) Traffic
- UI: `TrafficPage`
- Expected APIs (frontend):
  - `GET /api/traffic/metrics/:deviceId`
  - `POST /api/traffic/metrics`
  - (already present) `GET /api/traffic/cameras`
- Status: metrics endpoints are missing; cameras list exists.

2) Healthcare
- UI: `HealthcarePage`
- Expected APIs (frontend):
  - `GET/POST /api/healthcare/patients`
  - `POST /api/healthcare/sessions/start`
  - `POST /api/healthcare/sessions/:id/end`
- Status: backend has `GET/POST /api/patients` (vertical service), but not `/api/healthcare/*` and no sessions.

3) Agriculture / Soil
- UI: `SoilHealthPage`
- Expected APIs (frontend):
  - `POST /api/agriculture/advice`
  - `POST /api/agriculture/rules` (seed)
- Status: missing.

4) Telemetry export (UI wiring)
- UI: `AnalyticsPage` has an Export button but no handler.
- Backend: has `GET /api/telemetry/export` (currently returns demo data).
- Status: FE wiring deferred; backend export is not production-ready (demo payload).

5) Logistics UI
- UI: `LogisticsPage` is explicitly disabled.
- Backend: logistics routes exist under protected `/api/logistics/*` (trips/geofences/assets timeline).
- Status: UI enablement deferred.

6) Digital Twin / Custom Dashboards
- UI: stub pages.
- Backend: no requirement yet.

7) Network Planner
- UI: FE-only Leaflet planner.
- Backend: no persistence routes.

---

## V3 Enablement Plan (how we make these loadable per-project)

### A) Introduce explicit "vertical enablement" in project config
- Add a project-level config field (or config bundle field) like:
  - `enabled_modules: ["pm_kusum", "traffic", "healthcare", "agriculture", "inventory", "logistics"]`
- Source of truth:
  - Prefer `config:bundle:{projectId}` (already referenced by config bundle consumers) so FE can query once and gate nav/routes.

### B) Frontend gating
- In `AdminLayout` (nav + command palette): hide/show modules based on `enabled_modules` for active project.
- Ensure URLs don’t 404 awkwardly:
  - If module disabled, route should render a single-line "Module disabled for this project" page.

### C) Backend gating (optional but recommended)
- Protect non-enabled vertical endpoints by checking project module flags.
- Prevent accidental use of demo endpoints in PM-KUSUM projects.

---

## Concrete Work Items (V3 backlog)

### P0 (platform correctness; benefits PM-KUSUM too)
- Implement device read/update endpoints used by the core admin UI:
  - `GET /api/devices` (pagination/filter)
  - `GET /api/devices/:id`
  - `PUT /api/devices/:id`

### P1 (vertical enablement framework)
- Add `enabled_modules` to project config + include in config bundle.
- FE: gate nav/routes by `enabled_modules`.
- BE: optionally gate routes by `enabled_modules`.

### P2 (Traffic)
- Add traffic metrics APIs + storage model:
  - `POST /api/traffic/metrics`
  - `GET /api/traffic/metrics/:deviceId`

### P3 (Healthcare)
- Decide canonical route set:
  - Either update FE to use existing `/api/patients`, or provide `/api/healthcare/*` aliases.
- Add sessions model + endpoints:
  - `POST /api/healthcare/sessions/start`
  - `POST /api/healthcare/sessions/:id/end`

### P4 (Agriculture)
- Implement advice/rules endpoints:
  - `POST /api/agriculture/rules`
  - `POST /api/agriculture/advice`

### P5 (Inventory/Logistics)
- Inventory stock endpoint expected by UI:
  - `GET /api/inventory/stock/:locationId`
- Decide whether to enable `LogisticsPage` (currently disabled) once UX is approved.

### P6 (Export)
- Decide whether telemetry export is per-device/per-window and wire FE export button to:
  - a real export endpoint (or upgrade existing `/api/telemetry/export`).

### P7 (Planner/Digital Twin)
- If persistence is needed:
  - add CRUD endpoints for network plans, twin simulations, and dashboard schemas.

---

## Notes / Decisions Needed
- Are non-PM modules intended as demos only, or real verticals in V3?
- For healthcare, do we standardize on `/api/patients` or `/api/healthcare/patients`?
- For exports, is the requirement XLSX only, or PDF too?
