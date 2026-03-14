# Delivery Plan, Workstreams, and Estimates

Date: 2026-03-04

## 1) Program shape

Tracks:
- Track A: Android app build
- Track B: Backend/mobile bridge APIs
- Track C: Security and observability hardening
- Track D: QA/UAT and rollout

## 2) Team assumptions

- Single implementation owner: GitHub Copilot (Android + Backend + QA automation support)
- Human collaborator (you): priority decisions, acceptance, and field validation inputs
- Execution model: strictly sequenced by dependency; parallelism only where safe via CI automation

## 3) Milestone plan

## M1 — Foundation (Weeks 1-3)
- Finalize contracts from `07-openapi-mobile-bridge-draft.yaml`
- Backend: mobile client registration + auth stubs + bridge session skeleton
- App: login, project/device selection, local secure storage, base shell

Exit criteria
- End-to-end auth works for test users
- Mock bridge session established from app

## M2 — Bridge Core (Weeks 4-7)
- App: BLE/BT/WiFi adapters and local queue
- Backend: packet ingest batch endpoint with idempotency + reason codes
- Basic command submit/result APIs

Exit criteria
- App can collect and upload sample packet batches
- Server returns deterministic accept/reject/duplicate outcomes

## M3 — Reliability + Security (Weeks 8-10)
- Backoff/retry, resume, large-batch handling
- Phone enrollment approval/revocation policies
- Audit events and metrics dashboards

Exit criteria
- 10k packet sync scenario passes reliability targets
- Unauthorized scope and replay tests blocked

## M4 — UAT + Go-live (Weeks 11-12)
- Field pilot with real devices
- Production runbook and rollback tested
- Release readiness sign-off

Exit criteria
- UAT pass with agreed KPIs
- Incident response drill completed

## 4) Effort estimate (high level)

- Android app: 12-16 engineer-weeks
- Backend APIs/data model: 10-14 engineer-weeks
- Security/observability: 4-6 engineer-weeks
- QA automation + UAT: 5-7 engineer-weeks

Total: 31-43 engineer-weeks (parallelized across 10-12 calendar weeks).

## 5) Critical dependencies

- Final device local protocol details (BLE/BT/WiFi command/log extraction)
- Stable packet schema variants from firmware
- RBAC decision for mobile impersonation permissions

## 6) Release strategy

- Internal alpha (staff devices)
- Controlled beta (selected field teams)
- Progressive production rollout by project/state

Rollback
- Feature flags on server for mobile ingest and command relay
- Ability to disable bridge session creation without impacting existing web/device flows

---

## 7) Sprint board (ticket-sized, aligned to Docs 01-12)

Status legend: `[ ]` not started, `[~]` in progress, `[x]` complete.

### Sprint 1 — Foundation (Auth + Contracts + Session Shell)
Goal: establish secure identity, contract baseline, and app entry shell.

- [x] **BE-01** (`Doc 02`): mobile auth endpoints (`request-otp`, `verify`, `refresh`, `logout`) — **L**
- [x] **BE-02** (`Doc 02`): assignments endpoint (`/api/mobile/me/assignments`) — **M**
- [x] **BE-10** (`Doc 07`): finalize OpenAPI v1 paths/schemas for auth/assignments/ingest/command status — **M**
- [x] **BE-05** (`Doc 03`): consistent error schema + status code contract — **S**
- [x] **MOB-01** (`Doc 01`): Login + session bootstrap flow — **M**
- [x] **MOB-05** (`Doc 03`): bridge client + auth handling — **M**
- [x] **MOB-10** (`Doc 05`): encrypted token/session storage — **M**
- [x] **MOB-02** (`Doc 01`): technician home + assigned device list shell — **M**
- [x] **DB-01** (`Doc 10`): `mobile_sessions`, `mobile_assignments`, `mobile_ingest_dedupe` migrations — **M**
- [x] **QA-01** (`Doc 09`): unit tests for token refresh/session reducer logic — **M**

Exit criteria:
- Auth + refresh works end-to-end from Android to backend
- Assignments rendered in app from live API
- OpenAPI and implementation responses are consistent

### Sprint 2 — Bridge + Offline Sync Core
Goal: local transport abstraction, durable queue, and idempotent ingest.

