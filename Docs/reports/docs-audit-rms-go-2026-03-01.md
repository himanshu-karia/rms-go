# RMS-Go Docs Audit (Manual) — 2026-03-01

## Scope reviewed
- `rms-go/Docs/*`
- `rms-go/Docs/firmware-*/*`
- `rms-go/go-kusumc/docs/*`

This audit was performed by manual reading and cross-checking against canonical handover docs.

## Executive conclusion
- Core handover docs are in good shape and mostly non-redundant.
- Historical/planning docs in `go-kusumc/docs/` that were superseded by canonical handover docs and current runbooks have now been archived.
- Result: cleanup completed via **archive (not delete)**.

## Essential (keep active)
- `Docs/HANDOVER-CANONICAL-INDEX.md`
- `Docs/PROJECT-STATUS.md`
- `Docs/HANDOVER-READINESS-CHECKLIST.md`
- `Docs/CONTRACT-TRACEABILITY-MATRIX.md`
- `Docs/DIAGRAM-BUILD-REFRESH-GUIDE.md`
- `Docs/SYSTEM-SPEC.md`, `Docs/SYSTEM-STORY.md`, `Docs/CRITICAL-FLOWS.md`, `Docs/FLOW-SEQUENCES.md`
- `Docs/01-rationale-and-decision.md`, `02-target-architecture.md`, `03-implementation-adaptations.md`, `04-env-and-deploy.md`, `08-risks-and-guardrails.md`
- Firmware canonical path: `Docs/firmware-integration-kusumc-legacy-only/for-firmware-agent/*`
- Runtime runbooks: `go-kusumc/README.md`, `go-kusumc/scripts/README.md`
- Backend reference contracts: `go-kusumc/docs/payload-contract.md`, `go-kusumc/docs/mqtt-bootstrap-contract.md`, `go-kusumc/docs/provisioning-emqx-acl.md`, `go-kusumc/docs/dataflow-telemetry-archive.md`, `go-kusumc/docs/rules-virtual-sensors.md`

## Keep as supporting references (not primary)
- `Docs/firmware-integration-kusumc/*` (wider migration context; not primary firmware handoff set)
- `Docs/firmware-old-to-new-legacy/*` (compact/compat migration addendum)
- `go-kusumc/docs/operator-playbook-dna.md` (operationally useful DNA lifecycle runbook)
- `go-kusumc/docs/openapi-dna.yaml` (API reference artifact)

## Superseded docs archived (completed)
These planning snapshots and migration-era drafts have been moved to archive:

1. `go-kusumc/docs/archive/fixing-plan-26-12-25.md`
2. `go-kusumc/docs/archive/future-multi-project-plan.md`
3. `go-kusumc/docs/archive/project-dna-unification-plan.md`
4. `go-kusumc/docs/archive/provisioning-plan.md`
5. `go-kusumc/docs/archive/provision-upgrade.md`
6. `go-kusumc/docs/archive/rms-final-plan.md`
7. `go-kusumc/docs/archive/rms-migration-matrix.md`
8. `go-kusumc/docs/archive/deferredForV3.md`
9. `go-kusumc/docs/archive/pending-2-1-26.md`
10. `go-kusumc/docs/archive/smoke_pm_kusum_solar_pump_msedcl.md`
11. `go-kusumc/docs/archive/security-plan-v0.md`

Archive location:
- `go-kusumc/docs/archive/`

## Delete candidates
- No hard-delete recommendation in this cycle.
- Prefer archival retention for traceability during handover.

## Alignment fixes applied during audit
- Fixed stale/non-local references in:
  - `Docs/firmware-integration-kusumc/00-index.md`
  - `Docs/firmware-integration-kusumc-legacy-only/00-index.md`
- Updated handover control path and next-agent execution references in canonical docs.

## Suggested next action
- Keep archived files read-only unless an audit specifically requires historical reconstruction.
