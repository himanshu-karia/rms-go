# Server Scope of Work for App Support

Date: 2026-03-04
Target backend: `rms-go/go-kusumc`

## 1) Objective

Add backend capabilities so mobile app can act as a secure field bridge for RMS devices, including command relay and offline log upload on behalf of devices.

## 2) Required server capabilities

## 2.1 Mobile auth and phone enrollment
- New entity: `mobile_clients` (phone identity)
- Link phone to user/org/project scope
- Server APIs to register/approve/revoke phone
- Policy: max active phones per user

## 2.2 App-to-device bridge sessions
- New entity: `bridge_sessions`
- Session lifecycle: opened, active, paused, closed
- Fields: phone_id, user_id, device_id, transport, started_at, ended_at, metadata

## 2.3 On-behalf ingestion path
- New ingestion mode flag: `source = mobile_bridge`
- Preserve raw payload exactly as received from device
- Attach ingestion metadata:
  - `source_phone_id`
  - `source_user_id`
  - `bridge_session_id`
  - `origin_transport` (ble|bt|wifi)
- Enforce project/device authorization for impersonation

## 2.4 Command relay and reconciliation
- Mobile command submit endpoint
- Command result submit endpoint (ack/resp)
- Correlation model: `command_id`, `msgid`, `bridge_session_id`

## 2.5 Sync and dedup
- Idempotency key required per packet
- Batch ingest endpoint with partial success response
- Duplicate detection and reason coding

## 2.6 Observability and audit
- Per-phone metrics:
  - packets uploaded
  - duplicates
  - rejects
  - avg latency
- Audit logs for on-behalf actions
- Diagnostics endpoint for mobile sync jobs

## 3) Data model additions

Suggested tables:
- `mobile_clients`
- `mobile_client_assignments`
- `bridge_sessions`
- `mobile_ingest_batches`
- `mobile_ingest_events`
- optional `mobile_command_events`

## 4) API backlog (high level)

- Auth and enrollment
  - `POST /api/mobile/auth/login`
  - `POST /api/mobile/clients/register`
  - `POST /api/mobile/clients/:id/approve`
  - `POST /api/mobile/clients/:id/revoke`

- Bridge and sync
  - `POST /api/mobile/bridge/sessions`
  - `POST /api/mobile/bridge/sessions/:id/packets:ingest`
  - `POST /api/mobile/bridge/sessions/:id/commands:submit`
  - `POST /api/mobile/bridge/sessions/:id/commands/:commandId:result`
  - `GET /api/mobile/bridge/sessions/:id/status`

- Admin/ops
  - `GET /api/mobile/clients`
  - `GET /api/mobile/sync/jobs`
  - `GET /api/mobile/sync/jobs/:jobId`

## 5) Priority implementation plan

P0
- Phone enrollment + auth
- Bridge session creation
- Packet ingest batch endpoint with dedup/idempotency
- Basic audit + metrics

P1
- Command relay + result reconciliation
- Sync diagnostics UI endpoints
- Fine-grained RBAC and throttling

P2
- QoS controls, backpressure hints, adaptive upload windows
- Advanced reconciliation reports and anomaly alerts

## 6) Backward compatibility constraints

- Do not break existing device direct MQTT ingest paths
- Keep existing telemetry payload contract unchanged
- Mobile bridge must be additive and explicitly tagged

## 7) Server acceptance criteria

- Phone enrollment and scoped auth enforced
- Unauthorized device impersonation blocked
- Batch ingest supports partial accept/reject with explicit reasons
- Audit logs include user + phone + device attribution
- Existing web/device E2E suite remains green
