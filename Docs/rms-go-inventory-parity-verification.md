# rms-go inventory + parity verification (refer-rms-deploy + unified-go)

Date: 2026-02-24

## Scope
This document compares:
1) Government/legacy RMS protocol docs: `refer-rms-deploy/RMS JSON MQTT Topics MDs/*`
2) Unified platform feature set (reference implementation): `unified-go/`
3) Current target system: `rms-go/` (`go-kusumc/` backend + `ui-kusumc/` frontend)

Goal: verify that govt MQTT topics/payload nuances are mirrored, unified-go “extra” platform capabilities (rules/automation, etc.) are retained where needed, and rms-go is internally consistent (URLs/configs/topics match end-to-end).

---

## A) External surfaces (URLs, ports, entrypoints)

### A1) Reverse proxy and public endpoints
Source: `rms-go/go-kusumc/infra/nginx/nginx.conf` + `rms-go/go-kusumc/docker-compose*.yml`

- HTTPS entrypoint: `:443`
  - `/api/*` → backend HTTP (Fiber)
  - `/mqtt` → EMQX WebSocket listener (proxy via nginx; browser uses `wss://<host>/mqtt`)
- MQTTS entrypoint: `:8883` (TLS terminated at nginx stream; proxied to EMQX tcp)

### A2) Backend HTTP server
- Internal: backend listens on `GO_PORT` (default `8081`).
- Health endpoints:
  - `GET /health`
  - `GET /api/health`
  - `GET /api/v1/health`

### A3) Key env knobs (URLs and allowed origins)
- `FRONTEND_ORIGINS` (CORS allowlist)
- `VITE_URL` (fallback CORS origin)
- MQTT public broker info returned to devices during bootstrap:
  - `MQTT_PUBLIC_HOST`, `MQTT_PUBLIC_PORT`, `MQTT_PUBLIC_PROTOCOL`
  - `MQTT_PUBLIC_URLS` (comma-separated endpoints override)

---

## B) Govt protocol parity (refer-rms-deploy) — MQTT topics and payloads

### B1) Topics (required)
Source: `refer-rms-deploy/RMS JSON MQTT Topics MDs/MQTT_TOPICS.md`

Required topic shapes:
- `<IMEI>/heartbeat`
- `<IMEI>/pump`
- `<IMEI>/data` (documented as PumpData)
- `<IMEI>/daq`
- `<IMEI>/ondemand` (both command + response)

Status in rms-go/go-kusumc:
- MQTT subscribe includes `+/heartbeat,+/pump,+/data,+/daq,+/ondemand`.
- Packet type inference handles `/data → pump` and `/ondemand → command/response`.

### B2) Payload formats (required)
Source: `refer-rms-deploy/RMS JSON MQTT Topics MDs/JSON_FORMATS.md`

- HeartbeatData: raw keys like `IMEI`, `TIMESTAMP`, `ASN`, etc.
- PumpData: raw keys like `PDHR1`, `PTOTHR1`, `PDC1V1`, etc.
- DaqData: raw keys like `AI11`, `DI11`, `DO11`, etc.
- OnDemandCommand: `{ msgid, timestamp, type, cmd, DO1, ... }`
- OnDemandResponse: `{ timestamp, status, DO1, PRUNST1, ... }`

Status in rms-go/go-kusumc:
- Ingestion accepts IMEI from payload (`IMEI`) or topic prefix.
- `/ondemand` response correlation works even if response omits correlation fields (fallback to latest outstanding command request).
- Command publish shape was adjusted to match govt format (top-level `msgid`, `timestamp`, `type`, `cmd`, plus flattened params).

Important nuance (govt docs have two “layers”):
- `COMMON_PARAMETERS.md` describes a *normalized envelope* (e.g., `project_id`, `protocol_id`, `packet_type`) that is useful for platform routing/bridging.
- Govt devices often send *raw* payloads (as in `JSON_FORMATS.md`) without that envelope.
- rms-go validates primarily against the raw-per-topic schema (CSV/DNA), not against the full platform envelope.

---

## C) Internal consistency checks (end-to-end)

### C1) Topic consistency: ingestion ↔ bootstrap ↔ EMQX ACL
Verified alignment in go-kusumc:
- Ingestion subscribes to legacy topics: `+/heartbeat,+/pump,+/data,+/daq,+/ondemand`.
- Bootstrap returns device publish topics including `<imei>/heartbeat,<imei>/pump,<imei>/data,<imei>/daq` and subscribe topic `<imei>/ondemand`.
- Provisioning worker grants device ACLs matching those topics.

### C2) Service-account ACL consistency (backend-service)
The backend service user must be able to:
- subscribe to the topics the MQTT handler subscribes to
- publish to the topics used by RulesService (alerts) and command publishing

Fix applied in rms-go:
- Default `SERVICE_MQTT_PUB_TOPICS` now includes `channels/+/alerts`.
- Default `SERVICE_MQTT_SUB_TOPICS` now includes channels compatibility topics:
  - `channels/+/messages/+`, `channels/+/commands/+/resp`, `channels/+/commands/+/ack`, `devices/+/telemetry`, plus legacy `+/...`.

