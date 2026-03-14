# UI-KUSUMC <> GO-KUSUMC Alignment Deep-Dive Report
Date: 2026-03-11
Scope: rms-go/ui-kusumc/version-a-frontend and rms-go/go-kusumc

## 1) Method and Evidence
This report was built by reading:
- Frontend route registry: src/App.tsx
- Frontend page imports: src/pages/**/*.tsx and src/features/admin/**/*.tsx
- Frontend API layer: src/api/*.ts
- Backend route wiring: cmd/server/main.go and internal/adapters/primary/http/router.go
- Backend admin payload/response contracts: internal/adapters/primary/http/admin_controller.go
- Backend persistence contract for authorities: internal/adapters/secondary/postgres_repo.go and schemas/v1_init.sql

This is a source-based mapping (not only runtime probing), with special attention to contract drift and response shape mismatches.

## 2) Global Architecture and Data Flow
### 2.1 Frontend request pipeline
1. Pages call functions in src/api/*.ts.
2. api/http.ts normalizes key casing and auth header behavior.
3. Requests go to /api via Vite proxy.
4. RequireAuth gate protects most pages except /login and /simulator.

### 2.2 Backend request pipeline
1. cmd/server/main.go composes controllers/services/repos.
2. /api protected group uses middleware order:
   - ApiKeyMiddleware
   - AuthMiddleware
   - AuditMiddleware
3. Route-level RequireCapability is applied to most business endpoints.
4. Controllers call services, services call repos (postgres + redis + workers).

### 2.3 Runtime data domains furnishing UI
- Identity/session: auth service + JWT claims + refresh tokens.
- Hierarchy: states, authorities, projects, vendors, protocol versions.
- Device lifecycle: create/update/status/credential rotation/import jobs.
- Operations: commands, alerts, rules, automation, scheduler.
- Observability: telemetry history, live token/stream, thresholds, exports.
- Vertical/RMS entities: beneficiaries, installations, VFD models and catalogs.

## 3) Per-Page Expectations and API Mapping
Legend:
- Status: Aligned / Partial / Misaligned
- Backend mapping references are from cmd/server/main.go unless stated otherwise.

| Route | UI expectation | Frontend API calls/endpoints | Backend mapping | Status |
|---|---|---|---|---|
| /login | Authenticate and start session | POST /api/auth/login, POST /api/auth/refresh, POST /api/auth/logout, GET /api/auth/session | router.go registers all auth endpoints in public group; session shape transformed in FE | Partial |
| / (dashboard) | Quick device health summary | GET /api/devices (via fetchDeviceList) | protected GET /devices with devices:read capability | Aligned |
| /devices/enroll | Create device + seed credentials + hierarchy selections | POST /api/devices, GET /api/devices/{id}/status, GET /api/lookup/states, GET /api/lookup/authorities, GET /api/lookup/projects | protected POST /devices, GET /devices/:idOrUuid/status, lookup endpoints at /lookup/states, /lookup/authorities, /lookup/projects | Partial |
| /devices/import | Import devices and govt credentials | POST /api/devices/import, POST /api/devices/government-credentials/import | protected POST /devices/import and /devices/government-credentials/import | Aligned |
| /devices/import/jobs | Monitor import jobs and retries | GET /api/devices/import/jobs, GET /api/devices/import/jobs/{jobId}, GET /errors.csv, POST /retry | protected routes exist for import jobs and retries | Aligned |
| /devices/configuration/internal | Queue/poll/ack internal configuration | POST /api/devices/{id}/configuration, GET /pending, POST /ack | protected config queue/pending/ack routes exist | Aligned |
| /devices/configuration/government | Upsert government credentials, history, import | PUT /api/devices/{id}/government-credentials and related fetches | router.go wires both govt-creds and government-credentials style routes | Partial |
| /devices/configuration/drive | Drive/VFD commands + model selection | VFD fetch + device command endpoints | protected /vfd-models* and /devices/:device_uuid/commands* routes exist | Partial |
| /telemetry | History + live token + thresholds | GET /api/telemetry/devices/{device}/history, POST /live-token, GET/PUT/DELETE /telemetry/thresholds/{device} | protected telemetry history/live/threshold routes exist | Aligned |
| /telemetry/v2 | Enhanced telemetry + command interactions | telemetry endpoints + /devices/{device}/commands/history + POST /commands | telemetry and command routes exist | Aligned |
| /telemetry/export | Export telemetry data | GET /api/telemetry/export | protected GET /telemetry/export exists | Aligned |
| /operations/installations | Manage installations and linked beneficiaries | GET/POST/PATCH /api/installations, GET/POST/DELETE /api/installations/{id}/beneficiaries, /api/beneficiaries* | verticalController routes exist for installations and beneficiaries | Aligned |
| /operations/command-center | Send operational commands and inspect responses | GET /api/commands/catalog, POST /api/commands/send, device command endpoints | protected /commands* and /devices/:device_uuid/commands* routes exist | Aligned |
| /operations/command-catalog | Manage command catalog entries | GET /api/commands/catalog-admin, POST /api/commands/catalog, DELETE /api/commands/catalog/{id} | protected catalog routes exist | Aligned |
| /operations/rules-alerts | Rule CRUD + alert ack | GET/POST/DELETE /api/rules, GET /api/alerts, PUT /api/alerts/{id}/ack | rules and alerts routes exist with alerts:manage | Aligned |
| /operations/automation | Read/write automation graph | GET /api/config/automation/{projectId}, POST /api/config/automation | protected config automation routes exist | Aligned |
| /live/device-inventory | Device list, filter, search | GET /api/devices, GET /api/devices/lookup | protected routes exist | Aligned |
| /live/device-inventory/:idOrUuid | Device detail + history + config actions | GET /api/devices/{id}, GET /status, GET /credentials/history, GET /commands/history, queue/pending config | protected routes exist | Aligned |
| /reports | Download compliance report by id | GET /api/reports/{id}/compliance | protected GET /reports/:id/compliance exists | Aligned |
| /admin/states | State CRUD | GET/POST/PATCH/DELETE /api/admin/states | protected admin states routes exist | Partial |
| /admin/state-authorities | Authority CRUD filtered by state | GET/POST/PATCH/DELETE /api/admin/state-authorities | protected routes exist; contract mismatch found in payload/response conventions | Partial |
| /admin/projects | Project CRUD | GET/POST/PATCH/DELETE /api/admin/projects | routes wired in router.go RegisterRoutes | Aligned |
| /admin/orgs | Organization CRUD | GET/POST/PUT /api/orgs | protected org routes exist | Aligned |
| /admin/apikeys | API key create/revoke/list | GET/POST/DELETE /api/admin/apikeys | protected routes exist | Aligned |
| /admin/audit | Audit listing | GET /api/audit | protected route exists with audit:read/admin:all | Aligned |
| /admin/scheduler | Schedule list/create/toggle | GET/POST /api/scheduler/schedules, PUT /api/scheduler/schedules/{id}/toggle | protected scheduler routes exist | Aligned |
| /admin/dna | DNA read/upsert | GET/PUT /api/dna/{projectId} | protected dna routes exist | Aligned |
| /admin/device-profiles | Device profile catalog | GET/POST /api/config/profiles | protected routes exist | Aligned |
| /admin/simulator-sessions | Simulator session CRUD | GET/POST /api/simulator/sessions, DELETE /api/simulator/sessions/{id} | protected simulator session routes exist | Aligned |
| /admin/server-vendors | Server vendor CRUD | GET/POST/PATCH/DELETE /api/admin/server-vendors | protected routes exist and dispatch to admin controller vendor funcs | Aligned |
| /admin/protocol-versions | Protocol-version CRUD | GET/POST/PATCH /api/admin/protocol-versions | protected routes exist | Aligned |
| /admin/drive-manufacturers | Vendor catalog partition: drive | uses admin vendor APIs through VendorsSection | backed by admin vendor endpoints | Partial |
| /admin/pump-vendors | Vendor catalog partition: pump | uses admin vendor APIs through VendorsSection | backed by admin vendor endpoints | Partial |
| /admin/rms-manufacturers | Vendor catalog partition: RMS | uses admin vendor APIs through VendorsSection | backed by admin vendor endpoints | Partial |
| /admin/users | User CRUD + password reset + role assignment | /api/admin/users*, /api/users/roles, /api/admin/users/{id}/password | protected routes exist | Aligned |
| /admin/user-groups | Group CRUD + membership | /api/user-groups*, /api/user-groups/{id}/members | protected routes exist | Aligned |
| /admin/vfd-models | VFD catalog list/import/export | /api/vfd-models, /api/vfd-models/import, /api/vfd-models/export.csv | protected routes exist | Aligned |
| /admin/vfd-catalog-ops | command dictionary import ops | /api/vfd-models/command-dictionaries/import/jobs and /import | protected routes exist | Aligned |
| /simulator | Generate simulator script and bootstrap stack dependencies | GET /api/builder/simulator/{projectId} + lookup/admin/device APIs used in setup workflow | public builder route exists; data deps rely on protected admin/device routes during setup | Partial |

## 4) Verified Gaps and Misalignments
### Critical
1. Schema-contract drift in authorities updated_at:
- DB schema in schemas/v1_init.sql defines authorities without updated_at.
- Repo previously selected/updated updated_at for authorities.
- This caused runtime SQLSTATE 42703 (column updated_at does not exist).
- Scope: simulator default setup and admin authority flows.

2. Contract fragility in simulator bootstrap chain:
- Simulator setup requires ordered creation/read of state -> authority -> project -> protocol -> vendors.
- Any envelope mismatch or missing field causes cascading failure and generic setup error banners.

### High
1. Response envelope inconsistency across admin APIs:
- Some list APIs return bare arrays.
- Some create/update APIs wrap with keys like state or authority.
- FE now includes collection coercion fallback, but this is defensive complexity and brittle.

2. Case-style inconsistency (snake_case vs camelCase):
- Backend generally expects snake_case on bodies.
- Frontend must send compatibility fields in some places.
- Query aliases exist for selected keys only; drift risk remains for newly added params.

3. Endpoint naming duplication for government credentials:
- Both govt-creds and government-credentials styles exist.
- Functional but increases maintenance risk and client confusion.

### Medium
1. Capability requirements are strict but page UX does not always preflight permissions with explicit messaging.
2. Some pages rely on wrapper components (Admin pages) making API ownership less obvious and harder to audit.
3. Live telemetry and provisioning states have partial UI lifecycle handling (status transitions not always surfaced end-to-end).

### Low
1. Inconsistent error object shapes (error.message vs message vs plain text) force many FE fallback branches.
2. Duplicate v1 and non-v1 route aliases increase route surface and testing cost.

## 5) Internal Data Flow Needed to Furnish UI
### 5.1 Login/session
- UI login -> AuthService -> token issuance -> FE session snapshot -> capability gates in RequireAuth and page-level controls.

### 5.2 Enrollment/import
- UI form/csv -> DeviceController/Router handlers -> DeviceService/BulkService -> Postgres writes + provisioning job -> UI status polling/readbacks.

### 5.3 Telemetry views
- Device telemetry ingest (MQTT/HTTP) -> ingestion pipeline -> storage/cache -> analytics/threshold endpoints -> UI charts/live stream.

### 5.4 Commands
- UI command issue -> CommandsService queue -> MQTT worker/device -> command responses/history -> UI refresh.

### 5.5 Admin hierarchy
- UI sections -> AdminController CRUD -> AdminService -> PostgresRepo -> lookups feeding enrollment/simulator.

### 5.6 Simulator workflow
- UI script fetch + hierarchy bootstrap + optional device create/config steps -> mixed public/protected routes -> depends on strict shape alignment.

## 6) Scope of Improvements
### 6.1 Short-term (1-2 sprints)
1. Freeze and document canonical response envelopes for all admin endpoints (prefer one list envelope + one mutation envelope style).
2. Introduce contract tests for simulator bootstrap path (state/authority/project/protocol/vendor create/list chain).
3. Keep authority SQL compatible with current schema (do not rely on authorities.updated_at until schema migration lands).
4. Add explicit frontend mapping adapters per endpoint family (admin, telemetry, devices) instead of ad hoc coercion in pages.
5. Add a per-page permission and endpoint smoke test suite (headless API contract checks).

### 6.2 Medium-term (quarter)
1. Generate frontend API types from backend OpenAPI and remove manual dual-case normalization where possible.
2. Consolidate route aliases:
- choose canonical government credential path style,
- keep temporary compatibility aliases,
- deprecate with timeline.
3. Add route ownership docs (controller/service/repo) for faster debugging and onboarding.
4. Add integration tests for every App.tsx route’s primary read/write call set.

## 7) Recommended Action Plan
1. Contract Baseline:
- Publish a canonical JSON contract doc for admin and enrollment/simulator APIs.

2. Regression Harness:
- Add a route-by-route API smoke test matrix that mirrors this report.

3. Simulator Reliability:
- Make simulator setup report exact failing API step and payload diff in UI.

4. Schema Governance:
- Add startup schema sanity checks for required columns used by repo queries.

## 8) Confidence Notes
- Route existence and capabilities are high-confidence (verified against main.go/router.go).
- Per-page API ownership is high-confidence for pages importing src/api directly and medium-confidence for wrapper pages delegating to section components.
- Data-flow paths are based on wiring in composition root and controller naming conventions, then cross-checked with API call sites.
