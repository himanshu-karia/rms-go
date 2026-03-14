# Contract Map (Legacy-only)

This is the “1-1-1 mapping” in legacy-only mode.

## Firmware → Broker (MQTT)
- Firmware connects using credentials returned by `GET /api/bootstrap?imei=...`.
- Firmware publishes telemetry on:
  - `<imei>/heartbeat`, `<imei>/data`, `<imei>/daq`
- Telemetry packets follow the full contract in `03-mqtt-topics-and-payloads.md`:
  - fixed envelope (`packet_type`, `project_id`, `protocol_id`, `device_id`, `imei`, `ts`, `msg_id`)
  - packet-specific keys by family (heartbeat, PumpData on data topic, daq)
- Firmware publishes device errors/offline-rule alerts on:
  - `<imei>/errors`
- Firmware subscribes to commands on:
  - `<imei>/ondemand`
- Firmware publishes command ack/response on:
  - `<imei>/ondemand`

## Broker → Backend (Go)
- Backend subscribes to legacy topics and ingests JSON payloads.
- Topic suffix determines lane; `/data` is treated as PumpData (stored as `packet_type=pump`).

## Backend → DB
- Telemetry is persisted into the unified telemetry store (Timescale/Postgres).
- The stored telemetry JSON includes `packet_type` so UI/history filters work.

## Backend → UI
- UI reads device latest/history/alerts/commands via HTTP APIs.
- Firmware does not need to know UI routes; only the device-facing routes in `04-rest-api-contract.md` are relevant.

## Commands (downlink + response)
Downlink (server → device) on `<imei>/ondemand` (govt legacy shape):
```json
{
  "msgid": "<uuid>",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "set_ping_interval_sec",
  "interval_sec": 60
}
```

Uplink response (device → server) on `<imei>/ondemand` (govt legacy response; echo msgid recommended):
```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "code": 0,
  "msgid": "<uuid>"
}
```

Response payload may include command-specific output keys (for example `DO1`, `PRUNST1`, `applied`) while preserving `msgid` correlation.

## Device errors / offline-rule alerts
Device → server on `<imei>/errors`:
```json
{
  "open_id": "<device_uuid>",
  "timestamp": 1760870400456,
  "error_code": "RS485_TIMEOUT",
  "error_data": {"bus": "rs485", "count": 3}
}
```
