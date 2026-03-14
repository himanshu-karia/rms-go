# Next Agent Handover Execution Prompt

Last updated: 2026-03-01

Use this as the exact instruction set for the next implementation agent.

## Mission
Finalize RMS-Go handover signoff using the canonical docs and produce one consolidated, evidence-backed handover report for receiving teams.

## Scope boundaries (strict)
Work only inside:
- `rms-go/go-kusumc`
- `rms-go/ui-kusumc`
- `rms-go/Docs`

Treat as reference-only (do not modify):
- `unified-go/`
- `new-frontend/`
- `refer-rms-deploy/`
- workspace-root planning notes outside `rms-go/`

## Canonical docs to use
- `Docs/HANDOVER-CANONICAL-INDEX.md`
- `Docs/HANDOVER-READINESS-CHECKLIST.md`
- `Docs/CONTRACT-TRACEABILITY-MATRIX.md`
- `Docs/DIAGRAM-BUILD-REFRESH-GUIDE.md`
- `Docs/PROJECT-STATUS.md`
- `go-kusumc/README.md`
- `go-kusumc/scripts/README.md`

## Required execution tasks
1. **Run readiness checklist end-to-end**
   - Evaluate each checklist item in `Docs/HANDOVER-READINESS-CHECKLIST.md`.
   - For each item: mark status (`PASS` / `GAP` / `N/A`) and attach concrete evidence reference.

2. **Validate runtime profiles and scripts**
   - Confirm core runtime path works without LoRaWAN profile.
   - Confirm `lorawan` profile path includes ChirpStack services.
   - Confirm PowerShell + Bash lifecycle helper parity remains valid.

3. **Validate contract traceability integrity**
   - Verify each row in `Docs/CONTRACT-TRACEABILITY-MATRIX.md` maps to:
     1) canonical contract source,
     2) actual runtime enforcement point,
     3) test/verification evidence.
   - Flag any broken links, stale assertions, or missing evidence.

4. **Validate diagram hygiene**
   - Ensure active `.mmd` sources have corresponding `.svg` exports in canonical diagram folders.
   - If any are out of date, regenerate per `Docs/DIAGRAM-BUILD-REFRESH-GUIDE.md`.

5. **Produce final handover signoff report**
   - Create a dated report under `Docs/reports/` named:
     - `handover-signoff-rms-go-YYYY-MM-DD.md`
   - Include: summary, checklist outcomes, verification commands run, evidence links, open risks, owner assignments, and recommended next 7-day actions.

## Deliverables (must produce all)
- Updated `Docs/HANDOVER-READINESS-CHECKLIST.md` with explicit status annotations.
- Updated `Docs/CONTRACT-TRACEABILITY-MATRIX.md` if mismatches are discovered.
- Updated diagram exports if required.
- New signoff report in `Docs/reports/`.
- Brief final summary message listing:
  - what passed,
  - what remains open,
  - handover go/no-go recommendation.

## Guardrails
- Do not rewrite large docs unnecessarily; apply focused consolidation only.
- Do not introduce new architecture features during handover validation.
- Keep changes auditable and evidence-linked.
- If any item cannot be validated, mark it clearly as `GAP` with reason and owner.

## Definition of done
Handover is considered complete when:
- checklist items are statused with evidence,
- traceability matrix is current,
- diagram source/export parity is confirmed,
- final signoff report exists with clear go/no-go decision and open-risk ownership.
