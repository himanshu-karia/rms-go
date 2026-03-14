# Scripts

This folder contains operational wrappers for lifecycle, integration, and smoke flows.

## LoRaWAN / ChirpStack behavior in RMS
- `docker-compose.yml` keeps ChirpStack services dormant by default.
- Core RMS startup does **not** bring up `chirpstack`, `chirpstack-gateway-bridge`, `chirpstack-postgres`, or `chirpstack-redis`.
- To include LoRaWAN services, use the `lorawan` profile explicitly.

## Compose lifecycle wrappers

### Core RMS only (default, no LoRaWAN)
- PowerShell: `up-core.ps1` (up), `down-core.ps1` (down)
- Bash: `up-core.sh` (up), `down-core.sh` (down)

Optional clean bringup (remove volumes, then start):
- PowerShell: `./scripts/up-core.ps1 -Clean -Build`
- Bash: `CLEAN=1 BUILD=1 ./scripts/up-core.sh`

Use this path when you need a clean-room validation where DB + broker state is recreated from scratch.

### RMS + optional LoRaWAN profile
- PowerShell: `up-lorawan.ps1` (up), `down-lorawan.ps1` (down)
- Bash: `up-lorawan.sh` (up), `down-lorawan.sh` (down)

### Generic compose wrappers
- Bash: `bringup.sh`, `bringdown.sh`, `status.sh`, `stats.sh`
- These support `COMPOSE_FILE=<file>` override (for example `docker-compose.integration.yml`).

Examples:

```powershell
cd C:\Project-Play\Unified-IoT-Portal-18-Jan\rms-go\go-kusumc
.\scripts\up-core.ps1
.\scripts\down-core.ps1
.\scripts\up-lorawan.ps1
.\scripts\down-lorawan.ps1
```

```bash
cd /path/to/rms-go/go-kusumc
./scripts/up-core.sh
./scripts/down-core.sh
./scripts/up-lorawan.sh
./scripts/down-lorawan.sh
```

## Integration and E2E runners
- `run-integration.ps1` / `run-integration.sh`: integration orchestration with optional compose management.
- `run-e2e-ordered.ps1` / `run-e2e-ordered.sh`: deterministic integration-tagged E2E sequence.
- `command-integration.ps1` / `command-integration.sh`: command-path focused integration setup checks.

Common options:
- `run-integration.*`: skip compose, keep up, compose file override, test filter.
- `run-e2e-ordered.*`: skip compose, keep up, compose file override.
- `command-integration.sh`: `--skip-compose`, `--skip-down`, `--project-root`.

## Smoke and verification scripts
- `smoke.ps1` / `smoke.sh`: broad API smoke flow.
- `smoke-device-open-broker.ps1` / `smoke-device-open-broker.sh`: device-open + broker resync checks.
- `smoke-mirror-simulator.ps1` / `smoke-mirror-simulator.sh`: telemetry mirror/simulator checks.
- `seed_fixtures.ps1` / `seed_fixtures.sh`: local fixture seeding.

## Secrets management scripts
- Folder: `scripts/secrets/`
- `publish-gsm-secrets.ps1`: publish local env values to Google Secret Manager.
- `render-env-from-gsm.ps1`: render a runtime env file from Google Secret Manager.
- `bootstrap-gcp-rms-go.ps1`: bootstrap GCP project, APIs, IAM bindings, and service account with command log.
- `check-certificate-status.ps1`: inspect local certificate files and expiry status without OpenSSL dependency.
- Usage details: `scripts/secrets/README.md`

These scripts are intended to keep development runnable after secret cleanup by separating:
- local-only secret files (`.env.local`, untracked), and
- deployment/runtime secrets sourced from GSM.

## Dead-letter replay operations

Environment knobs (set in compose defaults and overridable):
- `INGEST_DEADLETTER_REPLAY_ENABLED`
- `INGEST_DEADLETTER_REPLAY_INTERVAL_MS`
- `INGEST_DEADLETTER_REPLAY_BATCH`
- `INGEST_DEADLETTER_REQUEUE_MAX`
- `INGEST_DEADLETTER_REQUEUE_TTL_HOURS`

Diagnostics endpoints:
- `GET /api/diagnostics/ingest/deadletter`
- `POST /api/diagnostics/ingest/deadletter/replay`

Manual replay example:

```bash
curl -k -X POST "https://rms-iot.local:7443/api/diagnostics/ingest/deadletter/replay?limit=100" \
  -H "Authorization: Bearer <access-token>" \
  -H "x-api-key: <api-key>"
```

## Linux VM note
For Linux VM execution, ensure shell scripts are executable once:

```bash
chmod +x ./scripts/*.sh
```

Quick Linux startup examples:

```bash
./scripts/up-core.sh
./scripts/down-core.sh
./scripts/up-lorawan.sh
./scripts/down-lorawan.sh
```

## Seed baseline after clean bringup
On a fresh volume bringup, runtime seeding should create:
- Organization hierarchy: `Maharashtra -> MSEDCL -> PM_KUSUM`
- Admin users: `Him` and `Hadi` (admin role)

These seeds are relevant for smoke/integration flows and should be validated during environment bringup checks.
