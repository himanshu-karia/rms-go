# Contracts: HTTPS, MQTTS, WSS (App ↔ Server)

Date: 2026-03-04

## 1) Transport recommendation

Primary: HTTPS + WSS
- Use HTTPS for auth, control, batch ingest
- Use WSS for near-real-time uplink/downlink events

Optional: MQTTS from app
- Only for advanced deployments
- Requires strict mobile ACL and per-phone credential lifecycle

Reasoning
- HTTPS/WSS reduces broker credential distribution risk to phones
- Easier RBAC, audit, and revocation than direct broker auth for every handset

## 2) Authentication

- JWT bearer token for all `/api/mobile/*`
- Token claims must include:
  - user_id
  - org_id
  - allowed project scope
  - phone/client id

## 3) HTTPS contracts (proposed)

## 3.1 Register phone
`POST /api/mobile/clients/register`

Request
```json
{
  "device_fingerprint": "android-uuid-or-attested-id",
  "device_name": "FieldPhone-12",
  "platform": "android",
  "app_version": "1.0.0"
}
```

Response
```json
{
  "client_id": "mob_123",
  "status": "pending_approval"
}
```

## 3.2 Open bridge session
`POST /api/mobile/bridge/sessions`

Request
```json
{
  "project_id": "pm-kusum-solar-pump-msedcl",
  "device_id": "device-uuid",
  "transport": "ble"
}
```

Response
```json
{
  "bridge_session_id": "bs_abc",
  "status": "active",
  "expires_at": "2026-03-04T12:00:00Z"
}
```

## 3.3 Upload packets batch
`POST /api/mobile/bridge/sessions/{sessionId}/packets:ingest`

Request
```json
{
  "packets": [
    {
      "idempotency_key": "sha256:...",
      "captured_at": "2026-03-04T11:00:00Z",
      "topic_suffix": "data",
      "raw_payload": {
        "IMEI": "999...",
        "TIMESTAMP": 1741086000,
        "PDHR1": 1.2
      }
    }
  ]
}
```

Response
```json
{
  "accepted": 980,
  "duplicates": 15,
  "rejected": 5,
  "results": [
    {
      "idempotency_key": "sha256:...",
      "status": "accepted"
    },
    {
      "idempotency_key": "sha256:...",
      "status": "rejected",
      "reason": "schema_validation_failed"
    }
  ]
}
```

## 3.4 Submit command result
`POST /api/mobile/bridge/sessions/{sessionId}/commands/{commandId}:result`

Request
```json
{
  "msgid": "cmd-uuid",
  "status": "success",
  "response_payload": {
    "timestamp": 1741086000,
    "status": "OK",
    "DO1": 1
  }
}
```

## 4) WSS contracts (proposed)

Endpoint
- `wss://<host>/api/mobile/ws`

Client messages
- `bridge.heartbeat`
- `command.result`
- `packet.stream` (optional small-batch mode)

Server messages
- `bridge.ack`
- `command.request`
- `sync.backpressure`
- `session.expiring`

Envelope
```json
{
  "type": "packet.stream",
  "ts": "2026-03-04T11:00:00Z",
  "session_id": "bs_abc",
  "payload": { }
}
```

## 5) MQTTS contracts (optional mode)

If enabled, app uses dedicated phone MQTT identity:
- publish: `mobile/{client_id}/uplink/{project_id}/{device_id}`
- subscribe: `mobile/{client_id}/downlink/#`

Server bridge worker maps mobile topic payload into canonical ingestion pipeline.

## 6) Error model

HTTP standard:
- `400` validation
- `401` auth
- `403` scope violation
- `409` duplicate/conflict
- `413` batch too large
- `429` rate limit
- `500` internal

Error body
```json
{
  "code": "scope_violation",
  "message": "Device does not belong to your assigned project",
  "trace_id": "req-..."
}
```

## 7) Contract invariants

- Raw payload must remain unchanged from device source
- Server metadata must be additive under `metadata.source = mobile_bridge`
- Idempotency key uniqueness must be enforced per project/device/time window
