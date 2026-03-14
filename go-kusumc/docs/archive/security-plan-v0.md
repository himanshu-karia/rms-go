# Security Plan v0

## Scope and goals
- Enforce secure-only ingress for external traffic: HTTPS, MQTTS, WSS.
- Keep device-facing ingest/bootstrap APIs open (no login/CORS), while UI/admin APIs are CORS-guarded and authenticated.
- Terminate TLS at Nginx for a single external entrypoint; remove host exposure of raw broker/API ports.

## Port and exposure policy
- Expose externally: 443 (HTTPS), 8883 (MQTTS/WSS via Nginx), 80 (optional redirect only).
- Close/remove from host: 1884 (MQTT TCP), 8084 (WS), 18084 (EMQX dashboard), 8081 (direct HTTP API).
- Internal-only: EMQX 1883, EMQX dashboard, ingestion-go 8081 (reachable only from Nginx in the docker network).

## Dev vs prod: which compose exposes what
- **Prod-like (secure-only)**: `docker compose -f docker-compose.yml up`
	- Host-exposed ports: `80` (redirect), `443` (HTTPS), `8883` (MQTTS).
	- Not host-exposed: Go `8081`, EMQX `1883`, EMQX dashboard `18083`.
- **Integration / verification (insecure allowed)**: `docker compose -f docker-compose.integration.yml up`
	- Host-exposed ports include: Go `8081` and plain MQTT `1884 -> 1883`.
	- Use this only for local verification and CI where firmware/tools still need non-TLS endpoints.

## Provisioning: publish both MQTT URLs when needed
By default, the bootstrap/device-open credential payloads advertise a single MQTT endpoint derived from:
- `MQTT_PUBLIC_PROTOCOL`, `MQTT_PUBLIC_HOST`, `MQTT_PUBLIC_PORT` (defaulting to `mqtts://<host>:8883`).

For phased rollouts where you want to share both a secure and insecure broker URL (verify first, then enforce TLS), set:
- `MQTT_PUBLIC_URLS` = comma-separated list of broker URLs.

Example:
- `MQTT_PUBLIC_URLS=mqtts://iot.example.com:8883,mqtt://iot.example.com:1884`

Notes:
- When `MQTT_PUBLIC_URLS` is set and parses cleanly, it **overrides** the advertised `endpoints[]` list.
- Keep production ingress secure-only by simply not publishing the insecure ports and by advertising only `mqtts://...`.

## TLS termination and routing
- Nginx terminates TLS for HTTPS (443) and MQTTS/WSS (8883) using `infra/nginx/certs`.
- Stream block: `8883 ssl` -> proxy to `emqx:1883` (MQTT TCP). Clients connect only to 8883.
- HTTP block: `443 ssl` -> `/api/` proxied to `ingestion-go:8081`; `/mqtt` proxied to `emqx:8083` for WSS.
- EMQX retains TLS config internally but is not published to host; Nginx is the external terminator.

## CORS and auth posture
- UI/admin APIs: keep under CORS + auth. Set explicit allowlist via env (e.g., `FRONTEND_ORIGINS=https://app.example.com,https://admin.example.com`). Apply CORS middleware to UI route group only.
- Device/ingest APIs: open (no browser CORS gate, no login) but still auditable and may use API keys where relevant.
- MQTT topics: open to provisioned device creds over MQTTS/WSS; no CORS in MQTT.

## Device-facing open HTTP APIs (no login/CORS required)
- `POST /api/ingest` (HTTP ingest path for devices/legacy sensors; audit/API-key as configured).
- `GET /api/bootstrap?imei=...` (device bootstrap/config fetch by IMEI).
- `POST /api/northbound/chirpstack` (LoRa integration ingress).
- Any other device/webhook ingress routes explicitly grouped outside the UI CORS middleware remain open.

