# Target architecture (KUSUMC)

## Components

### 1) ui-kusumc (frontend)
- Single-purpose RMS admin/operator UI for the KUSUMC product line.
- Talks to go-kusumc over HTTPS at:
  - `https://kusumc.hkbase.in/api/...`

### 2) go-kusumc (backend)
- Derived from `unified-go` codebase.
- Runs behind Nginx TLS termination using hkbase.in wildcard cert.
- Core responsibilities:
  - REST APIs for admin UI and device-open flows.
  - MQTT worker: ingest telemetry and correlate on-demand responses.
  - MQTT provisioning: create/update EMQX ACLs and credentials for devices.

### 3) EMQX broker
- Terminates MQTT and MQTTS.
- Enforces per-device ACL.
- Topic model for this product line is legacy/government:
  - Publish: `<imei>/{heartbeat,pump,data,daq}`
  - Command channel: `<imei>/ondemand`

### 4) Database
- Postgres/Timescale for persistence.
- Redis optional for hot cache.

## Wire contracts

### REST (high-level)
Must cover:
- Auth/login for UI
- Device CRUD and lookup
- Device credentials rotation/retry
- Device-open:
  - credentials local/government
  - VFD lookup
  - installation/beneficiary
- Commands:
  - issue, history, ack
- Telemetry:
  - ingest endpoints used by tooling
  - history/latest

### MQTT (frozen government protocol)
Source references in repo:
- `refer-rms-deploy/RMS JSON MQTT Topics MDs/*`

Telemetry topics:
- `<imei>/heartbeat`
- `<imei>/pump`
- `<imei>/data`
- `<imei>/daq`

Command/response topic:
- `<imei>/ondemand`

Packet typing:
- suffix decides packet family.

Correlation:
- legacy uses `msgid` and may have mixed casing (`MSGID`, etc.).

## URL/public endpoint configuration

KUSUMC must advertise broker endpoints that match the deployment domain.

Backend → device credential bundle should advertise:
- `mqtts://kusumc.hkbase.in:8883` (required)
- optionally `mqtt://kusumc.hkbase.in:1883/1884` (only if you allow plaintext)

UI web simulator (if used) typically needs a WS/WSS endpoint:
- `wss://kusumc.hkbase.in/mqtt`

## Separation from platform
- KUSUMC does not use `channels/{project_id}/...` topics.
- KUSUMC does not rely on multi-project forwarding/routing metadata.
