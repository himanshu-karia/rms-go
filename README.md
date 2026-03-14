# rms-go — Standalone KUSUMC RMS Workspace

This folder is intended to be self-contained for the KUSUMC RMS product line.

Contains:
- `go-kusumc/` — Go backend runtime + APIs + MQTT ingest + scripts
- `ui-kusumc/` — RMS frontend app
- `Docs/` — canonical architecture, protocol, test, and handover documentation

## Runtime model
- Target runtime is Linux containers via Docker Compose.
- Host OS (Windows/Linux/macOS) is only for orchestration and local developer tooling.
- Backend application is a single Go service, but deployment is a multi-service stack (Go API + EMQX + Redis + Timescale + Nginx).

## Start here (handover)
1. `Docs/HANDOVER-CANONICAL-INDEX.md`
2. `go-kusumc/README.md`
3. `go-kusumc/scripts/README.md`
4. `ui-kusumc/README.md`

## Integration run (recommended)
- PowerShell:
  - `cd rms-go/go-kusumc`
  - `./scripts/run-integration.ps1 -ProjectRoot . -ComposeFile docker-compose.integration.yml -TestName TestDeviceLifecycle`
- Bash:
  - `cd rms-go/go-kusumc`
  - `./scripts/run-integration.sh --compose-file docker-compose.integration.yml --test TestDeviceLifecycle`

## Ordered E2E run
- PowerShell: `./scripts/run-e2e-ordered.ps1 -ProjectRoot .`
- Bash: `./scripts/run-e2e-ordered.sh`
