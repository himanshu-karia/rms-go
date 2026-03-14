# RMS-Go Decision Log (ADR-style)

Date baseline: 2026-02-26

This log captures high-impact decisions that affect behavior, reliability, and handover clarity.

---

## ADR-001 — Legacy-first protocol contract for KUSUMC

### Context
KUSUMC firmware is tied to government/legacy MQTT patterns and cannot be treated as a rapidly evolving contract.

### Decision
Treat legacy topic/payload behavior as primary contract for RMS-Go. Compatibility overlays are explicit, not default.

### Consequence
- Reduced protocol drift risk for field firmware.
- Cleaner expectations for firmware and operations teams.

### Evidence
- `firmware-integration-kusumc-legacy-only/for-firmware-agent/03-mqtt-topics-and-payloads.md`
- `SYSTEM-SPEC.md`

---

## ADR-002 — Profile-gated MQTT compatibility

### Context
Unconditional compatibility subscriptions (`+/pump`) can unintentionally broaden behavior in environments meant to be strict.

### Decision
Introduce `MQTT_TOPIC_PROFILE` and default to strict data-only behavior; enable compat only when explicitly set.

### Consequence
- Safer defaults for production.
- Controlled enablement of legacy compatibility where needed.

### Evidence
- `go-kusumc/internal/adapters/primary/mqtt_handler.go`
- `go-kusumc/docker-compose.yml`
- `go-kusumc/docker-compose.integration.yml`

---

## ADR-003 — Live telemetry stream authorization must be enforced

### Context
Token issuance existed, but stream access validation had gaps and in-memory only state was fragile across replica/process boundaries.

### Decision
Enforce stream ticket validation and store tickets in Redis with TTL (in-memory fallback only).

### Consequence
- Prevents unauthorized stream usage.
- Better behavior in scaled/multi-instance topologies.

### Evidence
- `go-kusumc/internal/adapters/primary/http/telemetry_live_controller.go`
- `ui-kusumc/version-a-frontend/src/api/telemetry.ts`

---

## ADR-004 — Overflow is recoverable workload, not silent loss

### Context
Ingest buffer pressure previously caused unrecoverable drops without actionable trace/replay path.

### Decision
On overflow, write to dead-letter queue + counters; add replay worker and diagnostics endpoints for operator recovery.

### Consequence
- Operational recovery path exists.
- Better observability under pressure conditions.

### Evidence
- `go-kusumc/internal/core/services/ingestion_service.go`
- `go-kusumc/internal/adapters/secondary/redis_store.go`
- `go-kusumc/internal/core/workers/deadletter_replay_worker.go`
- `go-kusumc/internal/adapters/primary/http/diagnostics_controller.go`

---

## ADR-005 — Integration runner must be runtime-image agnostic

### Context
Running `go test` inside runtime service fails when runtime image intentionally excludes Go toolchain.

### Decision
Integration runners execute tests in this order:
1) `test-runner` service,
2) `ingestion-go` if Go exists,
3) host fallback with fixture seeding.

### Consequence
- Robust integration execution independent of runtime image internals.
- Lower maintenance friction for CI and local runs.

### Evidence
- `go-kusumc/scripts/run-integration.ps1`
- `go-kusumc/scripts/run-integration.sh`
- `go-kusumc/scripts/README.md`

---

## ADR-006 — Compose-aware URL defaults to eliminate false-negative E2E failures

### Context
Different compose profiles expose API via different host paths (`https://localhost` behind nginx vs direct `http://localhost:8081`).

### Decision
Set script URL defaults based on selected compose profile and add explicit API readiness checks in ordered harness.

### Consequence
- Fewer environment-caused test failures.
- Predictable harness behavior across profiles.

### Evidence
- `go-kusumc/scripts/run-integration.ps1`
- `go-kusumc/scripts/run-integration.sh`
- `go-kusumc/scripts/run-e2e-ordered.ps1`
- `go-kusumc/scripts/run-e2e-ordered.sh`

---

## ADR-007 — Canonical/archival documentation split for handover

### Context
Planning-era and active operational docs were mixed, increasing onboarding ambiguity.

### Decision
Create canonical index + status + system docs and move older planning docs into archive.

### Consequence
- Clearer handover path.
- Reduced risk of conflicting instructions.

### Evidence
- `HANDOVER-CANONICAL-INDEX.md`
- `PROJECT-STATUS.md`
- `SYSTEM-SPEC.md`
- `SYSTEM-STORY.md`
- `CRITICAL-FLOWS.md`
- `FLOW-SEQUENCES.md`
- `INNER-PATTERNS.md`
- `reports/archive/README.md`

---

## ADR-008 — Deterministic ordered E2E harness for phase-wise verification

### Context
Package-level test runs do not enforce business-flow order and can hide phase-specific regressions.

### Decision
Add ordered harness scripts that run selected E2E tests in explicit sequence.

### Consequence
- Better stage-by-stage validation for handover and release checks.
- Easier pinpointing of first failing business stage.

### Evidence
- `go-kusumc/scripts/run-e2e-ordered.ps1`
- `go-kusumc/scripts/run-e2e-ordered.sh`

