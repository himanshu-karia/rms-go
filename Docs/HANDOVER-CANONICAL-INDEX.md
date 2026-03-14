# RMS-Go Canonical Index (Handover)

Use this file as the single source of truth for what is current vs archival for rms-go.

## Canonical docs (active)

### Status snapshot
- `PROJECT-STATUS.md`

### System-level handover set
- `SYSTEM-SPEC.md`
- `SYSTEM-STORY.md`
- `CRITICAL-FLOWS.md`
- `FLOW-SEQUENCES.md`
- `INNER-PATTERNS.md`
- `DECISION-LOG.md`

### Handover control set
- `HANDOVER-READINESS-CHECKLIST.md`
- `CONTRACT-TRACEABILITY-MATRIX.md`
- `DIAGRAM-BUILD-REFRESH-GUIDE.md`

### Product + architecture
- `01-rationale-and-decision.md`
- `02-target-architecture.md`
- `03-implementation-adaptations.md`
- `04-env-and-deploy.md`
- `08-risks-and-guardrails.md`
- `MANDATORY-SEED-CONTRACT.md`
- `MANDATORY-SEED-SAMPLE-KUSUMC.md`
- `PRODUCTION-ONLY-SEED-SET-KUSUMC.md`

### Validation + status
- `e2e-review-rms-go-2026-02-26.md`
- `rms-go-inventory-parity-verification.md`
- `govt-protocol-verification-kusumc.md`
- `reports/docs-audit-rms-go-2026-03-01.md`

### Platform capability catalog
- `GO-KUSUMC-EXTRA-FEATURES.md`

### Documentation governance
- `HANDOVER-CLEANUP-PLAN.md`
- `NEXT-AGENT-HANDOVER-EXECUTION.md`

### Firmware contracts (legacy govt protocol)
- `firmware-integration-kusumc-legacy-only/for-firmware-agent/00-index.md` (primary handoff path)
- `firmware-integration-kusumc-legacy-only/for-firmware-agent/03-mqtt-topics-and-payloads.md`
- `firmware-integration-kusumc-legacy-only/for-firmware-agent/04-rest-api-contract.md`

### Runbooks and scripts
- `../go-kusumc/scripts/README.md`
- `../go-kusumc/README.md`
- `../ui-kusumc/README.md`

## Working conventions for new contributors
- Prefer `docker-compose.integration.yml` for repeatable integration/E2E verification.
- Use compose-aware URL defaults from scripts; avoid hardcoding one base URL across profiles.
- Treat firmware legacy-only docs as normative for topic/payload behavior.
- Follow `HANDOVER-CLEANUP-PLAN.md` for archival/retention decisions.

## Archived (non-canonical)
- `reports/archive/05-test-plan-and-acceptance.md`
- `reports/archive/06-cutover-plan.md`
- `reports/archive/07-task-breakdown.md`
- `reports/archive/09-day1-docker-compose-blueprint.md`

Archive contents (backend planning snapshots):
- `../go-kusumc/docs/archive/`

These files are historical context only; do not use them as primary implementation guidance.
