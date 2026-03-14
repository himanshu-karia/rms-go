# Firmware Dev Playbook (Server-Facing)

## Purpose
This playbook is the practical “how do I validate end-to-end quickly?” guide for a firmware engineer.

It assumes:
- You have an IMEI
- You can hit REST endpoints
- You can connect to MQTT/MQTTS

## One-page checklist
1) Bootstrap succeeds: `GET /api/bootstrap?imei=...` + `x-api-key`
2) MQTT connects using `primary_broker.*`
3) Telemetry publishes on `primary_broker.publish_topics[0]`
4) Command arrives on `primary_broker.subscribe_topics[0]`
5) Device publishes `ondemand_rsp` with matching `correlation_id`
6) (Gateway mode) Device forwards `forwarded_data` with `origin_node_id`

## Local environment: choose a mode
### Mode A — Integration bring-up (HTTP + MQTT)
- Compose: `unified-go/docker-compose.integration.yml`
- REST base: `http://localhost:8081`
- MQTT base: `mqtt://localhost:1884`

### Mode B — Prod-like (HTTPS + MQTTS)
- Compose: `unified-go/docker-compose.yml`
- REST base: `https://localhost`
- MQTTS base: `mqtts://localhost:8883`

Notes:
- The broker endpoints returned in REST responses can advertise one or multiple endpoints.
- `MQTT_PUBLIC_URLS=mqtts://<host>:8883,mqtt://<host>:1884` is useful during phased bring-up.

## Bootstrap (required)
Canonical:
```bash
curl -i -X GET "http://localhost:8081/api/bootstrap?imei=<IMEI>" \
  -H "x-api-key: <device-api-key>"
```

Bootstrap response fields you must use:
- `primary_broker.endpoints[]`
- `primary_broker.username`
- `primary_broker.password`
- `primary_broker.client_id`
- `primary_broker.publish_topics[]`
- `primary_broker.subscribe_topics[]`

## MQTT topics you should expect
- Telemetry publish: `<imei>/{heartbeat|pump|data|daq}`
- Command downlink + response: `<imei>/ondemand`

## Command contract (server -> device)
The server publishes JSON on the command downlink topic.

Minimum keys to implement on device:
- `packet_type=ondemand_cmd`
- `msgid` (same as `correlation_id`)
- `correlation_id`
- `command.name` (preferred)
- `command.params` (preferred)

Legacy compatibility keys (may also be present):
- `cmd` (same semantic as `command.name`)
- `payload` (same semantic as `command.params`)

Default commands (seeded as core):
- `reboot`
- `rebootstrap`
- `set_ping_interval_sec` params: `{ "interval_sec": int }`
- `send_immediate`

## Response contract (device -> server)
Device should publish JSON response that includes:
- `packet_type=ondemand_rsp`
- `correlation_id` (or `msgid`) that matches the command
- `code` (recommended)

Codes:
- `0` ack / accepted
- `1` failed
- `2` wait (used by `send_immediate` when next periodic publish is ≤30s away)

## Gateway forwarding: getting attached nodes
Firmware can fetch the list of attached child nodes for a gateway:
```bash
curl -i -X GET "http://localhost:8081/api/device-open/nodes?imei=<GATEWAY_IMEI>"
```

## Gateway forwarding: forwarded payload requirements
For forwarded telemetry payloads (`packet_type=forwarded_data` or `metadata.forwarded=true`):
- `metadata.forwarded=true`
- origin identity: `metadata.origin_node_id` (preferred) **or** `metadata.origin_imei`
- `metadata.route.path` non-empty array
- `metadata.route.hops` non-negative integer
- `metadata.route.ingress` non-empty string

## What server-side artifacts help firmware most (and where)
- Endpoints + examples: `09-device-api-samples.md`
- MQTT contract: `03-mqtt-topics-and-payloads.md`
- Lifecycle overview: `05-lifecycle-flows.md`
- Pseudocode templates: `07-firmware-pseudocode.md`
- Troubleshooting: `12-firmware-troubleshooting.md`
- Diagrams: `docs/firmware-integration/diagrams/*.svg`
