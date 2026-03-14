# Device API Request/Response Samples

## Purpose
This file provides practical request/response examples for firmware-facing device APIs.

## Last refresh
- Date: 2026-02-21
- Scope: Added secure credential claim + download-token samples (canonical snake_case: `expires_at`, `device_id`, `claimed_at`, etc.).

## Canonical routes (device-open family)
Preferred prefixes:
- `/api/device-open`
- `/api/v1/device-open`

Aliases (supported for backward compatibility):
- `/api/devices/open`
- `/api/v1/devices/open`

Bootstrap is an exception:
- **Canonical:** `/api/bootstrap?imei=...` (requires `x-api-key`)
- **Alias routes:** `/api/device-open/bootstrap` (and `/api/v1/...`) redirect to `/api/bootstrap` with the same querystring.

## Base URLs (HTTP+MQTT and HTTPS+MQTTS)
- **HTTP (dev/integration):**
  - REST base: `http://localhost:8081`
  - MQTT base (plain): `mqtt://localhost:1884` (only if using `docker-compose.integration.yml`)
- **HTTPS (prod-like via Nginx):**
  - REST base: `https://localhost`
  - MQTTS base: `mqtts://localhost:8883`

Note: the broker endpoints advertised in REST responses can be controlled via `MQTT_PUBLIC_PROTOCOL/HOST/PORT` or `MQTT_PUBLIC_URLS`.

## Identifier naming convention in examples
- HTTP query params and JSON keys use **snake_case** as the canonical wire format.
  - Prefer `device_uuid`, `device_id`, `project_id`, `packet_type`, `correlation_id`, etc.
- Legacy camelCase aliases may still be accepted by some endpoints during migration, but clients should not rely on them.

## 1) Bootstrap

### Request
```bash
curl -i -X GET "http://localhost:8081/api/bootstrap?imei=359762081234567" \
  -H "x-api-key: <device-api-key>"

# HTTPS / prod-like (Nginx) variant:
# curl -k -i -X GET "https://localhost/api/bootstrap?imei=359762081234567" \
#   -H "x-api-key: <device-api-key>"
```

### Response (example)
```json
{
  "status": "success",
  "identity": {
    "imei": "359762081234567",
    "uuid": "dev-9c6b8de2",
    "lifecycle": "active",
    "protocol_id": "proto-local-v2",
    "contractor_id": "ctr-01",
    "supplier_id": "sup-01",
    "manufacturer_id": "mfg-01",
    "org_id": "org-01"
  },
  "credentials": {
    "protocol": "mqtt",
    "protocol_id": "proto-local-v2",
    "host": "broker.example.net",
    "port": "8883",
    "username": "dev_user",
    "password": "***",
    "client_id": "dev-9c6b8de2",
    "publish_topics": [
      "359762081234567/heartbeat",
      "359762081234567/data",
      "359762081234567/daq",
      "359762081234567/ondemand",
      "359762081234567/errors"
    ],
    "subscribe_topics": ["359762081234567/ondemand"],
    "endpoints": ["mqtts://broker.example.net:8883"]
  },
  "primary_broker": {
    "protocol": "mqtt",
    "protocol_id": "proto-local-v2",
    "host": "broker.example.net",
    "port": "8883",
    "username": "dev_user",
    "password": "***",
    "client_id": "dev-9c6b8de2",
    "publish_topics": [
      "359762081234567/heartbeat",
      "359762081234567/data",
      "359762081234567/daq",
      "359762081234567/ondemand",
      "359762081234567/errors"
    ],
    "subscribe_topics": ["359762081234567/ondemand"],
    "endpoints": ["mqtts://broker.example.net:8883"]
  },
  "govt_broker": {
    "protocol": "mqtt",
    "protocol_id": "proto-govt-v1",
    "client_id": "govt-client-001",
    "username": "gov_user",
    "password": "***",
    "publish_topics": [
      "359762081234567/heartbeat",
      "359762081234567/data",
      "359762081234567/daq",
      "359762081234567/ondemand",
      "359762081234567/errors"
    ],
    "subscribe_topics": ["359762081234567/ondemand"],
    "endpoints": ["mqtt://gw.gov.example:1883"]
  },
  "context": {
    "project": {"id": "prj-01", "name": "Project prj-01"},
    "location": {"lat": 19.098, "lng": 72.871, "village": "Kherwadi"},
    "beneficiary": {"name": "Ravi Patil", "phone": "9876543210"},
    "vfd_model": {"id": "vfd-001", "model": "VFD-X", "version": "1.0"},
    "vfd_assignment": {"id": "assign-01", "protocol_id": "proto-local-v2", "vfd_model_id": "vfd-001"}
  },
  "configuration": {
    "server_vendor": "SynapseIO",
    "sampling_rate_sec": 60
  },
  "envelope": {
    "required": ["imei", "project_id", "msgid", "ts", "packet_type", "sensors"]
  }
}
```

