# REST API Contract (Firmware-facing, Legacy-only)

This is the small set of HTTP endpoints firmware may call.

Base URL rule:
- Firmware REST base URL must be runtime-configurable.
- Firmware must not assume a fixed domain (`localhost`, `iot.hkbase.in`, etc.).
- MQTT URL must come from bootstrap/credential response endpoint fields.

## Bootstrap (required)
- `GET /api/bootstrap?imei=<imei>`
- Headers:
  - `x-api-key: <device-api-key>`

## Device-open (optional, operational recovery)
Preferred prefixes:
- `/api/device-open/*`
- `/api/v1/device-open/*`

### Credential refresh (optional)
If your firmware needs a credential-refresh call without re-running bootstrap:
- `GET /api/device-open/credentials/local?imei=<imei>`

Response shape (actual wire contract):
```json
{
  "device": {
    "uuid": "dev-3f2b9c27",
    "imei": "869630050762180",
    "project_id": "pm-kusum-solar-pump-msedcl"
  },
  "credential": {
    "client_id": "client-869630050762180",
    "username": "u_869630050762180",
    "password": "p_...",
    "endpoints": [
      {
        "protocol": "mqtts",
        "host": "broker.example.com",
        "port": "8883",
        "url": "mqtts://broker.example.com:8883"
      }
    ],
    "publish_topics": ["869630050762180/heartbeat", "869630050762180/data", "869630050762180/daq", "869630050762180/ondemand", "869630050762180/errors"],
    "subscribe_topics": ["869630050762180/ondemand"],
    "mqtt_access_applied": true,
    "lifecycle": "active",
    "issued_at": "2026-02-25T10:00:00Z",
    "valid_to": null
  }
}
```

Endpoint selection rule:
- Prefer `credential.endpoints[0].url` when present.
- Else derive URL from `credential.endpoints[0].protocol + host + port`.
- Else fall back to bootstrap `primary_broker` endpoint.

Note: if this endpoint is unavailable in your deployment, fall back to `GET /api/bootstrap?imei=...`.

### Command backlog / recovery
- `GET /api/device-open/commands/status?imei=<imei>`
- `GET /api/device-open/commands/history?imei=<imei>&limit=<n>`
- `GET /api/device-open/commands/responses?imei=<imei>&limit=<n>`

### Gateway node list (only if forwarding)
- `GET /api/device-open/nodes?imei=<gateway_imei>`

### VFD / RS485 metadata (only if device controls a VFD)
- `GET /api/device-open/vfd?imei=<imei>` (or `device_uuid=<uuid>`)

### Error ingest (MQTT fallback)
If MQTT publish to `<imei>/errors` fails (no broker, auth/ACL problems, repeated disconnect), firmware may POST the same event via HTTP.

- `POST /api/device-open/errors?imei=<imei>`
  - Also available as aliases:
    - `POST /api/devices/open/errors?imei=<imei>`
    - `POST /api/v1/device-open/errors?imei=<imei>`
    - `POST /api/v1/devices/open/errors?imei=<imei>`

Body (schema matches the `<imei>/errors` topic):
```json
{
  "open_id": "dev-9c6b8de2",
  "timestamp": 1760870400456,
  "error_code": "RS485_CRC_ERROR",
  "error_data": {
    "bus": "rs485",
    "slave_id": 1,
    "count": 12,
    "window_sec": 300
  },
  "severity": "warning",
  "message": "optional human hint"
}
```

Response:
- `202 Accepted` with `{ "status": "accepted" }`

## Notes
- Use snake_case on the wire (`device_uuid`, `project_id`, etc.).
- Avoid relying on temporary camelCase aliases.
