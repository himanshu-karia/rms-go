# Firmware Test Vectors (Copy/Paste)

## Purpose
Quick manual bench-testing vectors for firmware developers and QA.

These vectors assume the device is already bootstrapped and connected to MQTT, and that you know:
- `project_id`
- gateway/device `imei`

## Topics
- Downlink commands (server -> device): `<imei>/ondemand`
- Uplink telemetry (device -> server): `<imei>/{heartbeat|data|daq}`
- PumpData uplink topic: `<imei>/data` only
- Command response (device -> server): `<imei>/ondemand`

Correlation note:
- In the Go stack, responses are correlated by `msgid` when present (and by explicit correlation keys when provided).

## Common rules
- Govt legacy downlink commands use top-level keys: `msgid`, `timestamp`, `type=ondemand_cmd`, `cmd`, plus params at top level.
- Govt legacy responses use `status` and may include `code`.
- Strong recommendation: echo `msgid` from the command in the response for deterministic correlation.

Codes:
- `0` = accepted / acked
- `1` = failed
- `2` = wait (used by `send_immediate` when next periodic publish is ≤30s away)

---

## Vector A — reboot
### Command (server -> device)
```json
{
  "msgid": "<corr-uuid>",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "reboot"
}
```

### Expected response (device -> server)
```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "code": 0,
  "message": "reboot scheduled",
  "msgid": "<corr-uuid>"
}
```

---

## Vector B — rebootstrap
### Command
```json
{
  "msgid": "<corr-uuid>",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "rebootstrap"
}
```

### Expected response
```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "code": 0,
  "message": "bootstrap refresh scheduled",
  "msgid": "<corr-uuid>"
}
```

---

## Vector C — set_ping_interval_sec
### Command
```json
{
  "msgid": "<corr-uuid>",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "set_ping_interval_sec",
  "interval_sec": 60
}
```

### Expected response
```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "code": 0,
  "applied": {"interval_sec": 60},
  "msgid": "<corr-uuid>"
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
  "msgid": "<corr-uuid>",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "send_immediate"
}
```

### Expected response (WAIT)
```json
{
  "timestamp": 1760870400456,
  "status": "wait",
  "code": 2,
  "message": "next periodic publish is soon; skipping immediate burst",
  "msgid": "<corr-uuid>"
}
```

### Expected response (BURST)
```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "code": 0,
  "message": "burst sent",
  "msgid": "<corr-uuid>"
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
