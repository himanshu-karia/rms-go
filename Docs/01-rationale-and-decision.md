# Rationale & Decision Record (KUSUMC)

## Decision
Create a dedicated RMS product stack on `kusumc.hkbase.in`:
- Backend: go-kusumc (derived from `unified-go`)
- UI: ui-kusumc (derived from `old-ui-copy`)

The RMS stack will:
- Speak the **frozen government MQTT protocol** (legacy topic structure + payload formats).
- Remain internally consistent with the RMS firmware.
- Replace the legacy refer-rms-deploy runtime deployment.

The platform stack will remain separate:
- `unified-go` + `new-frontend` continues as the multi-project “Unified IoT” platform.

## Why this is the right split
- The legacy/government protocol is intentionally frozen; supporting it inside the multi-project channels architecture creates ongoing complexity.
- A dedicated backend can simplify assumptions:
  - topic model is `<IMEI>/<suffix>`
  - packet typing is suffix-driven
  - payload keys remain government/legacy shape
- It reduces the risk that platform changes (routing, schema strictness, multi-project ACLs) break RMS firmware.

## What is NOT the goal
- Not trying to keep RMS firmware compatible with `unified-go` channels topics.
- Not trying to maintain dual-protocol support in the platform backend.

## Compatibility expectation
- KUSUMC firmware continues to publish/subscribe on legacy government topics.
- go-kusumc ingests and correlates commands/responses using those legacy topics.
- UI operates against go-kusumc REST APIs that mirror the old UI expectations.

## Deployment naming
- Domain: `ui-kusumc.hkbase.in` (KUSUMC UI)
- Domain: `api-kusumc.hkbase.in` (KUSUMC API)
- Domain: `mqtt-kusumc.hkbase.in` (KUSUMC MQTT/MQTTS)
- Domain: `iot.hkbase.in` (Unified IoT platform)

## Change management rules
- Security fixes must be applied to both product lines.
- Protocol contract changes require explicit approval (should be rare for KUSUMC).
- Shared improvements should be cherry-picked deliberately, not copied ad-hoc.
