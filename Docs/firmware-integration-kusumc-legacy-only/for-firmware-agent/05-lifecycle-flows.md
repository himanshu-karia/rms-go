# Lifecycle Flows (Legacy-only)

## 1) Provisioning flow
1. Admin provisions device.
2. Platform provisions MQTT creds and ACLs.
3. Device bootstraps to fetch credentials.
4. Device connects and publishes telemetry.

![](../diagrams/02-credentials-and-connect.flowchart.svg)

## 2) Telemetry flow (self data)
1. Device publishes on `<imei>/{heartbeat,data,daq}`.
2. Backend ingests and persists.

![](../diagrams/03-telemetry-self.flowchart.svg)

## 3) Telemetry flow (forwarded data)
1. Gateway receives child-node data.
2. Gateway republishes on `<gateway_imei>/data` with `metadata.*` forwarding block.

![](../diagrams/04-telemetry-forwarded.flowchart.svg)

## 4) Command flow
1. Server publishes on `<imei>/ondemand` (govt legacy command shape).
2. Device executes and publishes ack/resp on `<imei>/ondemand`.
3. Device can recover via HTTP if MQTT continuity is uncertain.

![](../diagrams/05-commands-roundtrip.sequence.svg)
![](../diagrams/06-command-recovery-http-fallback.sequence.svg)
