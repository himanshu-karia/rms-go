# RMS-GO (KUSUMC) — Documentation Hub

This folder contains architecture, protocol, validation, and handover docs for the active rms-go product workspace.

## First read
- `HANDOVER-CANONICAL-INDEX.md` (authoritative index for active vs archival docs)
- `NEXT-AGENT-HANDOVER-EXECUTION.md` (copy-paste execution prompt for final handover signoff)
- `PROJECT-STATUS.md` (current build/test/runtime snapshot)
- `DECISION-LOG.md` (ADR-style rationale for major technical choices)
- `HANDOVER-READINESS-CHECKLIST.md` (final pre-handover gate)
- `CONTRACT-TRACEABILITY-MATRIX.md` (contract → enforcement → verification mapping)
- `DIAGRAM-BUILD-REFRESH-GUIDE.md` (Mermaid source/export runbook)
- `MANDATORY-SEED-CONTRACT.md` (minimum runtime data/provisioning contract)
- `MANDATORY-SEED-SAMPLE-KUSUMC.md` (exact seeded sample values in SQL + runtime seeder)
- `PRODUCTION-ONLY-SEED-SET-KUSUMC.md` (curated production subset excluding demo/sample data)

## Core doc groups
- Product/architecture: `01-*` to `04-*`, `08-risks-and-guardrails.md`
- Validation/status: `e2e-review-rms-go-2026-02-26.md`, `rms-go-inventory-parity-verification.md`, `govt-protocol-verification-kusumc.md`
- Go platform extras visibility: `GO-KUSUMC-EXTRA-FEATURES.md`
- Firmware contracts:
  - Primary: `firmware-integration-kusumc-legacy-only/for-firmware-agent/`
  - Secondary/reference: `firmware-integration-kusumc/`

## Scope note
- Runtime and development scope here is limited to:
  - `rms-go/go-kusumc`
  - `rms-go/ui-kusumc`
  - `rms-go/Docs`
- Any similarly named docs outside `rms-go/` are non-canonical for KUSUMC handover.
