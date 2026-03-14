# Production-Only Seed Set (KUSUMC)

This document defines the **production-only** seed set for `rms-go/go-kusumc`.

Purpose:
- Keep only data required for real KUSUMC operation.
- Exclude demo/sample/multi-project convenience seeds.
- Provide deterministic seed order for fresh production deployment.

## 1) Scope

Use this alongside:
- `MANDATORY-SEED-CONTRACT.md` (normative minimum contract)
- `MANDATORY-SEED-SAMPLE-KUSUMC.md` (all authored sample seeds)

This file is the filtered production subset.

## 2) Include in production seeds

## 2.1 Identity + access rails

Seed/ensure:
- At least one admin operator user.
- Role/capability catalogs.
- User-role/capability bindings for operator accounts.
- API key/JWT-compatible auth rails (as required by your ops model).

## 2.2 Hierarchy and reference master data

Seed/ensure only required references used by enrollment and validation:
- states
- authorities
- server vendor / implementation vendor / OEM organizations
- protocol version references (if your enrollment flow enforces them)

## 2.3 KUSUMC project baseline

Seed/ensure:
- Project: `pm-kusum-solar-pump-msedcl`
- Primary protocol (`proto-pm-primary`) and Govt protocol (`proto-pm-govt`)
- Topic templates (legacy KUSUMC profile):
  - publish: `<IMEI>/heartbeat`, `<IMEI>/data`, `<IMEI>/daq`, `<IMEI>/ondemand`
  - subscribe: `<IMEI>/ondemand`

## 2.4 Command baseline

Seed/ensure:
- Core command catalog entries (global):
  - `reboot`
  - `rebootstrap`
  - `set_ping_interval_sec`
  - `send_immediate`
  - `apply_device_configuration`
- Project-scoped command catalog only if required by your command UX.

## 2.5 Device baseline

Seed/ensure for real devices (not demo placeholders):
- Device rows mapped to project.
- Credential bundles/history.
- Provisioning status/job rows where expected by backend/UI.

## 2.6 VFD baseline (if your rollout uses VFD flows)

Seed/ensure:
- VFD manufacturer rows (production OEMs).
- VFD models and command dictionaries used by deployed devices.
- Protocol-VFD assignments.

## 2.7 Project DNA baseline (recommended)

Seed/ensure for each production project:
- Payload schema rows for accepted packets.
- Sensor definitions used for transformation.
- Threshold defaults (project scope), plus device overrides only where needed.

Project DNA is strongly recommended for quality/validation, even when ingestion can technically continue without it.

## 3) Exclude from production seeds

Do **not** include these in production seed packs:
- Demo convenience baseline:
  - `Demo Org`
  - `demo-project-01`
  - demo admin/API key rows with placeholder hashes
- Generic multi-project samples:
  - `project_01_flood` ... `project_18_waste`
  - auto-created sample devices under those projects
- Test/sample RMS rows not used in your target deployment:
  - `rms-pump-01`
  - `RMS-DEVICE-001`
  - sample govt endpoints such as `gw.gov.example.com`
- Seed placeholders meant only for local bring-up:
  - `Seed VFD OEM`
  - `Seed-Model`
  - any metadata flags like `{ "seed": true }` when replacing with real values

## 4) Production seed order (recommended)

Apply in this order on a fresh DB:
1. Schema migrations.
2. Capability/role catalogs.
3. Users + memberships + auth keys/tokens model.
4. Hierarchy and vendor master data.
5. Project row(s).
6. Protocol row(s) and transport metadata.
7. Command catalog rows.
8. Device rows + credentials/provisioning rows.
9. VFD and protocol-VFD assignment rows (if used).
10. Project DNA rows (payload schemas, sensors, thresholds).

## 5) Validation gate after seeding

Before onboarding firmware traffic, verify:
- auth/session works for admin/operator,
- project and protocol resolve correctly,
- device credentials are retrievable and active,
- one heartbeat packet ingests and appears in history,
- command issue/ack path works,
- thresholds and telemetry validation paths behave as expected.

## 6) Operational guardrail

Runtime seeding in `rms-go` is now PM-KUSUM-only by default (no seed profile flags required).

Current posture:
- runtime seeder keeps admin + core commands + PM-KUSUM baseline,
- runtime seeder does not create multi-project samples or legacy RMS demo seeds,
- SQL bootstrap no longer inserts demo/sample projects/devices.

For strict production control, continue using a curated migration/seed execution pipeline and treat this document as the required baseline contract.