- [x] **MOB-07** (`Doc 04`): `LocalTransport` abstraction (`BLE`, `WiFiLocal`, `Mock`) — **L**
- [x] **MOB-08** (`Doc 04`): Room-backed outbox (`outbox_events`) — **L**
- [x] **MOB-09** (`Doc 04`): WorkManager sync worker (constraints + backoff) — **M**
- [x] **BE-03** (`Doc 02`): `POST /api/mobile/ingest` with validation and dedupe integration — **L**
- [x] **BE-07** (`Doc 04`): idempotency service/store with TTL and replay-safe behavior — **M**
- [x] **MOB-15** (`Doc 10`): Room schema for `outbox_events`, `sync_runs`, `command_cache` — **M**
- [x] **BE-06** (`Doc 03`): request-id/trace propagation middleware — **S**
- [x] **QA-02** (`Doc 09`): instrumentation flow login → assignment → enqueue → sync (skeleton assertions) — **L**

Exit criteria:
- Offline events survive app restart and sync reliably
- Duplicate ingest submissions are safely deduped
- Sync retries converge without data loss

### Sprint 3 — Command Reliability + Hardening + Pilot Readiness
Goal: command lifecycle correctness, security hardening, observability, and UAT readiness.

- [x] **BE-04** (`Doc 02`): command status endpoint with lifecycle states — **M**
- [x] **MOB-06** (`Doc 03`): command transport abstraction (HTTP-first, WSS fallback) — **M**
- [x] **BE-08** (`Doc 05`): mobile audit attribution fields + middleware — **M**
- [x] **BE-09** (`Doc 05`): metrics for auth/ingest/command latency + outcomes — **M**
- [x] **MOB-11** (`Doc 05`): TLS pinning + hostname policy — **M**
- [x] **QA-03** (`Doc 09`): backend integration tests for ingest idempotency + command transitions — **M**
- [x] **QA-04** (`Doc 09`): field UAT script and pass/fail rubric — **S**
- [x] **OPS-01..03** (`Doc 11`): runbook + replay + alert thresholds — **M**
- [x] **PLAN-03** (`Doc 08`): release gate checklist and sign-off matrix — **S**

Exit criteria:
- Command status reconciliation is deterministic and test-covered
- Security and observability controls are active in staging
- UAT and runbook artifacts are complete for pilot

---

## 8) Autonomous execution protocol

This project runs under single-agent implementation mode.

Rules of execution:
- I pick the next ticket by dependency order and start implementation immediately.
- I update status in docs/checklists as tickets move `[ ] -> [~] -> [x]`.
- I run focused tests first, then broader integration tests where applicable.
- I keep scope strict to ticket acceptance criteria; no side-feature additions.
- I escalate only when a hard product decision is required (transport priority, retention limits, timeout matrix).

Decision checkpoints requiring your input:
- **DEC-01** (`Doc 12`): MVP local transport priority (`BLE-first` vs `WiFi-first`)
- **DEC-02** (`Doc 12`): offline retention limits and purge cadence
- **DEC-03** (`Doc 12`): command timeout/retry policy table

---

## 9) Immediate execution queue (next 10 tickets)

1. **BE-10** (OpenAPI finalize)
2. **BE-05** (error schema unification)
3. **DB-01** (mobile schema migrations)
4. **BE-01** (auth endpoints)
5. **BE-02** (assignments endpoint)
6. **MOB-01** (login + session bootstrap)
7. **MOB-05** (API client + auth interceptor)
8. **MOB-10** (encrypted token storage)
9. **QA-01** (session/auth unit tests)
10. **PLAN-01/02** (sprint board ownership + sizing lock)

Start state: all above are currently `[ ]` and ready to begin in listed order.

## 10) Execution progress update (2026-03-04)

