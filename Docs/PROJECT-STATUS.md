# RMS-Go Project Status

Last updated: 2026-03-01

## Scope
This status applies to the standalone `rms-go/` workspace:
- `rms-go/go-kusumc`
- `rms-go/ui-kusumc`
- `rms-go/Docs`

## Runtime model
- Target runtime: Linux containers via Docker Compose.
- Host OS (Windows/Linux/macOS) is orchestration only.
- Deployment shape is multi-service (Go API + EMQX + Redis + Timescale + Nginx).

## Current health
- Backend compile: **PASS**
  - `go build ./cmd/server` succeeded.
- Integration harness (`run-integration.ps1/.sh`): **PASS** on integration profile for representative E2E target
  - `TestDeviceOpenAliasCoverage` passed with `docker-compose.integration.yml`.
- Ordered deterministic harness: **PASS (full certification run captured)**
  - `scripts/run-e2e-ordered.ps1`
  - `scripts/run-e2e-ordered.sh`
  - default profile: `docker-compose.integration.yml`
  - Latest full run (2026-02-26): 13/13 ordered tests completed with expected scenario skips only (`TestBootstrapConnectPersist`, `TestLiveBootstrapTLS` when optional envs are not provided).

## Important fixes completed in this cycle
- Dead-letter replay worker + diagnostics endpoints added and wired.
- Live telemetry token enforcement and Redis-backed ticket validation in place.
- Ingest overflow now records dead-letter queue + counters.
- MQTT profile gating added (`MQTT_TOPIC_PROFILE`, strict data-only default).
- Linux parity scripts added for bringup/bringdown/status/stats/integration/smoke.
- Integration runners hardened:
  - fail-fast behavior on compose/test failures
  - no teardown in skip-compose mode
  - compose-aware URL defaults
  - fixture seeding for host fallback path
  - ordered runner readiness probe now handles non-2xx readiness statuses safely
  - ordered runner executes tests in compose `test-runner` network context when available (avoids host/container DNS mismatch)
- Compose profile split validated for dormant LoRaWAN behavior in RMS core mode:
  - default `docker-compose.yml` path runs without ChirpStack containers
  - explicit `--profile lorawan` enables ChirpStack stack
- PowerShell + Bash helper parity added for core/lorawan up/down lifecycle scripts
- Focused handover consolidation docs added:
  - `HANDOVER-READINESS-CHECKLIST.md`
  - `CONTRACT-TRACEABILITY-MATRIX.md`
  - `DIAGRAM-BUILD-REFRESH-GUIDE.md`

## Known caveats
- Environment-level port conflicts can occur if other EMQX stacks are already bound (for example dashboard port `18083`).

## Recommended handover entry sequence
1. `HANDOVER-CANONICAL-INDEX.md`
2. `PROJECT-STATUS.md`
3. `../go-kusumc/README.md`
4. `../go-kusumc/scripts/README.md`
5. Firmware primary: `firmware-integration-kusumc-legacy-only/for-firmware-agent/00-index.md`

## Next verification step (optional)
- Re-run ordered suite after any compose/env profile change and append a dated result note under `Docs/reports/`.
