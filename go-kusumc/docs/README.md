# go-kusumc docs

This folder contains backend contracts and runbooks for the active go-kusumc implementation.

Canonical product-level handoff docs live in `rms-go/Docs/`.

See also: top-level `../README.md` → **Tests** for quick run paths (this file remains canonical for env overrides).

For custom local URL bootstrap (`rms-iot.local`), run:

```pwsh
cd rms-go\go-kusumc
.\scripts\setup-custom-local-domain.ps1
```

## Test env overrides (PowerShell)
For integration/long E2E runs against local docker-backed stack:

```pwsh
cd rms-go\go-kusumc
$env:BASE_URL='https://rms-iot.local:7443'
$env:BOOTSTRAP_URL='https://rms-iot.local:7443/api/bootstrap'
$env:PROJECT_ID='pm-kusum-solar-pump-msedcl'
$env:TIMESCALE_URI='postgres://postgres:password@localhost:5433/telemetry?sslmode=disable'
$env:MQTT_BROKER='mqtts://rms-iot.local:18883'
go test -tags=integration ./tests/e2e -run 'TestDeviceCommandLifecycle|TestKusumFullCycle|TestRMSMegaFlow|TestSolarRMSFullCycle|TestStory_FullCycle|TestUIAndDeviceOpenFullCycle' -count=1 -v
```

Before `docker compose up`, clear host-local DB override to prevent container runtime from inheriting host endpoint:

```pwsh
Remove-Item Env:TIMESCALE_URI -ErrorAction SilentlyContinue
```

## Active backend docs
- `provisioning-emqx-acl.md` — provisioning flow and EMQX ACL contract
- `dataflow-telemetry-archive.md` — telemetry ingest, persistence, archive, and rehydrate flow
- `payload-contract.md` — ingestion envelope and payload contract
- `data-topic-migration-2026-03-02.md` — canonical telemetry topic/type contract (`data`)
- `mqtt-bootstrap-contract.md` — bootstrap payload and topic contract
- `rules-virtual-sensors.md` — virtual sensor and rules execution behavior
- `rms_apis.md` — backend API contracts for provisioning/bootstrap/commands
- `operator-playbook-dna.md` — operator runbook for DNA/config workflows
- `openapi-dna.yaml` — API schema for DNA endpoints

## Reference docs
- `gaps-assumptions.md` — integration assumptions and implementation notes
- `rms-deploy-comparison.md` — historical architecture comparison reference
- `core-extension-doc/codegen-plan.md` — future deterministic codegen plan
