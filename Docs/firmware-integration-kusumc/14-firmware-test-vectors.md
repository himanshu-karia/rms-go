# Firmware Test Vectors (Copy/Paste)

## Purpose
Quick manual bench-testing vectors for firmware developers and QA.

These vectors assume the device is already bootstrapped and connected to MQTT, and that you know:
- `project_id`
- gateway/device `imei`

## Topics
- Downlink commands (server -> device): `<imei>/ondemand`
- Uplink telemetry (device -> server): `<imei>/{heartbeat|pump|data|daq}`
- Command response (device -> server): `<imei>/ondemand`

Correlation note:
- In the Go stack, responses are correlated by `correlation_id` (fallback: `msgid`) from the JSON payload.

## Common rules
- Prefer parsing `command.name` + `command.params`.
- Legacy compatibility keys may exist: `cmd` and `payload`.
- For responses, prefer sending `packet_type=ondemand_rsp` and numeric `code`.

Codes:
- `0` = accepted / acked
- `1` = failed
- `2` = wait (used by `send_immediate` when next periodic publish is ≤30s away)

---

## Vector A — reboot
### Command (server -> device)
```json
{
  "packet_type": "ondemand_cmd",
  "msgid": "<corr-uuid>",
  "correlation_id": "<corr-uuid>",
  "command": {"name": "reboot", "params": {}},
  "payload": {},
  "ts": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "reboot"
}
```

### Expected response (device -> server)
```json
{
  "packet_type": "ondemand_rsp",
  "correlation_id": "<corr-uuid>",
  "status": "ack",
  "code": 0,
  "message": "reboot scheduled",
  "ts": 1760870400456
}
```

---

## Vector B — rebootstrap
### Command
```json
{
  "packet_type": "ondemand_cmd",
  "msgid": "<corr-uuid>",
  "correlation_id": "<corr-uuid>",
  "command": {"name": "rebootstrap", "params": {}},
  "payload": {},
  "ts": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "rebootstrap"
}
```

### Expected response
```json
{
  "packet_type": "ondemand_rsp",
  "correlation_id": "<corr-uuid>",
  "status": "ack",
  "code": 0,
  "message": "bootstrap refresh scheduled",
  "ts": 1760870400456
}
```

---

## Vector C — set_ping_interval_sec
### Command
```json
{
  "packet_type": "ondemand_cmd",
  "msgid": "<corr-uuid>",
  "correlation_id": "<corr-uuid>",
  "command": {
    "name": "set_ping_interval_sec",
    "params": {"interval_sec": 60}
  },
  "payload": {"interval_sec": 60},
  "ts": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "set_ping_interval_sec"
}
```

### Expected response
```json
{
  "packet_type": "ondemand_rsp",
  "correlation_id": "<corr-uuid>",
  "status": "ack",
  "code": 0,
  "applied": {"interval_sec": 60},
  "ts": 1760870400456
}
```

---

## Vector D — send_immediate (burst vs wait)
Device behavior:
- If next periodic publish is ≤30s away, respond with `code=2` and do **not** do an immediate burst.
- Otherwise, publish an immediate burst (1–N packets) and respond with `code=0`.

Diagram:

![](diagrams/11-send-immediate-decision.flowchart.svg)

### Command
```json
{
  "packet_type": "ondemand_cmd",
  "msgid": "<corr-uuid>",
  "correlation_id": "<corr-uuid>",
  "command": {"name": "send_immediate", "params": {}},
  "payload": {},
  "ts": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "send_immediate"
}
```

### Expected response (WAIT)
```json
{
  "packet_type": "ondemand_rsp",
  "correlation_id": "<corr-uuid>",
  "status": "wait",
  "code": 2,
  "message": "next periodic publish is soon; skipping immediate burst",
  "ts": 1760870400456
}
```

### Expected response (BURST)
```json
{
  "packet_type": "ondemand_rsp",
  "correlation_id": "<corr-uuid>",
  "status": "ack",
  "code": 0,
  "message": "burst sent",
  "ts": 1760870400456
}
```

---

## Vector E — forwarded_data (gateway -> server)
Publish forwarded telemetry as the gateway identity (topic uses gateway IMEI), but include origin identity in payload metadata.

### Forwarded payload example
```json
{
  "imei": "<GATEWAY_IMEI>",
  "project_id": "<project_id>",
  "msgid": "fwd-001",
  "packet_type": "forwarded_data",
  "timestamp": 1760870400123,
  "TEMP": 29.8,
  "FLOW": 12.1,
  "metadata": {
    "forwarded": true,
    "origin_node_id": "field-node-111",
    "route": {
      "path": ["field-node-111", "gateway-001"],
      "hops": 1,
      "ingress": "mesh"
    }
  }
}
```

If origin identity is missing (`origin_node_id` and `origin_imei` empty), server ingests but marks payload `suspicious`.
