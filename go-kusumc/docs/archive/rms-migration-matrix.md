# RMS MQTT Migration Matrix

Comparative view for (a) legacy **refer-rms-deploy**, (b) **unified-go** canonical, and (c) **hybrid legacy-friendly** (bridge/shim) modes.

## 1) APIs (provisioning + credentials)
| Aspect | (a) refer-rms-deploy | (b) unified-go | (c) hybrid legacy-friendly |
| --- | --- | --- | --- |
| Device creation | REST creates device + EMQX user/ACL via Node worker | `POST /api/devices` creates device + EMQX user/ACL via Go worker | Same as (b); optional helper API to emit bridge config
| Credentials | Per-device MQTT username/password (bcrypt) | Per-device MQTT username/password (bcrypt) | Same as (b)
| ACL policy | Allow `<IMEI>/{heartbeat,pump,data,daq,ondemand}` pub; command sub; deny-all fallback | Allow `channels/{project_id}/messages/{imei}` pub; `channels/{project_id}/commands/{imei}` sub; deny-all intended | Combined: allow legacy suffix topics **and** channels topics; deny-all fallback
| Service account | Yes, broad ACL for backend | Yes, `SERVICE_MQTT_*` | Same as (b)

## 2) MQTT connection (broker/ports/TLS)
| Aspect | (a) refer-rms-deploy | (b) unified-go | (c) hybrid legacy-friendly |
| --- | --- | --- | --- |
| Broker host/port | mqtt(s)://host:1883/8883 | mqtt(s)://host:1883/8883 (dev: 1884→1883) | Same as (b)
| TLS | Supported (8883) | Supported; nginx terminates TLS on 8883 | Same as (b)
| Auth | Username/password per device | Username/password per device | Same as (b)

## 3) MQTT topics
| Aspect | (a) refer-rms-deploy | (b) unified-go | (c) hybrid legacy-friendly |
| --- | --- | --- | --- |
| Uplink publish | `<IMEI>/{heartbeat,pump,data,daq,ondemand}` | `channels/{project_id}/messages/{imei}` (single topic) with `packet_type`/`type` in payload | Firmware keeps legacy suffix topics; EMQX rule republishes to `channels/{project_id}/messages/{imei}` adding envelope and `packet_type`
| Downlink/commands | Often `<IMEI>/commands/#` (varied) | `channels/{project_id}/commands/{imei}` | Allow both legacy command topic (if needed) and channels command topic
| Subscriptions backend | `+/{suffix}` | `channels/+/messages/+` (and `devices/+/telemetry` fallback) | Same as (b); bridge feeds channels

## 4) Data payload per publish
| Aspect | (a) refer-rms-deploy | (b) unified-go | (c) hybrid legacy-friendly |
| --- | --- | --- | --- |
| Envelope fields | `imei`, `msgid`, `timestamp`, `protocol_id`; packet type implied by topic suffix | `imei`, `msgid`/`msg_id`, `timestamp`; **must include** `packet_type` (or `type`) and should include `project_id`, `device_id`, `protocol_id` | If firmware keeps legacy shape: bridge injects `packet_type`, `project_id`, `device_id`, `protocol_id`, `msg_id`, `ts`; firmware should still send `imei` and `msgid`
| Body fields | Upper-case telemetry keys (e.g., `RSSI`, `BATT`, `TEMP`, `GPS`, `PUMP_ON`) | Accepts body at top level; strict mode compares keys to project DNA; upper-case OK | Same as (b); bridge does not alter body
| Type discrimination | Topic suffix | Payload `packet_type`/`type` recommended | Topic suffix → bridge sets `packet_type`

## Recommended migration stance
- **Preferred**: move firmware to (b) — publish to `channels/{project_id}/messages/{imei}`, include envelope fields (`packet_type`, `project_id`, `protocol_id`, `device_id`, `imei`, `msgid`, `timestamp`), subscribe to `channels/{project_id}/commands/{imei}`.
- **Hybrid feasibility**: (c) is viable short-term using an EMQX rule to republish legacy `<IMEI>/<suffix>` into the channels topic with injected envelope; ACLs must allow both legacy and channels topics; backend ingest stays on channels.
- **Firmware changes needed**: if adopting (b), update broker URL/port, topic, and add `packet_type` + envelope fields. If staying on legacy topics temporarily, ensure payload still carries `imei` + `msgid`, and coordinate the EMQX bridge to add `project_id/device_id/packet_type`.

