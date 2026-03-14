# Day-1 docker-compose blueprint (documentation-only)

Goal: a minimal, production-shaped Docker Compose layout for **kusumc.hkbase.in** that brings up:
- HTTPS UI + REST API (via Nginx)
- MQTT broker with **MQTTS by default** (EMQX)
- Timescale/Postgres + Redis

Non-goals:
- This doc does **not** create any files in the repo (no Nginx configs, no compose files, no certs). It only specifies what to create and where.

## Day-1 service map

### Public ingress (open firewall ports)
- `443/tcp` → Nginx (HTTPS) 
- `8883/tcp` → EMQX (MQTTS)

Optional (only if you need them):
- `80/tcp` → Nginx (HTTP→HTTPS redirect + ACME)
- `443/tcp` also carries `wss://kusumc.hkbase.in/mqtt` (MQTT over WebSocket via Nginx)

### Internal-only
- `go-kusumc:8081` (HTTP)
- `timescaledb:5432`
- `redis:6379`
- `emqx:1883` (MQTT plaintext inside the Docker network; devices still use `8883`)

## Suggested host folder layout (paths and mounts)

Create a dedicated deployment folder on the server (example):

- `./deploy/kusumc/compose/` 
  - `docker-compose.yml` (to be created by you)
  - `.env` (to be created by you; never commit secrets)
- `./deploy/kusumc/nginx/`
  - `conf.d/kusumc.conf` (Nginx vhost)
  - `certs/`
    - `fullchain.pem`
    - `privkey.pem`
- `./deploy/kusumc/emqx/`
  - `certs/`
    - `fullchain.pem`
    - `privkey.pem`
  - (optional) `acl.conf`, `emqx.conf` if you prefer file-based config
- `./deploy/kusumc/data/`
  - `postgres/` (Timescale volume)
  - `redis/`
  - `emqx/`

If you already have an hkbase.in wildcard cert, reuse it in both Nginx and EMQX (same files, different mount points).

## TLS guidance (default: HTTPS + MQTTS)

### HTTPS (Nginx)
- Terminate TLS at Nginx for `https://kusumc.hkbase.in`.
- Mount the certificate into the container:
  - host: `./deploy/kusumc/nginx/certs/fullchain.pem`
  - host: `./deploy/kusumc/nginx/certs/privkey.pem`
  - container: `/etc/nginx/certs/fullchain.pem`
  - container: `/etc/nginx/certs/privkey.pem`

Nginx routes:
- `/` → `ui-kusumc` (static frontend)
- `/api/` → `go-kusumc` (REST)
- optional `/mqtt` (WebSocket upgrade) → EMQX WS listener (see below)

### MQTTS (EMQX)
- Devices should connect to: `mqtts://kusumc.hkbase.in:8883`
- EMQX should own the MQTTS listener (TLS happens at EMQX, not Nginx).
- Mount the same cert into EMQX:
  - host: `./deploy/kusumc/emqx/certs/fullchain.pem`
  - host: `./deploy/kusumc/emqx/certs/privkey.pem`
  - container: `/opt/emqx/etc/certs/fullchain.pem`
  - container: `/opt/emqx/etc/certs/privkey.pem`

EMQX must be configured to:
- enable `ssl:8883`
- point `certfile` + `keyfile` at the mounted paths

### Optional: WSS (MQTT over WebSocket)
If UI tooling needs broker access from a browser:
- Expose `wss://kusumc.hkbase.in/mqtt`
- Nginx terminates TLS and reverse-proxies to EMQX WS listener (typically `8083` for ws or `8084` for wss). 
- Prefer **ws internally** (`emqx:8083`) and use Nginx’s HTTPS cert for the external WSS.

## Compose blueprint (copy/paste skeleton)

This is intentionally a **blueprint**. Adjust image tags, volumes, and hardening as needed.

