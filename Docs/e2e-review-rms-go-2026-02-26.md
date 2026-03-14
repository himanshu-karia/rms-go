# RMS-Go End-to-End Validation Review (2026-02-26)

Scope audited:
- `rms-go/Docs/firmware-integration-kusumc-legacy-only` vs `rms-go/go-kusumc` code
- Backend (`go-kusumc`) ↔ Frontend (`ui-kusumc/version-a-frontend`) contract alignment
- Operational scripts for bring-up/down/status/stats portability
- System process chain: API → auth/ACL → broker sync → packet verification → DB persistence → analytics/rules/alerts/automation

Out-of-scope: FOTA (explicitly excluded).

## Addendum — ordered certification rerun (2026-02-26)

- Full deterministic ordered suite was rerun using `go-kusumc/scripts/run-e2e-ordered.ps1` on `docker-compose.integration.yml`.
- Result: **PASS** across all 13 ordered targets, with expected optional-env skips only:
  - `TestBootstrapConnectPersist` (requires `BOOTSTRAP_URL`, `BOOTSTRAP_IMEI`, `TIMESCALE_URI`)
  - `TestLiveBootstrapTLS` (requires `BOOTSTRAP_URL`, `BOOTSTRAP_IMEI`)
- Harness refinements validated in this rerun:
  1. API readiness probe now uses HTTP-status-tolerant probing (`-SkipHttpErrorCheck`) so readiness is not blocked by expected auth response codes.
  2. Ordered tests execute via compose `test-runner` network context when available, preventing host/container DNS mismatch (for example `emqx` resolution).

## 1) Summary verdict

- **Overall architecture is coherent and mostly wired end-to-end** for legacy RMS operations.
- **Critical issues found and fixed in this pass:**
  1. Live telemetry token was issued but not enforced on stream endpoint.
  2. Duplicate route registration for telemetry latest endpoint created handler ambiguity.
  3. Linux-native operational scripts for bringup/bringdown/status/stats were missing.
- **Follow-up hardening fixes applied after initial pass:**
  4. Live telemetry token storage moved to Redis-backed TTL keys (with in-memory fallback).
  5. Ingestion buffer overflow now records dead-letter payloads and overflow counters in Redis.
  6. MQTT `/pump` subscription now profile-gated (`MQTT_TOPIC_PROFILE`), with strict data-only default.
  7. Dead-letter replay worker + protected diagnostics replay/status APIs added.

## 2) What was validated

### A) Firmware docs vs backend code

Validated against:
- MQTT ingress: `go-kusumc/internal/adapters/primary/mqtt_handler.go`
- Ingestion normalization/verification/persistence: `go-kusumc/internal/core/services/ingestion_service.go`
- Bootstrap and credential topic generation: `go-kusumc/internal/core/services/bootstrap_service.go`
- ACL provisioning worker: `go-kusumc/internal/core/workers/mqtt_worker.go`

Result:
- Docs are aligned on firmware contract: PumpData on `<imei>/data` only.
- Backend remains compatibility-tolerant for `<imei>/pump` ingestion/ACL generation fallback paths.
- Added explicit compatibility note in docs to avoid ambiguity.

### B) Backend ↔ frontend contract

Validated against:
- Frontend transport/config: `ui-kusumc/version-a-frontend/src/api/http.ts`, `src/api/config.ts`, `vite.config.ts`
- Frontend telemetry/rules/alerts clients: `src/api/telemetry.ts`, `src/api/rules.ts`, `src/api/alerts.ts`
- Backend route wiring: `go-kusumc/cmd/server/main.go`, `internal/adapters/primary/http/*`

Result:
- API base/proxy alignment is correct (`/api` + Vite proxy to `https://localhost`).
- Query/body key-shape interop is robust (frontend camel→snake + backend aliasing).
- Rules/alerts/telemetry endpoint families are present and aligned.

### C) Infrastructure process chain

Validated components:
- Reverse proxy + TLS + WSS/MQTTS: `go-kusumc/infra/nginx/nginx.conf`
- Broker bootstrapper path: `go-kusumc/cmd/bootstrap_emqx/main.go`
- EMQX adapter provisioning/ACL: `go-kusumc/internal/adapters/secondary/emqx_adapter.go`
- Rules/alerts/automation path: `go-kusumc/internal/core/services/rules_service.go`, `device_service.go`
- Persistence path: `go-kusumc/internal/adapters/secondary/postgres_repo.go`

Result:
- API/Broker/DB/Rules chain is implemented and connected.
- Auth + ACL sync flow exists and is operationally coherent.

## 3) Tests run in this review

Passed:
- `internal/core/services/ingestion_service_test.go` (14 passed)
- `internal/core/services/commands_service_test.go` (7 passed)
- `internal/adapters/primary/http/router_bootstrap_test.go` + `router_govt_import_test.go` (12 passed)
- `internal/adapters/primary/http` package tests (85 passed)

Note:
- Frontend test invocation through current test runner integration did not discover selected TS test files via this environment tooling (manual `npm test` still recommended in `ui-kusumc/version-a-frontend`).

## 4) Issues found and actions

### Fixed in this pass

1. **Live stream token enforcement gap (HIGH)**
- Before: `/telemetry/devices/:device_uuid/live-token` issued tokens, but SSE stream did not validate them.
- Fix:
  - Added token validation in `go-kusumc/internal/adapters/primary/http/telemetry_live_controller.go`
  - Added expiring in-memory token store in controller
  - Updated frontend stream URL to send token in query: `ui-kusumc/version-a-frontend/src/api/telemetry.ts`

