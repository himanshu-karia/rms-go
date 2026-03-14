# Nginx + EMQX Hardening & Flow (refer-rms alignment)

## Goals
- Add an edge proxy (Nginx) to terminate TLS and route HTTP/WS.
- Harden EMQX: no anonymous auth, bcrypt passwords, TLS listeners, dashboard scoped to internal.
- Keep Go backend using EMQX API + MQTT over TLS; keep TimescaleDB/Redis unchanged.
- Separate infra bring-up from tests so logs can be inspected first.

## Target flow
```
Client → Nginx (443/80)
  - /api/*           → server:8081 (HTTP)
  - /mqtt/ws (opt.)  → emqx:8083 (WS) or 8084 (WSS)
  - static/frontend  → (optional later) web app
Internal: server ↔ EMQX (mqtts:8883 + HTTP API 18083 internal), TimescaleDB, Redis.
Dashboard: EMQX dashboard bound to localhost/private; proxied only if explicitly allowed.
```

## Changes to apply
- **Compose (integration)**
  - Add Nginx service with mounted certs + proxy config; expose 80/443 only.
  - Harden EMQX env: `EMQX_ALLOW_ANONYMOUS=false`, bcrypt authn, TLS listeners (8883), dashboard bind internal, mount certs.
  - Share `certs/` volume between Nginx and EMQX (self-signed for dev; real certs for prod).
  - Keep server, redis, timescaledb as-is; pass CA path so MQTT TLS works (`MQTT_URL=mqtts://emqx:8883`).
  - Keep test-runner optional (do not start by default when inspecting infra).

- **Configs**
  - Add `infra/nginx/emqx.conf` (proxy /mqtt/ws to EMQX WS/WSS) and `infra/nginx/api.conf` (proxy /api to server).
  - Add minimal EMQX TLS/ACL config if needed (otherwise env-based built-in auth is enough).

- **Security posture**
  - Disable anonymous MQTT; enforce bcrypt.
  - Bind dashboard to localhost/private subnet; only proxy behind Nginx with auth if required.
  - Use the same certs for Nginx TLS and EMQX TLS (for mTLS later if desired).

## Bring-up process (recommended)
1) Infra only: `docker compose -f unified-go/docker-compose.integration.yml up --build server emqx emqx-bootstrapper redis timescaledb nginx`
2) Tail logs: `docker compose -f unified-go/docker-compose.integration.yml logs -f emqx emqx-bootstrapper server nginx`
3) When clean, run tests: add `test-runner` or `go test -tags=integration ./tests/e2e`.

## Open items
- Decide certificate source for dev (self-signed) vs prod (real certs); wire CA trust for server.
- Optional: proxy EMQX dashboard behind Nginx with basic auth/IP allowlist.
- Optional: expose frontend via Nginx when ready.
