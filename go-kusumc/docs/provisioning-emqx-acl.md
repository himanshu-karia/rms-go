# Provisioning → EMQX ACL (strict)

## Strict end-to-end steps (what must happen)

1) **Enroll device in backend (identity creation)**
  - UI/ops calls `POST /api/devices` (protected) which ultimately calls `DeviceService.CreateDevice(projectId, name, imei, attributes)`.
  - Backend generates credentials:
    - `mqtt_user = imei`
    - `mqtt_pass = random`
  - Device record is persisted in Postgres.

2) **Queue provisioning job (deferred broker config)**
  - Backend enqueues an MQTT provisioning job row in Postgres.

3) **Provisioning worker pushes identity + ACL to EMQX**
  - `internal/core/workers/mqtt_worker.go` polls and processes provisioning jobs.
  - For each job, it calls EMQX REST API:
    - create/update built-in auth user (`username/password`)
    - replace ACL rules for that user

4) **Device connects to EMQX using generated creds**
  - Device authenticates as `username=imei` and receives authorization scoped to the exact topic contract below.

5) **Device publishes telemetry / subscribes to commands**
  - Telemetry publish must match what the Go server subscribes to.
  - Command subscribe must match what the Go server publishes to.

6) **Go server uses a separate “bridge/service” identity**
  - The Go backend connects to MQTT using `BRIDGE_USERNAME`/`BRIDGE_PASSWORD`.
  - This identity must have broader permissions than devices (subscribe to many devices; publish commands/alerts).

## What exists today (code truth)

### Device creation
- Device credentials are generated in `internal/core/services/device_service.go` (`CreateDevice`):
  - `mqtt_user = imei`
  - `mqtt_pass = random hex`
  - device row is inserted into Postgres via `PostgresRepo.CreateDeviceStruct` (`internal/adapters/secondary/repo_extensions.go`).
  - a provisioning job is queued via `PostgresRepo.CreateMqttProvisioningJob`.

### Provisioning worker
- `internal/core/workers/mqtt_worker.go` polls every 5 seconds and processes the next provisioning job.
- For each job it:
  1) loads the device row (`repo.GetDeviceByID`) and reads `auth.username/password`
  2) calls EMQX REST API via `internal/adapters/secondary/emqx_adapter.go`:
     - `ProvisionDevice(username,password)` → creates/updates EMQX built-in user
     - `UpdateACL(username, pubTopics, subTopics)` → replaces ACL rules for that user

### Current ACL topics (IMPORTANT)
The worker currently grants:
- publish: `projects/{project_id}/devices/{imei}/+`
- subscribe: `projects/{project_id}/devices/{imei}/cmd`

## Runtime topics actually used by the Go server
These are the topics the running server (`cmd/server/main.go`) actually uses:

### Ingestion subscriptions (server listens)
- `internal/adapters/primary/mqtt_handler.go` subscribes to:
  - `channels/+/messages/+`
  - `devices/+/telemetry`

### Commands publish (server → device)
- `internal/core/services/commands_service.go` publishes commands to:
  - `channels/{project_id}/commands/{imei}`
- `internal/core/services/shadow_service.go` publishes `SHADOW_SYNC` to the same command topic.

### Shadow “birth” subscription
- `internal/core/services/shadow_service.go` subscribes to:
  - `device/+/connected`

## Strictness gap (what must be aligned)
Today, a provisioned device (per `mqtt_worker.go`) is NOT granted permission to publish to the topics the server is listening to (`channels/...` or `devices/...`), and it is NOT granted permission to subscribe to the command topic the server publishes to (`channels/{project_id}/commands/{imei}`).

Concretely:
- Device publish allowed: `projects/{pid}/devices/{imei}/+`
- Server ingest listens: `channels/{pid}/messages/{imei}` and/or `devices/{imei}/telemetry`
- Server command publishes: `channels/{pid}/commands/{imei}`

## Recommended strict ACL mapping (minimal)
Pick *one* telemetry topic family for devices and make provisioning match it.

### Option A (matches current frontend + server subscription): `channels/...`
- Device publish:
  - `channels/{project_id}/messages/{imei}`
- Device subscribe:
  - `channels/{project_id}/commands/{imei}`

### Option B (matches current server subscription): `devices/...`
- Device publish:
  - `devices/{imei}/telemetry`
- Device subscribe:
  - (still needed for commands) `channels/{project_id}/commands/{imei}` (or move commands under `devices/{imei}/cmd` and update server)

### Shadow birth topic (if used)
If devices are expected to emit “online” events:
- Device publish:
  - `device/{imei}/connected`

## Topic contract (recommended canonical set)

If you want strictness with minimal surface area, align everything to the `channels/...` family:

- **Telemetry (device → broker → Go ingest)**
  - publish: `channels/{project_id}/messages/{imei}`
  - Go subscribes: `channels/+/messages/+`
- **Commands (Go → broker → device)**
  - subscribe (device): `channels/{project_id}/commands/{imei}`
  - Go publishes: `channels/{project_id}/commands/{imei}`
- **Alerts (Go → broker)**
  - publish (Go): `channels/{project_id}/alerts`
  - devices do not need permission unless they subscribe to alerts

## Bridge/service identities (server-side)
The Go backend connects to MQTT using `BRIDGE_USERNAME`/`BRIDGE_PASSWORD` in:
- `internal/adapters/primary/mqtt_handler.go`
- `internal/core/services/commands_service.go`
- `internal/core/services/shadow_service.go`

That identity should have broad permissions to:
- subscribe: `channels/+/messages/+`, `devices/+/telemetry`, `device/+/connected`
- publish: `channels/+/commands/+`, `channels/+/alerts`

## Strictness notes

- Provisioning strictness depends on the **same canonical topics** being used in:
  - device firmware publish/subscribe
  - frontend LIVE simulation publish/subscribe
  - Go server subscriptions and publishes
  - EMQX ACL rules created by the worker
- If you keep both `channels/...` and `devices/...` ingest subscriptions enabled, you must either:
  - authorize both families for each device identity, or
  - designate one family for production and disable the other.

## ACL hardening pattern
Two safety properties should be enforced by the Go worker:

1) **Replace ACLs idempotently**
  - Always replace the full rule set for the device identity (don’t append/merge).

2) **Deny-all fallback**
  - After granting the explicit allow rules, add a final rule that denies everything else (e.g. topic `#`, action `all`, permission `deny`).
  - This prevents unintended access if topic families drift.

## EMQX REST auth
`internal/adapters/secondary/emqx_adapter.go` reads:
- `EMQX_API_URL` (default `http://emqx:18083/api/v5`)
- `EMQX_APP_ID` / `EMQX_APP_SECRET` (basic auth)
