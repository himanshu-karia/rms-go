# MQTT bootstrap contract (Unified vs Govt)

This repo exposes two distinct “broker configs” in `GET /api/bootstrap?imei=...`:

## 1) `primary_broker` (Unified / internal)
- Purpose: device → Unified ingestion (our EMQX)
- Credentials source (DB): only auth material is persisted per device in `credential_history.bundle`:
  - `username`, `password`, `client_id`
- Endpoint source (runtime): derived from env (never stored in DB)
  - Default (single endpoint): `MQTT_PUBLIC_PROTOCOL`, `MQTT_PUBLIC_HOST`, `MQTT_PUBLIC_PORT`
  - Optional (multiple endpoints): `MQTT_PUBLIC_URLS` (comma-separated), e.g.
    - `MQTT_PUBLIC_URLS=mqtts://iot.example.com:8883,mqtt://iot.example.com:1884`
- Topics (project-scoped): derived from the project scope (`project_id`) + device (`imei`)
  - Default publish: `channels/{project_id}/messages/{imei}`
  - Default subscribe: `channels/{project_id}/commands/{imei}`
  - Optional override: `protocols.kind=primary` may define topic templates using `{project_id}` / `{imei}`.

## 2) `govt_broker` (Vendor/Govt server)
- Purpose: device ↔ external vendor/govt broker (not EMQX)
- Credentials source (DB): stored per device in govt creds storage keyed by `govt_protocol_id`.
- Endpoint + topics source: derived from the govt protocol profile (`protocols.kind=govt`) for the project.
  - Protocol profile may represent `mqtts://...` or other schemes; topic templates may use `{project_id}` / `{imei}`.
- Important: govt broker settings never fall back to the Unified broker routing.

## Notes
- The `protocols` table models broker profiles (kind `primary` / `govt`) per project.
- Device payload parsing is governed by Project DNA + device attributes (separate from broker routing).

### Precedence rules (primary broker endpoints)
1) If `MQTT_PUBLIC_URLS` is set and contains at least one valid URL, it becomes the advertised `endpoints[]` list.
2) Otherwise, a single endpoint is built from `MQTT_PUBLIC_PROTOCOL|HOST|PORT` (with secure defaults).