2. **Duplicate route registration (HIGH)**
- Before: `/api/telemetry/devices/:device_uuid/latest` registered twice with different handlers.
- Fix:
  - Removed duplicate inline handler in `go-kusumc/cmd/server/main.go`
  - Kept canonical `analyticsController.GetLatest` binding.

3. **Linux-native operations scripts missing (MEDIUM)**
- Added:
  - `go-kusumc/scripts/bringup.sh`
  - `go-kusumc/scripts/bringdown.sh`
  - `go-kusumc/scripts/status.sh`
  - `go-kusumc/scripts/stats.sh`
- Updated `go-kusumc/scripts/README.md` with Linux/macOS usage and compose-file override.

4. **Docs compatibility clarity (LOW/MEDIUM)**
- Updated `rms-go/Docs/firmware-integration-kusumc-legacy-only/for-firmware-agent/03-mqtt-topics-and-payloads.md`
- Added explicit note: backend may accept legacy `/pump`, but firmware contract is PumpData on `/data` only.

5. **Live token persistence across replicas (HIGH)**
- Before: tickets were in-memory only.
- Fix:
  - `telemetry_live_controller` now stores tokens in Redis (`telemetry:live:ticket:<token>`) with TTL.
  - Validation reads Redis first; in-memory map is fallback-only.

6. **Ingest overflow observability (HIGH)**
- Before: overflow packets were dropped with no recovery trail.
- Fix:
  - Added Redis dead-letter queue writes (`ingest:deadletter`) on overflow.
  - Added Redis counters (`metrics:ingest:buffer_overflow_total`, `metrics:ingest:buffer_overflow_hourly`).

7. **Profile-gated `/pump` compatibility (MEDIUM)**
- Before: runtime MQTT subscription always included `+/pump`.
- Fix:
  - Added `MQTT_TOPIC_PROFILE` with default `strict_data_only`.
  - `legacy_compat` enables `+/pump` subscription explicitly.
  - Updated service ACL/bootstrap defaults to data-only subscriptions.

8. **Dead-letter replay capability (MEDIUM → mitigated)**
- Added worker: `internal/core/workers/deadletter_replay_worker.go`.
- Added protected diagnostics APIs:
  - `GET /api/diagnostics/ingest/deadletter`
  - `POST /api/diagnostics/ingest/deadletter/replay`
- Added Redis list helpers for replay operations in `internal/adapters/secondary/redis_store.go`.

## 5) Remaining high-risk conditions and recommended fixes

1. **Dead-letter replay policy defaults now set; per-site tuning remains (LOW/MEDIUM)**
- Replay env defaults are now explicitly set in compose for local/integration profiles.
- Operator runbook now documents status/replay endpoints and tuning knobs.
- Remaining action: tune values per real device volume and DB latency envelope in production.

2. **Compatibility subscriptions still environment-driven (MEDIUM)**
- Profile gating is now available, but deployment envs must explicitly choose profile.
- Recommended fix: enforce profile per environment class (integration/prod) in deployment manifests and CI checks.

3. **PowerShell integration script quality issue (MEDIUM)**
- `scripts/run-integration.ps1` still contains duplicated structure.
- Recommended fix: clean/deduplicate `run-integration.ps1`, and add Linux `.sh` equivalent for parity.

4. **End-to-end observability confidence (MEDIUM)**
- Many processes are wired, but cause/effect tracing is still mostly manual.
- Recommended fix: add correlation-id propagation dashboard and synthetic health checks for:
  - API command issue
  - broker ACL apply
  - device ack
  - telemetry persistence
  - alert emission

## 6) Recommended next execution plan

1. Add Linux parity scripts for integration and smoke (`run-integration.sh`, `smoke.sh`).
2. Validate dead-letter replay defaults under expected production-like load and lock target values.
3. Add a CI job to run:
   - `go test ./...` (or scoped matrix)
   - frontend `npm test`
   - one integration compose smoke path.
4. Add deployment guardrails that require explicit `MQTT_TOPIC_PROFILE` value.
5. Add a small architecture verification checklist doc for each release.

## 7) File change log from this review pass

- `go-kusumc/internal/adapters/primary/http/telemetry_live_controller.go`
- `go-kusumc/cmd/server/main.go`
- `ui-kusumc/version-a-frontend/src/api/telemetry.ts`
- `go-kusumc/internal/core/services/ingestion_service.go`
- `go-kusumc/internal/adapters/secondary/redis_store.go`
- `go-kusumc/internal/adapters/primary/mqtt_handler.go`
- `go-kusumc/internal/core/workers/deadletter_replay_worker.go`
- `go-kusumc/internal/adapters/primary/http/diagnostics_controller.go`
- `go-kusumc/cmd/bootstrap_emqx/main.go`
- `go-kusumc/docker-compose.yml`
- `go-kusumc/docker-compose.integration.yml`
- `go-kusumc/scripts/bringup.sh`
- `go-kusumc/scripts/bringdown.sh`
- `go-kusumc/scripts/status.sh`
- `go-kusumc/scripts/stats.sh`
- `go-kusumc/scripts/README.md`
- `Docs/firmware-integration-kusumc-legacy-only/for-firmware-agent/03-mqtt-topics-and-payloads.md`