If you *do not* want channels compatibility at all, remove those subscriptions from the MQTT handler and remove the ACL grants.

### C3) Rules/automation gating (verified packets only)
- Rules and automation execute only when ingestion marks packets `status=verified` and has a resolved `project_id`.
- This means payload schema scoping must work for the project.

Fix applied in rms-go:
- Payload schema fallback: if `project:<projectId>` has no schema, fall back to `project:<projectType>` (so CSV scope IDs like `PM_KUSUM_SolarPump_RMS` apply).

---

## D) Module inventory (by functionality)

### D1) Southbound (device-facing) module set
- Bootstrap + credentials: `GET /api/bootstrap` (guarded), plus device-open bootstrap redirect routes
- Device-open (public-ish) endpoints (multiple aliases):
  - `/api/device-open/*`, `/api/devices/open/*`, `/api/v1/device-open/*`, `/api/v1/devices/open/*`
  - credentials (local/government), VFD models, command history/status, nodes, installation lookup
- MQTT provisioning worker: queues + applies EMQX users/ACLs and kills sessions on rotation

### D2) Ingestion + persistence
- MQTT ingestion (legacy + compat subscriptions)
- HTTP ingest:
  - `POST /api/ingest` (API key guarded)
  - telemetry HTTPS mirror endpoints (`/api/telemetry/*`)
- Persistence: Timescale/Postgres telemetry tables + Redis hot cache

### D3) Commands & command catalog
- Admin/UI endpoints:
  - `GET /api/commands/catalog`
  - `GET /api/commands/catalog-admin`
  - `POST /api/commands/send`
  - `GET /api/commands`, `/api/commands/status`, `/api/commands/responses`
- Legacy ondemand protocol:
  - publish `<imei>/ondemand` with govt OnDemandCommand JSON shape
  - correlate responses from `<imei>/ondemand`
- Retry worker: republishes pending/published commands

### D4) Rules, automation flows, thresholds
- RulesService (govaluate triggers) + MQTT alert publish to `channels/<project>/alerts`
- Automation flows (graph / action executor) run via DeviceService
- Thresholds:
  - stored in DB (`telemetry_thresholds`) and synced into Redis bundles

### D5) Admin, RBAC, audit, API keys
- Auth: `/api/auth/*` (JWT)
- ApiKey middleware supports single-project keys
- Audit middleware logs protected actions

### D6) Platform extras carried over from unified-go
Present in go-kusumc wiring:
- Shadow service
- OTA service + endpoints (`/api/ota/*`)
- Archiver service + storage provider switch (`STORAGE_TYPE` local/S3/GCS)
- Analytics history endpoints
- ERP/vertical controllers (maintenance/inventory/logistics/etc.)
- Northbound ChirpStack webhook handler

---

## E) unified-go parity (what’s retained vs simplified)

Retained (backend modules + APIs largely mirror unified-go):
- Rules + automation flows + Redis config bundles
- Commands catalog + send + history + retries
- Device provisioning + EMQX ACL lifecycle + credential rotation
- Shadow + archiver + OTA
- Device-open compatibility endpoints

Simplified vs legacy Node (`refer-rms-deploy/refer-rms-deploy/backend`):
- Node provisioning worker has richer metrics/resync lifecycle tracking and Mongo-specific history bookkeeping.
- go-kusumc provisioning worker implements core functional behavior (apply user/ACL, kill sessions, retry with backoff) but not all legacy metrics/resync nuances.

---

## F) Remaining gaps / risk notes

1) Strictness boundary (device cmd parsing)
- Commands are now emitted in strict govt shape to avoid unknown-field firmware failures.
- If a vendor firmware expects additional fields (rare), that firmware must be updated or handled via protocol profiles.

2) Ondemand correlation ambiguity (protocol limitation)
- Govt ondemand responses may not carry a correlation field.
- Fallback correlation to “latest outstanding request” is best-effort; if multiple outstanding commands exist simultaneously, matching can be wrong.

3) Envelope vs raw payload mismatch in govt docs
- Govt docs include both an envelope spec (`COMMON_PARAMETERS.md`) and raw-per-topic payload formats.
- Current rms-go behavior supports raw-per-topic ingestion directly; envelope bridging remains optional/compat.

---

## Pointers
- Govt protocol verification (detailed): `rms-go/Docs/govt-protocol-verification-kusumc.md`
- MQTT subscriptions: `rms-go/go-kusumc/internal/adapters/primary/mqtt_handler.go`
- Bootstrap broker URLs + topic lists: `rms-go/go-kusumc/internal/core/services/bootstrap_service.go`
- Provisioning worker ACLs: `rms-go/go-kusumc/internal/core/workers/mqtt_worker.go`
- Commands publish shape: `rms-go/go-kusumc/internal/core/services/commands_service.go`
- Rules alert publish: `rms-go/go-kusumc/internal/core/services/rules_service.go`
- HTTP routes composition: `rms-go/go-kusumc/cmd/server/main.go` and `rms-go/go-kusumc/internal/adapters/primary/http/router.go`