## Quick examples
- Direct (b) publish topic: `channels/pm-kusum-solar-pump-msedcl/messages/356000000000001`
```json
{
  "packet_type": "heartbeat",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "device_id": "<uuid-from-provisioning>",
  "imei": "356000000000001",
  "msgid": "hb-1735580001",
  "timestamp": 1735580001,
  "RSSI": -67,
  "BATT": 12.4,
  "TEMP": 31.2,
  "PUMP_ON": 1,
  "GPS": "0,0"
}
```

- Legacy (a) topic with bridge to (b): publish to `356000000000001/heartbeat` body:
```json
{
  "imei": "356000000000001",
  "msgid": "hb-1735580001",
  "timestamp": 1735580001,
  "RSSI": -67,
  "BATT": 12.4,
  "TEMP": 31.2,
  "PUMP_ON": 1,
  "GPS": "0,0"
}
```
Bridge republish adds envelope and forwards to `channels/{project_id}/messages/{imei}`.

## Decision note
- You can keep firmware unchanged **only if** EMQX bridge + ACLs are configured to map legacy topics into channels with proper envelope injection. Otherwise, plan a firmware update to the (b) contract for long-term simplicity and strict verification.

## Govt broker add-on (dual publish)
- Intent: device publishes to both our broker and a govt broker using govt-issued creds. We **do not** create or sync those creds to EMQX; we only store and return them.
- Set flow (UI→backend→DB): project-scoped endpoint to store govt broker profile (protocol, host, port, publish/subscribe topics) and per-device creds (client_id, username, password) captured at provisioning or via bulk CSV.
- Get flow (bootstrap/API): device bootstrap response returns two blocks: `primary_broker` (our EMQX: creds + channels topics) and `govt_broker` (their creds/topics). If govt not configured, omit or return null.
- Firmware expectation: connect to both; publish same payload to both brokers; commands still come from our broker channels topic unless govt requires otherwise.
- Backward compatibility: this keeps firmware change minimal—just handle dual broker config—while the main contract stays on channels topics with explicit envelope fields.

## Protocol entity + bootstrap (proposal)
- Protocol profile (project → server vendor → protocol): `{protocol: mqtt|mqtts|https, host, port, publish_topics: [...], subscribe_topics: [...]}`. Devices reference a protocol_id.
- Provisioning inputs: IMEI, device name, project, vendor, `protocol_id` (primary), optional `govt_protocol_id`, optional per-device govt creds (`client_id`, `username`, `password`).
- Bulk CSV example columns: `imei,name,protocol_id,govt_protocol_id,govt_client_id,govt_username,govt_password`.
- APIs: `POST /api/projects/{id}/protocols` to define profiles; `POST /api/devices/{id}/govt-creds` (or bulk) to attach per-device govt creds + `govt_protocol_id`; `GET /api/devices/{id}/bootstrap` to return both broker blocks.
- Bootstrap payload: `primary_broker` (protocol/host/port/topics, creds, required envelope fields) + optional `govt_broker` (protocol/host/port/topics from govt profile, per-device creds). No EMQX sync for govt.

## Legacy RMS hierarchy (Mongo) — key relationships
- Collections (from refer-rms indexes/seeds): `states` → `state_authorities` → `projects`; `server_vendors`, `solar_pump_vendors`, `vfd_drive_manufacturers`, `rms_manufacturers`; `protocol_versions` keyed by `{stateId, authorityId, projectId, serverVendorId, version}` with topic suffix metadata; `devices` referencing `protocolVersionId`; `installations` linking device ↔ project/state/authority; `beneficiaries` and `installation_beneficiaries`; govt creds stored per device in `government_credentials` with protocolSelector snapshot (state/authority/project/serverVendor/protocolVersion) and endpoints/topics.
- Seed hierarchy (bootstrap.service): creates State → StateAuthority → Project → ServerVendor → ProtocolVersion; vendors are separate collections. ProtocolVersion carries topics (`<IMEI>/heartbeat`, etc.) and publish/subscribe suffix metadata.
- Operational linkage: device imports and govt credential imports carry protocolSelector to bind credentials to the correct scope (state/authority/project/vendor/protocol version).