### Dual-endpoint advertising (optional)
If you want bootstrap/credentials to advertise both secure and insecure broker URLs during phased verification, set:
- `MQTT_PUBLIC_URLS=mqtts://<host>:8883,mqtt://<host>:1884`

Then `primary_broker.endpoints` will contain both URLs (order preserved).

## 2) Local credentials

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/credentials/local?imei=359762081234567"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/credentials/local?imei=359762081234567"
```

### Response (example)
```json
{
  "device": {
    "uuid": "dev-9c6b8de2",
    "imei": "359762081234567",
    "project_id": "prj-01",
    "status": "active",
    "state_id": "st-01",
    "state_authority_id": "sa-11",
    "server_vendor_id": "sv-22",
    "protocol_version_id": "proto-local-v2",
    "protocol_id": "proto-local-v2",
    "vfd_drive_model_id": "vfd-001",
    "vfd_model_id": "vfd-001"
  },
  "credential": {
    "client_id": "dev-9c6b8de2",
    "username": "dev_user",
    "password": "***",
    "endpoints": [
      {
        "protocol": "mqtts",
        "host": "broker.example.net",
        "port": "8883",
        "url": "mqtts://broker.example.net:8883"
      }
    ],
    "publish_topics": [
      "359762081234567/heartbeat",
      "359762081234567/data",
      "359762081234567/daq",
      "359762081234567/ondemand",
      "359762081234567/errors"
    ],
    "subscribe_topics": ["359762081234567/ondemand"],
    "mqtt_access_applied": true,
    "lifecycle": "active",
    "issued_at": "2026-02-18T10:02:11Z",
    "valid_to": null
  }
}
```

## 2.1) Device configuration apply (MQTT command)

The RMS UI can queue a device configuration and the backend will publish a command on:
- `<imei>/ondemand`

The command name is `apply_device_configuration`.

### Command payload (server -> device)
```json
{
  "msgid": "<config_id_uuid>",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "apply_device_configuration",
  "config_id": "<config_id_uuid>",
  "config": {
    "vfd_model_id": "vfd-model-seed",
    "overrides": {
      "rs485": {"baud": 9600}
    }
  }
}
```

### Response payload (device -> server)
Device should publish an ack/response with the same correlation id.

```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "code": 0,
  "message": "configuration applied",
  "msgid": "<config_id_uuid>"
}
```

If the device cannot apply the configuration:
```json
{
  "timestamp": 1760870400456,
  "status": "failed",
  "code": 1,
  "message": "invalid rs485 override",
  "msgid": "<config_id_uuid>"
}
```

Notes:
- If `MQTT_PUBLIC_URLS` is configured, `credential.endpoints[]` may contain multiple entries (e.g. both `mqtts://...` and `mqtt://...`).
- If the broker echoes publishes to the same client subscription, ignore your own publishes (treat inbound packets with `type=ondemand_cmd` as commands).

## 3) Government credentials

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/credentials/government?imei=359762081234567"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/credentials/government?imei=359762081234567"
```

### Response (example)
```json
{
  "device": {
    "uuid": "dev-9c6b8de2",
    "imei": "359762081234567",
    "project_id": "prj-01",
    "status": "active",
    "protocol_version_id": "proto-local-v2",
    "protocol_id": "proto-local-v2"
  },
  "credential": {
    "client_id": "govt-client-001",
    "username": "gov_user",
    "password": "***",
    "endpoints": [
      {
        "protocol": "mqtt",
        "host": "gw.gov.example",
        "port": 1883,
        "url": "mqtt://gw.gov.example:1883"
      }
    ],
    "publish_topics": ["govt/prj-01/up/359762081234567"],
    "subscribe_topics": ["govt/prj-01/down/359762081234567"],
    "lifecycle": "active",
    "issued_at": "2026-02-18T10:02:11Z",
    "valid_to": null,
    "protocol_id": "proto-govt-v1"
  }
}
```

## 4) VFD / RS485 model metadata

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/vfd?imei=359762081234567"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/vfd?imei=359762081234567"
```

