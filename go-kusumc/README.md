# go-kusumc (RMS backend)

Go backend for the **KUSUMC RMS product line** (within `rms-go/`).

## MQTT topic model (selected)
Current RMS telemetry topics:
- Uplink telemetry: `<imei>/{heartbeat,data,daq}`
- Commands + responses: `<imei>/ondemand` (same topic)

go-kusumc subscribes to data-only telemetry topics by default, with optional channels/devices compat overlays when explicitly enabled.

## Local dev
```pwsh
cd rms-go\go-kusumc
docker compose up --build
```

Local custom-domain setup helper (hosts + cert + CA import):
```pwsh
cd rms-go\go-kusumc
.\scripts\setup-custom-local-domain.ps1
```

Clean-room bringup (drop old volumes, rebuild, then start):
```pwsh
cd rms-go\go-kusumc
docker compose down --volumes --remove-orphans
docker compose up -d --build --remove-orphans
# helper
.\scripts\up-core.ps1 -Clean -Build
```

Run the server without Docker:
```pwsh
cd rms-go\go-kusumc
go run .\cmd\server
```

## Tests
Unit:
```pwsh
cd rms-go\go-kusumc
go test ./...
```

Integration (recommended profile):
```pwsh
cd rms-go\go-kusumc
./scripts/run-integration.ps1 -ProjectRoot . -ComposeFile docker-compose.integration.yml -TestName TestDeviceLifecycle
```

Integration/E2E against local docker-backed stack (PowerShell env overrides):
```pwsh
cd rms-go\go-kusumc
$env:BASE_URL='https://rms-iot.local:7443'
$env:BOOTSTRAP_URL='https://rms-iot.local:7443/api/bootstrap'
$env:PROJECT_ID='pm-kusum-solar-pump-msedcl'
$env:TIMESCALE_URI='postgres://postgres:password@localhost:5433/telemetry?sslmode=disable'
$env:MQTT_BROKER='mqtts://rms-iot.local:18883'
go test -tags=integration ./tests/e2e -run 'TestDeviceCommandLifecycle|TestKusumFullCycle|TestRMSMegaFlow|TestSolarRMSFullCycle|TestStory_FullCycle|TestUIAndDeviceOpenFullCycle' -count=1 -v
```

Important: before `docker compose up`, clear host-local `TIMESCALE_URI` override so containers do not inherit a host DB endpoint:
```pwsh
Remove-Item Env:TIMESCALE_URI -ErrorAction SilentlyContinue
```

Single source of truth for test env overrides: `docs/README.md` → **Test env overrides (PowerShell)**.

## Optional LoRaWAN stack (dormant by default)
ChirpStack and its helper containers are kept in this repo for optional LoRaWAN use, but are **disabled by default** in `docker-compose.yml` using the `lorawan` profile.

Default RMS start (no ChirpStack):
```pwsh
cd rms-go\go-kusumc
docker compose up -d
# helper
.\scripts\up-core.ps1
```

Optional clean core start:
```pwsh
cd rms-go\go-kusumc
.\scripts\up-core.ps1 -Clean -Build
```

Enable LoRaWAN/ChirpStack explicitly:
```pwsh
cd rms-go\go-kusumc
docker compose --profile lorawan up -d
# helper
.\scripts\up-lorawan.ps1
```

Stop including LoRaWAN services:
```pwsh
cd rms-go\go-kusumc
docker compose --profile lorawan down
```

E2E expectations:
- `docker-compose.integration.yml` is the E2E stack and does not require ChirpStack.
- RMS API/MQTT E2E flows remain independent of LoRaWAN services.

Seed baseline relevant for validation:
- Organization hierarchy: `Maharashtra -> MSEDCL -> PM_KUSUM`
- Admin users: `Him`, `Hadi`

## Docs
Canonical RMS docs live in `rms-go/Docs/`.
Canonical index: `rms-go/Docs/HANDOVER-CANONICAL-INDEX.md`.
Firmware integration docs: `rms-go/Docs/firmware-integration-kusumc/for-firmware-agent/`.
Backend contracts/runbooks index: `rms-go/go-kusumc/docs/README.md`.
