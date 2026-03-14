# Mandatory Seed Sample (KUSUMC)

This file lists the **actual seed data currently present in code** for `rms-go/go-kusumc`.

Source of truth:
- SQL bootstrap seeds: `go-kusumc/schemas/v1_init.sql`
- Runtime startup seeds: `go-kusumc/internal/core/services/seeder_service.go`

This document reflects the PM-KUSUM-only cleanup where demo, multi-project, and legacy RMS sample seeds were removed.

## 1) Seed execution paths

1. **DB bootstrap SQL** (`v1_init.sql`)
   - Runs during schema initialization.
  - Defines schema and core capability catalog only (no demo/sample project or device rows).

2. **App startup seeder** (`SeederService.Seed()`)
   - Runs on server boot (`cmd/server/main.go`).
  - Ensures admin/capabilities/core commands + PM-KUSUM baseline rows.

## 2) Seed precedence / conflict behavior

- Most inserts use `ON CONFLICT DO NOTHING` (first writer wins, later writer skipped).
- Result: deterministic PM-KUSUM-oriented baseline without demo/multi-project pre-seeding.

## 3) SQL bootstrap seeds (`v1_init.sql`)

SQL bootstrap now keeps **schema + capability catalog only**.

Included:
- capability keys (telemetry/alerts/devices/simulator/diagnostics/catalog/hierarchy/users/audit/admin)

Removed from SQL:
- demo convenience rows (`Demo Org`, `demo-project-01`, placeholder demo admin/api key)
- legacy RMS sample rows (`rms-pump-01`, `RMS-DEVICE-001`, sample govt endpoint)

## 4) Runtime startup seeds (`SeederService`)

### 4.1 Admin + capability rails

- Ensures user `Him` exists with password `0554` and role `admin`.
- Grants all capabilities to `Him`.

### 4.2 Core command catalog (global)

Seeded as `scope=core`, `project_id=NULL`, transport `mqtt`:
- `reboot`
- `rebootstrap`
- `set_ping_interval_sec` (required `interval_sec`, min 5, max 86400)
- `send_immediate`
- `apply_device_configuration` (required `config_id`, `config`)

### 4.3 PM-KUSUM seed bundle

- Project:
  - `id`: `pm-kusum-solar-pump-msedcl`
  - `name`: `PM-KUSUM Solar Pump RMS`
- Protocol IDs:
  - `proto-pm-primary` (`mqtt.local:1883`)
  - `proto-pm-govt` (`govt-broker.example.com:8883`)
- Topic templates:
  - publish: `<IMEI>/heartbeat`, `<IMEI>/data`, `<IMEI>/daq`, `<IMEI>/ondemand`
  - subscribe: `<IMEI>/ondemand`
- VFD seeds:
  - manufacturer id: `vfd-manu-seed`, name `Seed VFD OEM`
  - model id: `vfd-model-seed`, model `Seed-Model`, version `v1`
  - assignment id: `assign-proto-vfd-seed` linking primary protocol and VFD model

Removed from runtime seeder:
- multi-project sample project creation + sample devices
- sample Project DNA rows for flood/weather demo projects
- legacy RMS demo baseline (`rms-pump-01`, `RMS-DEVICE-001`)

## 5) What to treat as mandatory for go-live

Use `MANDATORY-SEED-CONTRACT.md` as the normative requirement list.
Use this file as the **exact sample seed dataset** currently implemented.

For curated production guidance, see `PRODUCTION-ONLY-SEED-SET-KUSUMC.md`.