### Response (example)
```json
{
  "device": {
    "uuid": "dev-9c6b8de2",
    "imei": "359762081234567",
    "project_id": "prj-01",
    "vfd_model_id": "vfd-001"
  },
  "vfd_models": [
    {
      "id": "vfd-001",
      "project_id": "prj-01",
      "manufacturer_id": "mfg-vfd-1",
      "model": "VFD-X",
      "version": "1.0",
      "rs485": {
        "baud_rate": 9600,
        "parity": "N",
        "stop_bits": 1,
        "slave_id": 1
      },
      "realtime_parameters": [
        {"key": "output_current", "register": 3010, "scale": 0.1},
        {"key": "output_frequency", "register": 3012, "scale": 0.01}
      ],
      "fault_map": [
        {"code": "E01", "meaning": "Overcurrent"}
      ],
      "command_dictionary": [
        {"key": "start", "register": 4001, "value": 1},
        {"key": "stop", "register": 4001, "value": 0}
      ]
    }
  ]
}
```

## 5) Installation + beneficiary details

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/installations/dev-9c6b8de2"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/installations/dev-9c6b8de2"
```

### Response (example)
```json
{
  "installation": {
    "id": "inst-001",
    "device_uuid": "dev-9c6b8de2",
    "project_id": "prj-01",
    "beneficiary_id": "ben-1001",
    "location": {"lat": 19.098, "lng": 72.871, "village": "Kherwadi"},
    "vfd_model_id": "vfd-001",
    "protocol_id": "proto-local-v2"
  },
  "beneficiaries": [
    {
      "id": "ben-1001",
      "name": "Ravi Patil",
      "phone": "9876543210"
    }
  ]
}
```

## 6) Commands history fallback (new)

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/commands/history?imei=359762081234567&limit=20"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/commands/history?imei=359762081234567&limit=20"
```

### Response (example)
```json
{
  "device": {
    "uuid": "dev-9c6b8de2",
    "imei": "359762081234567",
    "project_id": "prj-01",
    "status": "active",
    "protocol_version_id": "proto-local-v2",
    "protocol_id": "proto-local-v2"
  },
  "commands": [
    {
      "id": "cmdreq-01",
      "device_id": "dev-9c6b8de2",
      "project_id": "prj-01",
      "command_id": "set_vfd",
      "payload": {"speed": 45},
      "status": "published",
      "retries": 0,
      "correlation_id": "corr-5e4343",
      "created_at": "2026-02-18T10:00:00Z",
      "published_at": "2026-02-18T10:00:01Z"
    }
  ],
  "next_cursor": null
}
```

Notes:
- Prefer `imei` or `device_uuid`.
- Temporary aliases may be accepted by some handlers during migration: `deviceUuid`, `deviceId` (legacy; may be rejected when `STRICT_SNAKE_WIRE=true`).
- `limit` defaults to `20`, minimum effective value is `20` when `<=0`, and max is capped at `100`.

## 7) Commands responses fallback (new)

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/commands/responses?imei=359762081234567&limit=20"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/commands/responses?imei=359762081234567&limit=20"
```

### Response (example)
```json
{
  "device": {
    "uuid": "dev-9c6b8de2",
    "imei": "359762081234567",
    "project_id": "prj-01",
    "status": "active",
    "protocol_version_id": "proto-local-v2",
    "protocol_id": "proto-local-v2"
  },
  "responses": [
    {
      "correlation_id": "corr-5e4343",
      "device_id": "dev-9c6b8de2",
      "project_id": "prj-01",
      "raw_response": {"status": "ack", "speed": 45},
      "parsed": {"status": "ack"},
      "matched_pattern_id": "pat-01",
      "received_at": "2026-02-18T10:00:03Z"
    }
  ]
}
```

## 8) Commands status fallback (new)

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/commands/status?imei=359762081234567"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/commands/status?imei=359762081234567"
```

### Response (example)
```json
{
  "device_id": "dev-9c6b8de2",
  "project_id": "prj-01",
  "status_counts": {
    "queued": 1,
    "published": 3,
    "completed": 2,
    "failed": 0,
    "timeout": 0
  },
  "total_retries": 1,
  "pending_past_cutoff": 0,
  "worker_config": {
    "interval_ms": 5000,
    "age_seconds": 180,
    "batch": 200,
    "max_retries": 5
  }
}
```

## 9) Mesh nodes attached (forwarding config) (new)

If the gateway device forwards telemetry for child nodes, firmware can fetch the current attachment list via:

