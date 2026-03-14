# Cutover plan (replace legacy refer-rms-deploy with KUSUMC stack)

## Objective
Replace the deployed legacy RMS stack (refer-rms-deploy) with:
- go-kusumc backend
- ui-kusumc frontend
- same protocol: government/legacy MQTT topics + payloads
- domain: `kusumc.hkbase.in`

## Cutover constraints
- Firmware cannot be changed (protocol is frozen).
- Topics/payloads must remain consistent.

## Steps

### Phase 0 — Preflight
- Ensure DNS for `kusumc.hkbase.in` points to new infra.
- Ensure hkbase.in wildcard cert is installed.
- Confirm EMQX is configured and reachable.

### Phase 1 — Shadow deploy
- Deploy go-kusumc + EMQX + DB in parallel without switching firmware.
- Validate UI smoke flows.
- Validate MQTT ingestion using test device.

### Phase 2 — Controlled device migration
Options:
- If the broker endpoint hostname remains the same and only backend changes, no device change is needed.
- If broker endpoint changes (host/port), devices must re-bootstrap/claim/download credentials.

### Phase 3 — Cutover
- Switch production DNS / routing.
- Monitor:
  - broker connections
  - telemetry ingest rate
  - command ack rate
  - error rates

### Phase 4 — Rollback plan
- Keep legacy stack available for rollback window.
- Rollback is DNS/routing + credential/ACL restore if necessary.

## Post-cutover
- Freeze the KUSUMC contract docs.
- Only patch critical bugs/security.