## UI/admin APIs (CORS + auth)
- `/api/auth/*`, `/api/projects/*`, `/api/protocols/*`, `/api/devices/*` (govt creds, bulk import), `/api/reports/*`, `/api/analytics/*`, `/api/erp/*`, `/api/verticals/*`, `/api/rules/*`, `/api/commands/*`, `/api/telemetry/export`, admin endpoints, etc.

## API access matrix (v0 posture)

Legend: Open = no JWT, no CORS; API key optional (recommended) for audit/scope. UI = CORS + auth. Device reads are scoped to its IMEI/project.

| API | Device read | Device write | UI write | Notes |
| --- | --- | --- | --- | --- |
| `/api/ingest` | N/A (ingest) | Yes (telemetry post) | No | Open; API key optional for audit. |
| `/api/bootstrap` | Yes | No | No | Open; IMEI-scoped config fetch. |
| `/api/northbound/chirpstack` | N/A | Yes (ingress) | No | Open webhook. |
| Govt creds `GET /api/devices/:id/govt-creds` | Yes (device fetch) | No | Yes | Allow device fetch (scoped to itself) with API key optional; UI mutations guarded. |
| Govt creds `POST /api/devices/:id/govt-creds`, bulk | No | No | Yes | UI-only. |
| Commands `GET /api/commands/catalog` | Yes (device fetch) | No | Yes | Allow device read with API key optional; UI can manage catalog. |
| Commands `GET /api/commands` | Yes (device fetch) | No | Yes | Device can poll its own commands (if enabled); UI guarded. |
| Commands `POST /api/commands/send` | No | No | Yes | UI-only to enqueue commands; devices receive via MQTT. |
| Installations `GET /api/installations` | Yes (device fetch) | No | Yes | Device fetch allowed (scoped); writes UI-only. |
| Beneficiaries `GET /api/beneficiaries` | Yes (device fetch) | No | Yes | Device fetch allowed (scoped); writes UI-only. |
| VFD metadata `GET /api/projects/:projectId/vfd/*` | Yes (device fetch) | No | Yes | Device fetch allowed (scoped); writes/imports UI-only. |
| Config profiles `GET /api/config/profiles` | Yes (device fetch) | No | Yes | Device fetch allowed (scoped); writes UI-only. |
| Rules (simple thresholds) `GET /api/rules` | Yes (device fetch) | No | Yes | Device can pull simple rules; writes UI-only. |
| Rules (create/delete) `POST/DELETE /api/rules` | No | No | Yes | UI-only. |
| Project DNA `GET /api/dna/:projectId` | Yes (device fetch) | No | Yes | Device fetch allowed (scoped); upserts UI-only. |
| Analytics/history/export | No | No | Yes | UI-only. |
| Admin/ERP/OTA/reporting | No | No | Yes | UI-only. |

Device fetch posture for the above read-open rows:
- Scope to IMEI/project; never return cross-project data.
- API key optional but recommended for audit/rate-limit; no JWT/CORS for device reads.
- Writes remain UI-only with CORS+auth.

## Action checklist to apply 1+2+3
1) docker-compose: remove host port publishes for emqx 1884/8084/18084 and ingestion-go 8081; keep only nginx 80/443/8883.
2) Nginx: keep TLS stream 8883 -> emqx:1883; keep HTTPS 443 with `/api/` to ingestion-go and `/mqtt` to emqx WSS; no other locations exposed.
3) Backend CORS: replace `AllowOrigins: "*"` with an env-driven allowlist; apply CORS middleware only to UI route group. Keep device ingress routes outside that CORS middleware (still behind API-key/audit as needed).
4) Credentials: rotate MQTT service creds from defaults before production; restrict EMQX dashboard to internal-only (no host publish or behind VPN).
5) Certificates: replace dev certs in `infra/nginx/certs` with real CA-issued certs for production hostnames; reload Nginx after update.

## Outcome
- External clients/devices see only HTTPS (443) and MQTTS/WSS (8883) with valid TLS.
- UI/API calls are CORS-bound to approved origins and require auth; device ingest/bootstrap remain open (non-JWT) but secured via API key/audit and MQTT credentials.