```yaml
services:
  nginx:
    image: nginx:1.27
    container_name: kusumc-nginx
    depends_on:
      - ui-kusumc
      - go-kusumc
    ports:
      - "80:80"     # optional (redirect + ACME)
      - "443:443"   # HTTPS
    volumes:
      - ./deploy/kusumc/nginx/conf.d:/etc/nginx/conf.d:ro
      - ./deploy/kusumc/nginx/certs:/etc/nginx/certs:ro
    networks:
      - public
      - internal

  ui-kusumc:
    # Option A: static build served by a tiny web server image you publish
    # Option B: nginx serves a bind-mounted dist/ (not shown here)
    image: <registry>/ui-kusumc:<tag>
    container_name: ui-kusumc
    environment:
      - VITE_API_BASE_URL=/api
      - VITE_MQTT_WS_URL=wss://kusumc.hkbase.in/mqtt
    networks:
      - internal

  go-kusumc:
    image: <registry>/go-kusumc:<tag>
    container_name: go-kusumc
    environment:
      - GO_PORT=8081
      - TIMESCALE_URI=postgres://<user>:<pass>@timescaledb:5432/<db>?sslmode=disable
      - REDIS_URL=redis://redis:6379

      # Internal broker connectivity (docker network)
      - MQTT_HOST=emqx
      - MQTT_PORT=1883

      # Public broker endpoints advertised to devices
      - MQTT_PUBLIC_HOST=kusumc.hkbase.in
      - MQTT_PUBLIC_TLS_PORT=8883
      - MQTT_PUBLIC_WS_URL=wss://kusumc.hkbase.in/mqtt

      # Strongly recommended in prod
      - STRICT_SNAKE_WIRE=true
      - LOG_LEVEL=info
    expose:
      - "8081"
    depends_on:
      - timescaledb
      - redis
      - emqx
    networks:
      - internal

  timescaledb:
    image: timescale/timescaledb:2.15.3-pg16
    container_name: kusumc-timescaledb
    environment:
      - POSTGRES_DB=<db>
      - POSTGRES_USER=<user>
      - POSTGRES_PASSWORD=<pass>
    volumes:
      - ./deploy/kusumc/data/postgres:/var/lib/postgresql/data
    networks:
      - internal

  redis:
    image: redis:7.4
    container_name: kusumc-redis
    command: ["redis-server", "--appendonly", "yes"]
    volumes:
      - ./deploy/kusumc/data/redis:/data
    networks:
      - internal

  emqx:
    image: emqx/emqx:5.8.6
    container_name: kusumc-emqx
    ports:
      - "8883:8883"   # MQTTS (public)
      # - "1883:1883" # optional plaintext (avoid in prod)
      # - "18083:18083" # optional dashboard (prefer behind nginx + auth)
    environment:
      # Plain MQTT inside Docker
      - EMQX_LISTENERS__TCP__DEFAULT__BIND=0.0.0.0:1883

      # MQTTS listener
      - EMQX_LISTENERS__SSL__DEFAULT__BIND=0.0.0.0:8883
      - EMQX_LISTENERS__SSL__DEFAULT__SSL_OPTIONS__CERTFILE=/opt/emqx/etc/certs/fullchain.pem
      - EMQX_LISTENERS__SSL__DEFAULT__SSL_OPTIONS__KEYFILE=/opt/emqx/etc/certs/privkey.pem

      # Optional WS for browser tools (behind nginx as wss)
      - EMQX_LISTENERS__WS__DEFAULT__BIND=0.0.0.0:8083
      - EMQX_DASHBOARD__DEFAULT_USERNAME=<admin_user>
      - EMQX_DASHBOARD__DEFAULT_PASSWORD=<admin_pass>
    volumes:
      - ./deploy/kusumc/data/emqx:/opt/emqx/data
      - ./deploy/kusumc/emqx/certs:/opt/emqx/etc/certs:ro
    networks:
      - internal

networks:
  public:
    driver: bridge
  internal:
    driver: bridge
```

## Nginx vhost minimum (what it must do)

Create `./deploy/kusumc/nginx/conf.d/kusumc.conf` that:
- listens on `443 ssl http2` for `kusumc.hkbase.in`
- uses cert paths:
  - `/etc/nginx/certs/fullchain.pem`
  - `/etc/nginx/certs/privkey.pem`
- proxies:
  - `/api/` → `http://go-kusumc:8081/`
  - `/` → `http://ui-kusumc:<port>/` (depends how you package UI)
- (optional) supports `/mqtt` websocket upgrade → `http://emqx:8083/mqtt`

## Ops checklist (Day-1)
- DNS: `kusumc.hkbase.in` points to the host running Nginx.
- Firewall:
  - allow `443/tcp` and `8883/tcp`
  - optionally allow `80/tcp` only if you need ACME HTTP-01
- Cert validity:
  - cert covers `kusumc.hkbase.in`
  - renew process is defined (ACME, wildcard manual, etc.)
- Device connectivity:
  - device broker URL matches `mqtts://kusumc.hkbase.in:8883`
  - backend advertises `MQTT_PUBLIC_*` consistent with above

## Notes / intentionally deferred
- Authentication hardening for EMQX dashboard (prefer: no public port 18083; proxy behind Nginx + allowlist).
- Dedicated migration job container for schema bootstrap (can be added once go-kusumc schema is finalized).
- Secrets management (Docker secrets / SOPS / Vault) instead of `.env`.
