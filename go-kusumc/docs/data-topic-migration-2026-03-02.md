# Canonical telemetry topic/type contract

## Scope
This document defines the active telemetry topic/type contract used by go-kusumc.

## Contract
- Canonical telemetry data type: `data`
- Accepted telemetry suffix families:
  - `<imei>/data`
  - `channels/{project}/messages/{imei}/data`
  - `devices/{imei}/telemetry/data`
- Non-canonical data aliases are rejected.

## Ingestion behavior
- Packet type normalization accepts canonical `data`.
- Telemetry parsing requires envelope integrity (`imei`, project context when configured).
- Topic/type validation enforces canonical suffix and packet type mapping.

## MQTT subscription baseline
Default server subscriptions:
- `+/heartbeat`
- `+/data`
- `+/daq`
- `+/ondemand`
- `+/errors`

## Configuration baseline
- Environment defaults use canonical telemetry suffixes.
- Runtime configuration does not include topic-profile switching for non-canonical telemetry aliases.

## Operator requirements
- Firmware publishes telemetry payloads to canonical `.../data` topics.
- Provisioning templates and ACL rules authorize canonical runtime topics.
- Monitoring and troubleshooting should treat non-canonical telemetry aliases as contract violations.
