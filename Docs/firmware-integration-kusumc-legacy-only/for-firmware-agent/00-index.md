# Firmware Agent Docs (Legacy-only)

This folder is the **only** set of docs a firmware developer/agent needs for KUSUMC when running in **legacy-only mode**.

## Scope (frozen)
- MQTT topics: `<IMEI>/{heartbeat,data,daq,ondemand,errors}`
- PumpData uplink topic: `<imei>/data` only.
- Full packet contract for all telemetry families is documented in `03-mqtt-topics-and-payloads.md` (envelope + packet-specific fields).
- Ondemand downlink command shape: govt legacy top-level fields (`msgid`, `timestamp`, `type`, `cmd`, params...)
- Ondemand uplink response shape: govt legacy (`timestamp`, `status`, optional `code`, and **echo `msgid`** recommended).

Endpoint policy (must follow):
- Do not hardcode REST base URL or MQTT broker URL in firmware binaries.
- REST base URL must be runtime-configurable (env/config/NVM).
- MQTT broker endpoint must be derived from backend bootstrap/credential endpoints (`primary_broker.endpoints[]` or `credential.endpoints[]`).

For compact/dual-topic migration guidance (`channels/...` and `devices/...` topic families), use:
- `../../firmware-old-to-new-legacy/00-main-guide.md`
- `../../firmware-old-to-new-legacy/03-packet-examples.md`

## Read in this order
1. `01-contract-map.md` — end-to-end 1:1 mapping (Firmware ↔ Broker ↔ Backend ↔ DB ↔ UI).
2. `02-onboarding-quickstart.md` — bootstrap → connect → publish → commands → recovery.
3. `03-mqtt-topics-and-payloads.md` — complete topics + full payload schemas + parameter dictionaries.
4. `04-rest-api-contract.md` — the small set of device-facing HTTP endpoints firmware may call.
5. `05-lifecycle-flows.md` — provisioning/telemetry/commands flows + diagrams.
6. `06-firmware-pseudocode.md` — implementation templates.
7. `07-firmware-test-vectors.md` — full, field-rich copy/paste MQTT vectors for all packet families.
8. `08-troubleshooting.md` — common failure modes + diagnostics.
9. `09-device-events-and-storage.md` — error taxonomy + store-and-forward + SD/flash fallback.
10. `10-error-codes.md` — canonical `error_code` enum list + severity guidance.

Includes:
- RS485/VFD read stub (minimal)
- MQTT auth-reject recovery loop (retry once, then refresh creds via HTTP)

## Diagrams
Diagrams live one level up at `../diagrams/` and are referenced from these docs.