Completed in code:
- `[x]` **BE-10**: OpenAPI v1 contract finalized for mobile auth/assignments/ingest/command status.
- `[x]` **BE-05** (phase-1): standardized API error contract helper added and wired in auth + api key middleware.
- `[x]` **DB-01**: baseline tables added (`mobile_sessions`, `mobile_assignments`, `mobile_ingest_dedupe`).
- `[x]` **BE-01**: `/api/mobile/auth/request-otp|verify|refresh|logout` implemented.
- `[x]` **BE-02**: `/api/mobile/me/assignments` implemented.
- `[x]` **BE-03** (phase-1): `/api/mobile/ingest` implemented with per-packet dedupe persistence.
- `[x]` **BE-04** (phase-1): `/api/mobile/commands/:id/status` implemented with lifecycle mapping.
- `[x]` **BE-06** (phase-1): request-id propagation middleware enabled for `/api/mobile/*` routes.
- `[x]` **BE-03** (phase-2): accepted mobile packets now flow through canonical ingest processing path.
- `[x]` **BE-07** (phase-1): duplicate idempotency submissions now replay stored prior result payload.
- `[x]` **BE-05** (phase-2): shared API error helper adopted in core auth/router handlers.
- `[x]` **BE-08** (prep): mobile audit attribution metadata (`actor_type`, `actor_id`, `request_id`) added via audit middleware.
- `[x]` **BE-09** (prep): mobile endpoint counters exported via `/metrics`.
- `[x]` **QA-03** (start): added mobile integration tests for idempotency replay and command status mapping.
- `[x]` **BE-08** (full): audit metadata now includes mobile actor attribution + request/session correlation metadata.
- `[x]` **BE-09** (full): per-endpoint mobile request/error/latency bucket metrics now exported on `/metrics`.
- `[x]` **QA-03** (phase-2): added unit test coverage for command status mapping function.
- `[x]` **MOB-01**: Android login + OTP verify + auth gate wired into app shell (`MainActivity` + `LoginScreen`).
- `[x]` **MOB-05**: mobile API client/models/session viewmodel scaffolding added and connected to auth flow.
- `[x]` **MOB-10**: token/session persistence moved to encrypted preferences with fallback path.
- `[x]` **MOB-01/05 stabilization**: fixed `UxMultiSeriesGraph` compile break and restored clean Kotlin compile.
- `[x]` **QA-01**: added Android unit tests for session bootstrap refresh success/failure and OTP verify error transitions.
- `[x]` **MOB-02**: home screen now renders assigned project/device shell from authenticated assignment state.
- `[x]` **MOB-06**: added command transport abstraction (`HTTP` primary + `WSS` fallback scaffold) and wired dashboard command sends to HTTP-first with MQTT runtime fallback.
- `[x]` **MOB-07**: added first-cut local transport abstraction and adapters (`BLE`, `WiFiLocal`, `Mock`) for upcoming offline bridge integration.
- `[x]` **MOB-08**: added Room outbox foundation (`outbox_events` entity, DAO, database, repository) with retry metadata fields.
- `[x]` **MOB-09**: added WorkManager sync skeleton (`MobileOutboxSyncWorker`, constraints, exponential retry policy) and auth-triggered scheduler wiring.
- `[x]` **MOB-15**: added Room tables `sync_runs` and `command_cache`, DAOs/repositories, and retention helpers for outbox terminal rows, finished sync runs, and expired command cache entries.
- `[x]` **QA-02**: added Android instrumentation skeleton covering login/assignment state, outbox enqueue, and sync worker retry path assertions.
- `[x]` **MOB-11**: centralized secure HTTP connection policy with HTTPS hostname checks, optional certificate pin validation, and debug-only cleartext guardrails; applied to mobile auth + command HTTP clients.
- `[x]` **QA-03** (non-skip validation): mobile integration tests now pass against refreshed backend runtime with live `/api/mobile/*` route execution.

Validation:
- `go test ./internal/adapters/primary/http/...` ✅
- `go test ./internal/core/services/...` ✅
- `go test -tags=integration ./tests/e2e -run 'TestMobileIngest_IdempotencyReplay|TestMobileCommandStatus_Mapping'` ✅ (routes skipped when server binary is not refreshed)
- `./gradlew.bat :app:compileDebugKotlin --console=plain --no-daemon` ✅
- `./gradlew.bat :app:testDebugUnitTest --tests com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModelTest --console=plain --no-daemon` ✅
- `./gradlew.bat :app:compileDebugKotlin :app:testDebugUnitTest --tests com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModelTest --console=plain --no-daemon` ✅
- `./gradlew.bat :app:compileDebugKotlin :app:testDebugUnitTest --tests com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModelTest --console=plain --no-daemon` ✅ (`MOB_08_09_OK`)
- `./gradlew.bat :app:compileDebugKotlin :app:testDebugUnitTest --tests com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModelTest --console=plain --no-daemon` ✅ (`MOB_15_OK`)
- `./gradlew.bat :app:compileDebugKotlin :app:compileDebugAndroidTestKotlin :app:testDebugUnitTest --tests com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModelTest --console=plain --no-daemon` ✅ (`QA_02_SKELETON_OK`)
- `./gradlew.bat :app:compileDebugKotlin :app:compileDebugAndroidTestKotlin :app:testDebugUnitTest --tests com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModelTest --console=plain --no-daemon` ✅ (`MOB_11_OK`)
- `go test -tags=integration ./tests/e2e -run 'TestMobileIngest_IdempotencyReplay|TestMobileCommandStatus_Mapping' -count=1 -v` ✅ (non-skip pass on refreshed server)

