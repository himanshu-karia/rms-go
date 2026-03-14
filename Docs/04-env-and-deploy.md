# Environment, URLs, and deployment plan (KUSUMC)

## Domains
- KUSUMC UI: `ui-kusumc.hkbase.in`
- KUSUMC API: `api-kusumc.hkbase.in`
- KUSUMC MQTT/MQTTS: `mqtt-kusumc.hkbase.in`
- Unified platform (separate product): `iot.hkbase.in`

## Certificates
- Use hkbase.in wildcard certificate for Nginx TLS.

## Required env knobs (KUSUMC)

### Backend (go-kusumc)
These must be configurable by env so future domain changes are rebuild-only:

Broker connectivity (internal docker network):
- `MQTT_HOST` (e.g. `emqx`)
- `MQTT_PORT` (e.g. `1883`)

Broker endpoints advertised to devices (public):
- `MQTT_PUBLIC_HOST=mqtt-kusumc.hkbase.in`
- `MQTT_PUBLIC_TLS_PORT=8883`
- optional: `MQTT_PUBLIC_TCP_PORT=1883` or `1884` (only if you allow plaintext)
- optional: `MQTT_PUBLIC_WS_URL=wss://mqtt-kusumc.hkbase.in/mqtt` (for web tools)

HTTP base (public via Nginx):
- ensure Nginx routes `/api/*` to backend container.

### UI (ui-kusumc)
- `VITE_API_BASE_URL` should be omitted (prefer same-origin `/api/v1` or `/api`) or set to `https://api-kusumc.hkbase.in/api/v1`.
- `VITE_MQTT_WS_URL=wss://mqtt-kusumc.hkbase.in/mqtt` when simulator features are used.

## Compose-level changes (expected)
Once copied:
- create a dedicated compose file for KUSUMC:
  - backend service name `go-kusumc`
  - nginx config for `kusumc.hkbase.in`
  - EMQX ports:
    - `8883` mqtss
    - optional `1883/1884` plaintext

## Build and deploy flow (high-level)
1) Build container images for go-kusumc and ui-kusumc.
2) Provision DNS `kusumc.hkbase.in` → Nginx.
2) Provision DNS:
  - `ui-kusumc.hkbase.in`
  - `api-kusumc.hkbase.in`
  - `mqtt-kusumc.hkbase.in`
3) Verify TLS.
4) Ensure EMQX is reachable at `mqtts://mqtt-kusumc.hkbase.in:8883`.
5) Run smoke tests (see `05-test-plan-and-acceptance.md`).

## Cutover note
See `06-cutover-plan.md` for replacing the legacy refer-rms-deploy runtime.
