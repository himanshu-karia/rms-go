# RMS-Go Handover Cleanup Plan

Goal: make `rms-go/` stand on its own without requiring context from sibling folders.

## Phase 1 — Canonicalization (done)
- Added top-level `rms-go/README.md`
- Added `Docs/HANDOVER-CANONICAL-INDEX.md`
- Updated `Docs/README.md` to mark external same-topic docs as non-canonical
- Cleaned `go-kusumc/README.md` and fixed primary firmware-doc pointers

## Phase 2 — Archive/retire noisy docs (completed)
Moved historical/planning-heavy docs into `Docs/reports/archive/`:
- `reports/archive/05-test-plan-and-acceptance.md`
- `reports/archive/06-cutover-plan.md`
- `reports/archive/07-task-breakdown.md`
- `reports/archive/09-day1-docker-compose-blueprint.md`

Keep active operational docs in place:
- `01-rationale-and-decision.md`
- `02-target-architecture.md`
- `03-implementation-adaptations.md`
- `04-env-and-deploy.md`
- `08-risks-and-guardrails.md`
- `e2e-review-rms-go-2026-02-26.md`
- `govt-protocol-verification-kusumc.md`
- `rms-go-inventory-parity-verification.md`

## Phase 3 — Single “operator path” docs (recommended)
Add/maintain one doc per concern, and avoid duplicates:
- Build/run: `go-kusumc/README.md`, `go-kusumc/scripts/README.md`
- Frontend run/test: `ui-kusumc/README.md`
- Firmware contract: `Docs/firmware-integration-kusumc-legacy-only/for-firmware-agent/*`
- Architecture + parity: `Docs/HANDOVER-CANONICAL-INDEX.md` + validation reports

## Phase 4 — Boundary rule
For KUSUMC RMS work, treat these folders as read-only references only:
- `unified-go/`
- `new-frontend/`
- `refer-rms-deploy/`
- workspace-root planning notes (`*.md` at root)

All new RMS implementation docs should be created under `rms-go/Docs/`.