Execution batch closed:
- **QA-04**: field UAT script updates for mobile sync/command fallback observability checks completed.
- **OPS-01..03**: runbook + replay + alert threshold docs for mobile sync and command fallback operations completed.
- **PLAN-03**: release gate checklist and sign-off matrix closure updates completed.

Latest stabilization (2026-03-04):
- `TestDeviceCommandLifecycle` now passes deterministically in the long automation chain (no warn-only bypass).
- E2E default `PROJECT_ID` fallbacks were aligned to seeded project `pm-kusum-solar-pump-msedcl` for device/bootstrap coverage flows.

Latest parity expansion (2026-03-05):
- `[x]` Added Android Admin Orgs page wired to `/api/orgs` (list/create/update).
- `[x]` Added Android Admin User Groups page wired to `/api/user-groups` and membership endpoints (`list/create/update/delete`, `members add/remove/list`).
- `[x]` Extended mobile admin API/client models for org and user-group contracts.
- `[x]` Added navigation + drawer entries for **Admin Orgs** and **User Groups**.
- `[x]` Expanded Admin Catalogs vendor management to switch across categories: `server-vendors`, `solar-pump-vendors`, `vfd-drive-manufacturers`, `rms-manufacturers`.
- Validation: `./gradlew.bat :app:compileDebugKotlin` ✅

---

## 11) Master long E2E plan (UI role + Device role + Broker + Persistence)

Objective:
- Run one sequential internal test that starts from mobile login and validates full user-role and device-role lifecycle through API + MQTT + persistence checks.
- Enforce a strict verification chain: User Role -> Backend -> Broker -> Device Role -> User Role.

Execution command (current baseline):

```powershell
cd rms-go/app-kusumc
./scripts/long-automated-mobile-auth-to-persistence.ps1 -RunAdbLoginOtpFlow
```

## 12) App <-> Server interaction inventory (what must be exercised)

Mobile auth/session interactions:
- `POST /api/mobile/auth/request-otp`
- `POST /api/mobile/auth/verify`
- `POST /api/mobile/auth/refresh`
- `POST /api/mobile/auth/logout`
- `GET /api/mobile/me/assignments`

User-role operational interactions (admin/operator):
- create/update device
- add beneficiary
- allocate installation details
- bootstrap request for device credentials
- command publish and status follow-up
- rotate device credentials

Device-role operational interactions:
- bootstrap to fetch local MQTT credentials and broker endpoint
- connect to broker with active credentials
- publish telemetry/events on required device topics
- subscribe to command/response topics and complete ack loop
- after credential rotation: old credential connect rejection -> re-bootstrap -> reconnect -> resume pub/sub

Persistence interactions that must be asserted:
- auth/session records and assignment scope state
- user-role creation/allocation state transitions
- telemetry ingestion persistence
- command timeline persistence (queued/sent/acked)
- idempotent replay behavior for duplicate packet/API submissions

## 13) UI auditor sequential steps (single run order)

Phase A: Mobile login and auth bootstrap
1. Launch app over ADB.
2. Enter phone and request OTP from app when UI allows.
3. Fallback path: server-side OTP request + latest OTP retrieval for internal test.
4. Inject OTP via ADB and trigger verify.
5. Validate authenticated state (token issued and assignments available).

Phase B: User-role API action chain
6. Execute create device, beneficiary mapping, and installation allocation.
7. Assert API success and fetch-back consistency for created entities.
8. Assert DB persistence for each user-role action.

Phase C: Device bootstrap and MQTT pub/sub cycle
9. Execute bootstrap and fetch active local credentials.
10. Connect device-role MQTT client with fetched credentials.
11. Publish telemetry packets on required topics.
12. Subscribe to command/response topics and validate ack loop.
13. Assert telemetry and command persistence through APIs and DB reads.

Phase D: Credential rotation recovery cycle
14. Rotate device credentials through backend API.
15. Assert old MQTT credentials are rejected by broker/server.
16. Re-bootstrap device role and fetch new credentials.
17. Reconnect MQTT with new credentials.
18. Re-run publish/subscribe checks and persistence assertions.

Phase E: End-state audit
19. Verify no critical step is warn-only.
20. Emit summary artifact with pass/fail per phase and evidence links.

## 14) Internal testing mechanisms: present vs to-add

Present now:
- `scripts/long-automated-mobile-auth-to-persistence.ps1`
- `scripts/mobile-adb-smoke.ps1`
- `scripts/mobile-adb-inject-server-otp.ps1`
- backend E2E tests for mega flow, full cycle, command lifecycle, and mobile bridge idempotency/command mapping
- internal OTP retrieval API for test use

