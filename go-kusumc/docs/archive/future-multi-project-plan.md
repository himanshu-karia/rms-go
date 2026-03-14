# Future multi-project plan (core + verticals)

Audience: the future team after PM-KUSUM demo, when shipping multi-vertical builds becomes necessary.
Scope: rationale, phased approach, and a proposed capabilities contract for backend/frontend. No changes are required now.

## Rationale (why not now)
- Shipping one vertical first (PM-KUSUM). Premature modular bringup adds testing matrix and developer overhead.
- Current codebase runs fine as a single build; we already staged modular DB SQL in `new-db-schema/` for later.
- For the demo, the smallest change is UI gating (hide non-PM-KUSUM routes/menus) without altering backend wiring.

## Current assets staged (do not wire yet)
- DB modular SQL + docs + diagrams: `unified-go/new-db-schema/` (core + vertical bundles, manifest, ER diagrams).
- No runtime changes; production DB still uses `schemas/`.

## Phased approach
1) **Now → Demo**
   - Keep single build (backend + frontend fully wired).
   - Optional: hide non-PM-KUSUM UI routes/menus for demo polish.
   - Do not refactor backend route/module wiring yet.
2) **Post-demo, pre-shipping (capabilities contract)**
   - Add capability metadata per project (from Project DNA / project type) and expose via API.
   - UI consumes capabilities to show/hide routes, nav, and panels (no code removal, just gating).
   - Backend keeps all routes but can optionally 404/forbid when capability is off (lightweight guard).
3) **Shipping-ready modular bringup (optional, if needed)**
   - Introduce module registry: core module always on; vertical modules register routes/workers conditionally.
   - Optional workers per module; optional build-time flags only if we truly ship different binaries.
   - Align with staged DB bundles (core + selected vertical SQL).

## Proposed capabilities contract (minimal, additive)
- Source of truth: Project DNA or project type (string) mapped to a set of capability keys.
- Backend endpoint (suggested): `GET /api/projects/:id/capabilities` → `{ projectId, type, capabilities: string[] }`.
- Example capability keys (can grow):
  - `core` (implicit)
  - `protocol_profiles`
  - `govt_creds`
  - `beneficiaries`
  - `installations`
  - `vfd_catalog`
  - `vfd_assignments`
  - `healthcare`
  - `gis_layers`
  - `traffic`
  - `logistics`
  - `maintenance`
- Backend guard pattern (optional): middleware that checks capability before handler; by default only UI hides.
- Frontend usage: a capabilities context/hook that gates nav items, routes, and page sections.

## UI gating plan (post-demo)
- Nav + routes only render when capability is present.
- Page-level sections (e.g., DeviceDetail panels for govt creds/rotation/provisioning status) check capabilities.
- Keep code loaded; focus on hiding and avoiding API calls when capability is absent.

## Backend module plan (shipping phase, if needed)
- Keep `cmd/server/main.go` as composition root; define `Register(moduleConfig)` functions per vertical.
- Core never depends on vertical modules; vertical modules depend on core services/ports.
- Optional background workers declared per module.
- API guards use capability middleware or route grouping by module.

## DB alignment (shipping phase)
- Use `new-db-schema/bundles.yaml` to decide which SQL to mount for a deployment: always `core.sql`, plus selected `sql/verticals/*.sql`.
- Keep FK rule: vertical → core only; avoid core → vertical.

## Minimal backlog to reach PM-KUSUM MVP (already discussed)
- Protocol Profiles UI; DeviceDetail panels (bootstrap preview, provisioning status, rotate MQTT creds, govt creds); Beneficiaries/Installations; VFD catalog + protocol assignments.
- These are orthogonal to the capabilities contract; build them now without modularization.

## Rollout checklist (post-demo)
1) Add capability model + API.
2) Add frontend capability context and gate nav/routes/panels.
3) (Optional) Add backend capability middleware for sensitive routes.
4) If needed, refactor modules (route/worker registration) after capabilities are stable.

## Risks / cautions
- Do not introduce build-time splits unless a product requirement demands distinct binaries; prefer runtime gating.
- Keep testing matrix small: `core` only and `core + pm_kusum` initially; add others only when a new vertical is real.

## References
- DB staging area: `unified-go/new-db-schema/` (README, bundles.yaml, handover, diagrams).
- UI coverage matrix: `unified-go/docs/ui-coverage-matrix.md` (what UI is missing today).
