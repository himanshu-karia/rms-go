# Firmware Integration Docs Index

This is the **legacy-only** firmware handoff doc set for KUSUMC.

If you are a firmware developer/firmware agent, start here:
- `for-firmware-agent/00-index.md`

Scope for firmware teams:
- Use only the frozen legacy topic model with PumpData on data lane (`<IMEI>/data`): `<IMEI>/{heartbeat,data,daq,ondemand,errors}`.
- Use legacy payload formats (as documented in the govt protocol references).
- Firmware should treat this folder as the single source of truth.

## Last refresh
- Date: 2026-02-18
- Scope: parity snapshot refresh, alias-compatibility convention hardening, REST contract updates, and dedicated device API request/response samples.

## Read in this order
Firmware-only (use these):
1. `for-firmware-agent/00-index.md`
2. `for-firmware-agent/01-contract-map.md`
3. `for-firmware-agent/02-onboarding-quickstart.md`
4. `for-firmware-agent/03-mqtt-topics-and-payloads.md`
5. `for-firmware-agent/04-rest-api-contract.md`
6. `for-firmware-agent/05-lifecycle-flows.md`
7. `for-firmware-agent/06-firmware-pseudocode.md`
8. `for-firmware-agent/07-firmware-test-vectors.md`
9. `for-firmware-agent/08-troubleshooting.md`

Platform verification / server migration notes (not required for firmware):
- `01-parity-audit-matrix.md`

Cross-profile consistency:
- `../firmware-old-to-new-legacy/00-main-guide.md`
- `02-delta-report.md`
- `08-gap-closure-plan.md`
- `10-handoff-readiness-report.md`
- `13-firmware-dev-playbook.md`

Source/reference docs (optional):
- `03-mqtt-topics-and-payloads.md`
- `04-rest-api-contract.md`
- `05-lifecycle-flows.md`
- `06-sequence-checkin-command.md`
- `07-firmware-pseudocode.md`
- `09-device-api-samples.md`
- `11-firmware-onboarding.md`
- `12-firmware-troubleshooting.md`
- `14-firmware-test-vectors.md`

## Diagrams
Mermaid sources and exported SVGs live in `diagrams/`.

Quick map:
- Deployment modes (HTTP+MQTT vs HTTPS+MQTTS): `00-deployment-modes.flowchart.{mmd,svg}`
- Bootstrap: `01-bootstrap.sequence.{mmd,svg}`
- Credentials + connect loop: `02-credentials-and-connect.flowchart.{mmd,svg}`
- Telemetry self publish path: `03-telemetry-self.flowchart.{mmd,svg}`
- Telemetry forwarded path: `04-telemetry-forwarded.flowchart.{mmd,svg}`
- Commands roundtrip: `05-commands-roundtrip.sequence.{mmd,svg}`
- HTTP command recovery: `06-command-recovery-http-fallback.sequence.{mmd,svg}`
- VFD/RS485 metadata fetch: `07-vfd-rs485-fetch.sequence.{mmd,svg}`
- Credential rotation reconnect: `08-credential-rotation.sequence.{mmd,svg}`
- Govt broker optional path: `09-govt-broker.optional.sequence.{mmd,svg}`
- Node provisioning + forwarding: `10-node-provisioning.flowchart.{mmd,svg}`
- SendImmediate decision: `11-send-immediate-decision.flowchart.{mmd,svg}`
- Device configuration apply: `12-device-configuration-apply.sequence.{mmd,svg}`

## Platform-only reference artifacts (not required for firmware)
- RMS inventory parity verification: `../rms-go-inventory-parity-verification.md`
- Govt protocol verification: `../govt-protocol-verification-kusumc.md`
- Go payload contract reference: `../../go-kusumc/docs/payload-contract.md`
- MQTT bootstrap contract reference: `../../go-kusumc/docs/mqtt-bootstrap-contract.md`

## Naming convention used in this folder
- Device-facing HTTP examples use **snake_case** on the wire:
	- HTTP query params: `device_uuid`, `device_id`, `project_id`, etc.
- Legacy MQTT payload examples follow the govt protocol (may use uppercase keys like `IMEI`, `TIMESTAMP`).
- Route path params remain as implemented in the backend router (for example `:device_uuid` / `:topic_suffix` in Go route templates), but this is not a JSON/query key.

Notes:
- Legacy camelCase aliases may still be accepted by some handlers during migration, but firmware should always send snake_case.
- Server operators can enable `STRICT_SNAKE_WIRE=true` to reject camelCase query/JSON keys on `/api/*`.