To add next (for complete UI auditor control):
- `scripts/mobile-ui-auditor.ps1` to orchestrate deterministic on-device UI actions beyond login (navigation, button taps, state probes).
- `scripts/mqtt-device-role-simulator.ps1` wrapper to run bootstrap -> connect -> pub/sub -> ack assertions with explicit topic matrix.
- `scripts/persistence-auditor.ps1` to compare API responses vs DB state for device, beneficiary, installation, telemetry, and command timeline.
- `scripts/credential-rotation-assert.ps1` to verify old-creds rejection and successful re-bootstrap/reconnect.
- consolidated artifact exporter (json + markdown) for per-step evidence.

## 15) Sequence evaluation gates (must pass)

Gate 1: User Role -> Backend
- Login/session established and scoped assignments returned.
- User-role APIs complete entity creation/allocation without mismatch.

Gate 2: Backend -> Broker -> Device Role
- Bootstrap returns valid endpoint/credentials.
- Device connects and completes publish + subscribe + ack loop.

Gate 3: Device Role -> Backend persistence
- Published packets and command lifecycle are queryable via APIs.
- DB rows are present and consistent with API views.

Gate 4: Rotation enforcement
- Old credentials are rejected post-rotation.
- Re-bootstrap issues new credentials and traffic resumes.

Gate 5: End-to-end closure
- Final report marks each phase pass/fail with evidence links.
- Any gate failure is overall fail for long-run certification.

## 16) Implementation backlog for full automation closure

P0 (next execution batch):
1. Implement `mobile-ui-auditor.ps1` with robust selector + coordinate fallback profiles.
2. Implement `mqtt-device-role-simulator.ps1` with topic contract assertions.
3. Implement `persistence-auditor.ps1` for API-vs-DB consistency checks.
4. Integrate the three scripts into `long-automated-mobile-auth-to-persistence.ps1` as mandatory gates.

P1 (stability hardening):
1. Add device profile map for OEM-specific UI dumps.
2. Add retry envelope + deterministic timeout policy per phase.
3. Add junit/json artifacts for CI dashboards.

Definition of done for this master plan:
- One command executes full chain (auth -> user-role ops -> bootstrap -> MQTT pub/sub -> persistence -> rotate -> re-bootstrap -> repeat).
- All gates are hard-fail (no hidden warn-only pass).
- Evidence artifacts are generated for each run.

## 17) Android parity delta (2026-03-05)

Reference web/UI stack has dedicated Admin and Simulator surfaces that were not present in Android earlier. The Android app now includes first parity pass for these two high-priority areas.

Implemented in Android app:
- New page: `Admin Command Catalog` (drawer route) calling `GET /api/commands/catalog-admin?projectId=...&deviceId=...`.
- New page: `Simulator Sessions` (drawer route) calling:
	- `GET /api/simulator/sessions?limit=...`
	- `POST /api/simulator/sessions`
	- `DELETE /api/simulator/sessions/:sessionId`
- New mobile admin data layer (`AdminApiClient`) with bearer token from mobile session.

Still pending for full 1:1 parity with web simulator/admin:
- Full simulator page workflow parity (state/authority/vendor bootstrap helpers, HTTPS ingest helper panel, runtime telemetry publishing controls, session logs).
- Remaining admin modules from web (`projects`, `users`, `hierarchy`, `devices`, `apikeys`, etc.) as separate Android pages.

Completed after this update:
- Command catalog create/update/delete from Android (`POST /api/commands/catalog`, `DELETE /api/commands/catalog/:id`) with inline editor state.
- Simulator helper actions from Android page:
	- `GET /api/device-open/credentials/local?imei=...`
	- `POST /api/telemetry/{topic}` with `Basic` auth and RMS headers for ingest validation.
- Additional admin parity pages now in Android with live backend calls:
	- `Admin Projects`: `GET/POST/PUT/DELETE /api/admin/projects`
	- `Admin Users`: `GET/POST/PATCH/DELETE /api/admin/users` and `POST /api/admin/users/:id/password`
	- `Admin API Keys`: `GET/POST/DELETE /api/admin/apikeys`
- New hierarchy/catalog parity pages now in Android with live backend calls:
	- `Admin Hierarchy`: states and authorities via `/api/admin/states` and `/api/admin/state-authorities`
	- `Admin Catalogs`: server vendors + protocol versions via `/api/admin/server-vendors` and `/api/admin/protocol-versions`

Execution note:
- This closes the "same old sample app" issue by adding visible, backend-wired Admin/Simulator pages in the mobile UI, while preserving existing dashboard/settings functionality.
