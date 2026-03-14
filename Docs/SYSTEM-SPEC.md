# RMS-Go System Spec (KUSUMC)

## 1. Product boundary
RMS-Go is the KUSUMC-specific product workspace under `rms-go/`.

In scope:
- `go-kusumc` backend APIs + MQTT ingest/provisioning + persistence + rules path
- `ui-kusumc` frontend UI
- Firmware contract docs for legacy govt protocol

Out of scope for implementation (reference only):
- `unified-go/`
- `new-frontend/`
- `refer-rms-deploy/`

## 2. Runtime model
- Primary runtime target: Linux containers.
- Local host OS can be Windows/Linux/macOS for orchestration.
- Deployment shape is multi-service:
  - Go API server
  - EMQX broker
  - Redis
  - Timescale/Postgres
  - Nginx

## 3. Contract model

### 3.1 MQTT topics (legacy govt)
- Telemetry uplink: `<imei>/heartbeat`, `<imei>/data`, `<imei>/daq`
- Optional compatibility path: `<imei>/pump` (profile-gated)
- Command downlink/up response: `<imei>/ondemand`

### 3.2 REST surface (high-level)
- Auth + capability-protected routes
- Device lifecycle (CRUD/rotate creds)
- Command catalog + command send/status
- Telemetry ingest/history/latest/live
- Diagnostics endpoints for dead-letter replay/status

### 3.3 Configuration controls
- `MQTT_TOPIC_PROFILE` controls strict vs compat topic behavior.
- `INGEST_DEADLETTER_REPLAY_*` controls replay interval/batch/retention.
- Compose profile influences URL defaults in integration scripts.

## 4. Data and reliability guarantees
- Ingest supports identity normalization (`IMEI`/`imei`) and topic-based inference.
- Verification/schema path uses config bundle + fallback strategy.
- Overflow does not silently disappear:
  - dead-letter queue capture
  - overflow/replay counters
  - replay worker + manual replay endpoint

## 5. Security/guardrails
- API + auth + capability checks protect control paths.
- Live telemetry stream requires issued/validated ticket (Redis-backed TTL).
- EMQX ACL provisioning enforces device-topic access boundaries.

## 6. Test and acceptance baseline
- Build must pass: `go build ./cmd/server`
- Integration harness path must pass on `docker-compose.integration.yml`
- Ordered E2E harness exists for deterministic stage-by-stage verification.
