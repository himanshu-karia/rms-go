# Firmware Integration Docs Index

This folder documents the active firmware integration contract for go-kusumc, with focus on minimal hardware change.

Firmware team handoff:
- If you are shipping/maintaining **legacy govt MQTT** (`<IMEI>/{heartbeat,pump,data,daq,ondemand}`), use `../firmware-integration-kusumc-legacy-only/`.
- This folder describes the current go-kusumc runtime contract, including optional compatibility subscriptions controlled by `MQTT_COMPAT_TOPICS_ENABLED`.

Cross-profile consistency:
- `../firmware-old-to-new-legacy/00-main-guide.md`

## Last refresh
- Date: 2026-02-18
- Scope: parity snapshot refresh, alias-compatibility convention hardening, REST contract updates, and dedicated device API request/response samples.

## Read in this order
1. `01-parity-audit-matrix.md` — old vs new endpoint/workflow parity snapshot.
2. `02-delta-report.md` — value-add features and compatibility impact.
3. `03-mqtt-topics-and-payloads.md` — MQTT topics, payload contracts, self vs forwarded telemetry.
4. `04-rest-api-contract.md` — REST contract map used by firmware/support tooling.
5. `05-lifecycle-flows.md` — provisioning → telemetry → alerts → commands lifecycle.
6. `06-sequence-checkin-command.md` — check-in and command response sequence details.
7. `07-firmware-pseudocode.md` — implementation templates for firmware teams.
8. `08-gap-closure-plan.md` — remaining parity items and execution order.
9. `09-device-api-samples.md` — concrete request/response examples for device-facing REST endpoints.
10. `10-handoff-readiness-report.md` — test-backed firmware handoff readiness and coverage matrix.
11. `11-firmware-onboarding.md` — firmware engineer quickstart (modes, endpoints, topics, TLS, logs).
12. `12-firmware-troubleshooting.md` — common failure modes, diagnostics checklist, and recovery patterns.
13. `13-firmware-dev-playbook.md` — firmware-focused validation checklist + contracts.
14. `14-firmware-test-vectors.md` — copy/paste MQTT payloads + expected responses.
15. `15-migration-legacy-only-to-compat-topics.md` — optional compat topics and migration steps.

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

## Canonical source artifacts
- RMS inventory parity verification: `../rms-go-inventory-parity-verification.md`
- Govt protocol verification: `../govt-protocol-verification-kusumc.md`
- Go payload contract reference: `../../go-kusumc/docs/payload-contract.md`
- MQTT bootstrap contract reference: `../../go-kusumc/docs/mqtt-bootstrap-contract.md`

## Naming convention used in this folder
- **Wire format is snake_case** in examples:
	- HTTP query params: `device_uuid`, `device_id`, `project_id`, etc.
	- HTTP JSON + MQTT JSON: snake_case keys (`packet_type`, `correlation_id`, ...)
- Route path params remain as implemented in the backend router (for example `:device_uuid` / `:topic_suffix` in Go route templates), but this is not a JSON/query key.

Notes:
- Some camelCase aliases may still be accepted by compatibility handlers, but firmware should always send snake_case.
- Server operators can enable `STRICT_SNAKE_WIRE=true` to reject camelCase query/JSON keys on `/api/*`.
