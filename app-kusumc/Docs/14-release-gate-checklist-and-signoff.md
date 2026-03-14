# Release Gate Checklist and Sign-off Matrix (PLAN-03)

Date: 2026-03-04

## 1) Release gate checklist

- [ ] Backend mobile APIs deployed to staging from latest source
- [ ] Mobile auth + assignment flow validated on debug build
- [ ] Offline outbox + sync worker flow validated
- [ ] Command transport HTTP-first with fallback path validated
- [ ] Credential rotation drop-and-rebootstrap flow validated
- [ ] Mobile bridge integration tests pass (`QA-03` non-skip)
- [ ] UAT rubric pass (`QA-04`) with no P0 failures
- [ ] Ops alerts and replay runbook reviewed (`OPS-01..03`)
- [ ] Security controls reviewed (TLS policy + hostname checks)
- [ ] Rollback path reviewed and rehearsed

## 2) Sign-off matrix

| Area | Owner | Status | Evidence |
|---|---|---|---|
| Backend bridge APIs | Engineering | Pending | Integration test logs |
| Android app flow | Engineering | Pending | APK + test logs |
| Security policy | Engineering/Security | Pending | TLS policy review notes |
| QA/UAT | QA/Product | Pending | UAT rubric output |
| Operations readiness | Ops/SRE | Pending | Runbook + alert config |
| Go-live approval | Product/Ops | Pending | Final gate meeting notes |

## 3) Mandatory evidence links

- `Docs/09-test-strategy-and-uat.md`
- `Docs/11-operational-runbook.md`
- `Docs/13-internal-test-automation-and-sequence.md`
- `scripts/long-automated-mobile-auth-to-persistence.ps1`

## 4) Go/No-Go rule

Go only if all P0 checks are completed and signed-off with evidence.
No-Go if any P0 check fails or if replay/runbook readiness is incomplete.