### Request
```bash
curl -i -X GET "http://localhost:8081/api/device-open/nodes?imei=359762081234567"

# HTTPS variant:
# curl -k -i -X GET "https://localhost/api/device-open/nodes?imei=359762081234567"
```

### Response (example)
```json
{
  "device": {
    "uuid": "dev-9c6b8de2",
    "imei": "359762081234567",
    "project_id": "prj-01",
    "status": "active",
    "protocol_version_id": "proto-local-v2",
    "protocol_id": "proto-local-v2"
  },
  "nodes": [
    {
      "id": "f0c6a7d9-2a0a-4d6b-9cf6-1a7c2b40d501",
      "project_id": "prj-01",
      "node_id": "field-node-111",
      "label": "Pump node 111",
      "kind": "mesh",
      "enabled": true,
      "discovered": false,
      "last_seen": "2026-02-18T10:02:11Z",
      "attributes": {},
      "link_metadata": {"source": "ui"}
    }
  ]
}
```

## 10) Common error responses (examples)

### Missing identifier
```json
{
  "message": "imei or device_uuid is required"
}
```

### Device not found
```json
{
  "message": "device not found"
}
```

### Installation not found
```json
{
  "message": "installation not found"
}
```

## 11) Secure credential claim + download (legacy device route)

These endpoints exist to support secure field provisioning flows and some legacy tooling.

Canonical wire format is still **snake_case** (camelCase aliases may exist temporarily during migration).

### 11.1) Issue a one-time download token

#### Request
```bash
curl -i -X POST "http://localhost:8081/api/devices/dev-9c6b8de2/credentials/download-token" \
  -H "content-type: application/json" \
  -d '{"expires_in_seconds": 600}'
```

#### Response (example)
```json
{
  "token": "tok-2f7b5f0a",
  "expires_at": "2026-02-21T10:20:00Z"
}
```

### 11.2) Secure claim (exchange IMEI+secret for credentials)

#### Request
```bash
curl -i -X POST "http://localhost:8081/api/devices/credentials/claim" \
  -H "content-type: application/json" \
  -d '{
    "imei": "359762081234567",
    "secret": "<bootstrap-secret>",
    "audience": "firmware"
  }'
```

#### Response (example)
```json
{
  "device": {
    "device_id": "dev-9c6b8de2",
    "imei": "359762081234567"
  },
  "credential": {
    "client_id": "dev-9c6b8de2",
    "username": "dev_user",
    "password": "***",
    "endpoints": [
      {
        "protocol": "mqtts",
        "host": "broker.example.net",
        "port": "8883",
        "url": "mqtts://broker.example.net:8883"
      }
    ],
    "publish_topics": [
      "359762081234567/heartbeat",
      "359762081234567/data",
      "359762081234567/daq",
      "359762081234567/ondemand"
    ],
    "subscribe_topics": ["359762081234567/ondemand"],
    "mqtt_access_applied": true,
    "lifecycle": "active",
    "issued_at": "2026-02-21T10:02:11Z",
    "valid_to": null
  },
  "claim": {
    "type": "secure",
    "claimed_at": "2026-02-21T10:02:11Z",
    "previous_claim_at": null,
    "total_secure_claims": 1
  },
  "token": {
    "value": "jwt-or-session-token",
    "expires_at": "2026-02-21T12:02:11Z"
  }
}
```

### 11.3) Download credentials by token

#### Request
```bash
curl -i -X GET "http://localhost:8081/api/devices/credentials/download?token=tok-2f7b5f0a"
```

#### Response (example)
```json
{
  "type": "local",
  "device_id": "dev-9c6b8de2",
  "imei": "359762081234567",
  "client_id": "dev-9c6b8de2",
  "username": "dev_user",
  "password": "***",
  "issued_at": "2026-02-21T10:02:11Z",
  "valid_to": null,
  "credential": {
    "client_id": "dev-9c6b8de2",
    "username": "dev_user",
    "password": "***",
    "endpoints": [
      {
        "protocol": "mqtts",
        "host": "broker.example.net",
        "port": "8883",
        "url": "mqtts://broker.example.net:8883"
      }
    ],
    "topics": {
      "publish": [
        "359762081234567/heartbeat",
        "359762081234567/data",
        "359762081234567/daq",
        "359762081234567/ondemand"
      ],
      "subscribe": ["359762081234567/ondemand"]
    },
    "issued_at": "2026-02-21T10:02:11Z",
    "valid_to": null,
    "lifecycle": "active"
  }
}
```
