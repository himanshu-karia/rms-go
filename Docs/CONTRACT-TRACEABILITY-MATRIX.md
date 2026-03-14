# RMS-Go Contract Traceability Matrix

Last updated: 2026-03-01

Purpose: map canonical contracts to runtime enforcement and verification evidence so handover has explicit coverage.

| Area | Canonical contract source | Runtime enforcement location | Verification evidence / runner |
|---|---|---|---|
| Firmware legacy topic + payload path | `Docs/firmware-integration-kusumc-legacy-only/for-firmware-agent/03-mqtt-topics-and-payloads.md` | `go-kusumc` ingestion + MQTT subscription pipeline (compose runtime defaults) | `scripts/run-integration.ps1/.sh`, `scripts/run-e2e-ordered.ps1/.sh`, `Docs/e2e-review-rms-go-2026-02-26.md` |
| REST auth/bootstrap/device API contract | `Docs/firmware-integration-kusumc-legacy-only/for-firmware-agent/04-rest-api-contract.md` | Go HTTP handlers + middleware chain in `go-kusumc` | `scripts/smoke.ps1/.sh`, integration E2E runners |
| Backend payload schema expectations | `go-kusumc/docs/payload-contract.md` | Ingest processing, validation, and persistence paths | Unit/integration tests in `go-kusumc`, protocol verification report |
| Bootstrap and MQTT credential contract | `go-kusumc/docs/mqtt-bootstrap-contract.md` | Bootstrap endpoint + EMQX bootstrapper/provisioning flow | Bootstrap/connect sequence docs + integration runs |
| OpenAPI reference surface | `go-kusumc/docs/openapi-dna.yaml` | API implementation in `go-kusumc` server routes | API smoke and E2E flows; receiving-team contract review |
| Seed data minimum runtime contract | `Docs/MANDATORY-SEED-CONTRACT.md` | SQL bootstrap + seed scripts + compose setup | `Docs/MANDATORY-SEED-SAMPLE-KUSUMC.md`, integration setup scripts |
| Production-only curated seed policy | `Docs/PRODUCTION-ONLY-SEED-SET-KUSUMC.md` | Deployment-time seeding strategy | Environment/deploy guidance + operator checklist |
| Govt protocol alignment | `Docs/govt-protocol-verification-kusumc.md` | Legacy firmware-compatible ingest and data path | Govt protocol verification report |
| Core RMS runtime without LoRaWAN | `go-kusumc/README.md`, `go-kusumc/scripts/README.md` | Compose profile split (`core` default, `lorawan` optional) | `up-core` / `up-lorawan` script verification evidence |

## Handover expectation
For each future contract change, update all three artifacts in the same pull request:
1. Canonical contract document
2. Runtime enforcement code/config
3. Verification evidence (test/report/update note)
