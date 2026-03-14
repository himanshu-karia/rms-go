# RMS Lifecycle Flows

## What changed since last refresh (2026-02-18)
- Reconfirmed lifecycle descriptions against the current legacy-only ingest/command flow.
- Kept forwarded telemetry lifecycle step aligned with route-metadata expectations.
- Retained command correlation and alert/rules sequencing as firmware-facing baseline.
- Added explicit command-recovery fallback via open-device command history endpoint.
- Added explicit command-recovery fallback via open-device command history/responses/status endpoints.

## 1) Provisioning flow
1. Admin creates device and assigns hierarchy/protocol.
2. Platform provisions MQTT credentials and topic ACLs.
3. Device retrieves bootstrap/open credentials.
4. Device connects to broker and starts telemetry publish.

Diagram:

![](diagrams/02-credentials-and-connect.flowchart.svg)

## 2) Telemetry flow (self data)
1. Device publishes telemetry envelope with `imei`, `project_id`, `msgid`, `packet_type`.
2. Broker authorizes publish and forwards to Go subscriber.
3. Ingestion service transforms and validates payload.
4. Packet persists to telemetry store; verified packets also update hot cache and shadow.
5. Rules engine evaluates verified payloads and can trigger actions/alerts.

## 3) Telemetry flow (forwarded data from other nodes)
1. Gateway node receives child-node data.
2. Gateway republishes on the legacy uplink topic (typically `<gateway_imei>/data`) and includes forwarding metadata when applicable.
3. Ingestion accepts payload as telemetry event.
4. Analytics/rules can still operate if sensor keys remain top-level and schema allows forwarding metadata.

## 4) Command flow
1. Operator/API issues command.
2. Server publishes to `<imei>/ondemand`.
3. Device executes and emits ack/response payload with correlation reference.
4. Ingestion links response to command request and updates status.
5. If MQTT session is interrupted, device can recover command state via:
	- `GET /api/device-open/commands/history?imei={imei}&limit={n}`
	- `GET /api/device-open/commands/responses?imei={imei}&limit={n}`
	- `GET /api/device-open/commands/status?imei={imei}`
	(preferred prefix is `/api/device-open/*`; aliases under `/api/devices/open/*` and `/api/v1/*` remain supported).

Diagram:

![](diagrams/05-commands-roundtrip.sequence.svg)

## 4.1) Device configuration flow (specialized command)
1. Operator/UI queues device configuration via HTTP.
2. Server persists a configuration record and publishes command `apply_device_configuration` on `<imei>/ondemand`.
3. Device applies configuration and publishes ack/resp (recommended: echo command `msgid`; if supported also include `correlation_id=config_id`).
4. Ingestion finalizes the configuration record based on response status (acknowledged/failed).

Diagram:

![](diagrams/12-device-configuration-apply.sequence.svg)

## 5) Alerts/rules flow
1. Verified telemetry enters rules evaluation.
2. Matching rules emit actions (alert/mqtt command based on configured action type).
3. Alert events and command correlation are persisted for audit/operations.
