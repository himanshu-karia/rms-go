# Troubleshooting (Legacy-only)

## Quick triage
1. `GET /api/bootstrap?imei=...` works with `x-api-key`.
2. MQTT connect succeeds with bootstrapped credentials.
3. Publish telemetry on `<imei>/heartbeat` succeeds.
4. Subscribe to `<imei>/ondemand` and confirm command reception.

## Common issues
### Bootstrap 401
- Missing/invalid `x-api-key`.

### MQTT auth error
- Credentials rotated; re-bootstrap.

Recommended sequence (legacy-only):
1. Retry connect once.
2. If still rejected:
  - Try `GET /api/device-open/credentials/local?imei=...` (if available), else re-run `GET /api/bootstrap?imei=...`.
3. Select broker endpoint in this order:
  - `credential.endpoints[0].url`
  - or `credential.endpoints[0].protocol + host + port`
  - or bootstrap `primary_broker.endpoints[0]`
4. Reconnect with refreshed `username/password/client_id` and re-subscribe to `<imei>/ondemand`.

![](../diagrams/08-credential-rotation.sequence.svg)

### Commands not received
- Not subscribed to `<imei>/ondemand`.
- Use HTTP fallback:
  - `/api/device-open/commands/status`
  - `/api/device-open/commands/history`
  - `/api/device-open/commands/responses`

![](../diagrams/06-command-recovery-http-fallback.sequence.svg)

### Forwarded telemetry suspicious
- Missing `metadata.origin_node_id`/`origin_imei` or `metadata.route.*`.

![](../diagrams/04-telemetry-forwarded.flowchart.svg)
