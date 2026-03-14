# RMS API Contracts (Provisioning, Bootstrap, Commands)

This document defines the minimal HTTP contracts to support RMS devices and UI flows on unified-go.

## 1) Provisioning (UI → Backend)
- Endpoint: `POST /api/devices`
- Auth: protected (same as other device APIs)
- Request body:
```json
{
  "project_id": "proj_x",
  "imei": "8696...",
  "name": "Pump-01",
  "protocol_id": "rms-v1",
  "contractor_id": "contractor-a",
  "supplier_id": "supplier-a",
  "manufacturer_id": "rms-oem",
  "model": "optional"
}
```
- Response:
```json
{
  "device_id": "uuid",
  "project_id": "proj_x",
  "imei": "8696...",
  "protocol_id": "rms-v1",
  "contractor_id": "contractor-a",
  "supplier_id": "supplier-a",
  "manufacturer_id": "rms-oem",
  "envelope_keys": ["packet_type","project_id","protocol_id","contractor_id","supplier_id","manufacturer_id","device_id","imei","ts","msg_id"],
  "mqtt": {
    "topic": "channels/proj_x/messages/8696..."
  }
}
```
- Side effects: insert into `devices` table (with identity fields), enqueue MQTT provisioning job if needed, update Redis config caches for project/device.

## 2) Bootstrap (Device → Backend)
- Endpoint: `GET /api/bootstrap?imei=...`
- Auth: bootstrap token or HMAC on imei (TBD).
- Response:
```json
{
  "project_id": "proj_x",
  "device_id": "uuid",
  "protocol_id": "rms-v1",
  "contractor_id": "contractor-a",
  "supplier_id": "supplier-a",
  "manufacturer_id": "rms-oem",
  "mqtt": {
    "host": "mqtt.example.com",
    "port": 1883,
    "topic": "channels/proj_x/messages/8696...",
    "command_topic": "channels/proj_x/commands/8696..."
  },
  "envelope": {
    "required": ["packet_type","project_id","protocol_id","contractor_id","supplier_id","manufacturer_id","device_id","imei","ts","msg_id"]
  },
  "payloadSchemas": { /* merged schemas for project */ }
}
```
- Source of truth: Redis config cache; DB fallback.

## 3) Command flow (UI → Backend → EMQX → Device)
- Submit command: `POST /api/commands`
```json
{
  "project_id": "proj_x",
  "device_id": "uuid" /* or imei */,
  "command": "start_pump",
  "payload": {"DO1": 1}
}
```
- Backend actions: validate ACL; resolve imei; publish to `channels/{project_id}/commands/{imei}`; insert command record (`status=pending`, `msg_id`); return `{msg_id, status: "pending"}`.
- Device response: publish to `channels/{project_id}/commands/{imei}` with `packet_type=ondemand_rsp` (or on telemetry topic) including `msg_id/status`.
- Ingestion: on `packet_type=ondemand_rsp`, update command record (`ack/success/fail`, response body, timestamps).
- Query commands: `GET /api/commands?device_id=...&limit=...` returns recent commands with statuses and responses.

## 3.1) Device configuration apply (UI → Backend → EMQX → Device)

Device configuration is delivered using the same MQTT command pipeline as normal commands, but uses a deterministic correlation ID so the configuration record can be finalized reliably.

### Queue configuration (UI → Backend)
- Queue: `POST /api/devices/:idOrUuid/configuration`
  - Request body: configuration JSON (stored verbatim as the configuration payload)
  - Backend actions:
    - Insert a `device_configurations` row with `status=pending` and generated `config_id`
    - Publish MQTT command `apply_device_configuration` to `channels/{project_id}/commands/{imei}`
    - Set `msgid == correlation_id == config_id`
  - Response: the stored configuration record (includes `id=config_id`, `status`, timestamps)

### Device response (Device → Backend)
- Publish `ondemand_rsp` with `correlation_id=config_id` (preferred topic `channels/{project_id}/commands/{imei}/resp`).
- Ingestion finalizes the matching configuration record:
  - ack/success → `acknowledged`
  - failed/timeout → `failed`

### Configuration tracking endpoints
- Pending: `GET /api/devices/:idOrUuid/configuration/pending`
- Manual status ack: `POST /api/devices/:idOrUuid/configuration/ack`

## 4) MQTT topics (runtime)
- Telemetry: `channels/{project_id}/messages/{imei}` with envelope+body per RMS schemas.
- Commands: `channels/{project_id}/commands/{imei}` (publish by backend, subscribe by device).

## 5) Required backend changes
- MQTT handler: pass real topic to ingestion (done separately).
- Controllers: add handlers for `/api/bootstrap` and `/api/commands`; extend `/api/devices` if needed.
- Persistence: add `commands` table (id, project_id, device_id, imei, msg_id, command, payload, status, response, created_at, updated_at).
- Config sync: ensure `payloadSchemas` and identity fields land in Redis for bootstrap.

## 6) Validation expectations
- Ingestion must see lower-case `imei` and required envelope keys; with loaded `payloadSchemas`, packets are marked `verified`.
- Command responses carry the originating `msg_id` to correlate and update status.
