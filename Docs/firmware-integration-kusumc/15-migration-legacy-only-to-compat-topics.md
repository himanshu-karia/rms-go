# Migration: Legacy-only MQTT → Compat Topics (Optional)

This guide is for teams that want to migrate firmware or an intermediate bridge from the legacy govt topic taxonomy (`<imei>/{heartbeat,pump,data,daq,ondemand}`) to optional compatibility ingest topics. In go-kusumc ingest, `pump` should be mapped to `data`.

## Default behavior (recommended)
- go-kusumc is **legacy-first**.
- Compatibility subscriptions are **disabled by default** unless `MQTT_COMPAT_TOPICS_ENABLED=true`.

## Enable parallel ingest (server-side)
To allow the server to ingest compatibility topics in parallel (for a broker bridge or phased firmware rollout):
- Set `MQTT_COMPAT_TOPICS_ENABLED=true` on the go-kusumc server.

When enabled, the server can ingest:
- `channels/{project_id}/messages/{imei}`
- `devices/{imei}/telemetry`
- `devices/{imei}/errors`

And it will also accept command responses published on:
- `channels/{project_id}/commands/{imei}/resp`
- `channels/{project_id}/commands/{imei}/ack`

## Payload requirements (compat topics)
To keep verification and device correlation deterministic, ensure the republished payload includes:
- `imei` or `IMEI`
- `project_id` (since compat topics don’t always allow deriving project from topic)
- `msgid`
- `packet_type` (strongly recommended)

For device errors, use:
- `packet_type=device_error` (recommended)
- `error_code` (required)
- `timestamp` (required)
- `error_data` (optional)

## Command flow (unchanged)
For KUSUMC legacy protocol, downlink commands remain:
- Server → device: publish JSON on `<imei>/ondemand`
- Device → server: publish ack/response JSON on `<imei>/ondemand`

## Rollback
- Set `MQTT_COMPAT_TOPICS_ENABLED=false` (or unset it) to return to legacy-only subscriptions.