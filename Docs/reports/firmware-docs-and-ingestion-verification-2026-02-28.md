# Firmware + Ingestion Verification Report (2026-02-28)

## Objective
Validate end-to-end readiness for:
1. Legacy topics with `MQTT_COMPAT_TOPICS_ENABLED=false`
   - Govt-only payload
   - Minimal payload
   - Full-envelope payload
2. Compact topic families with `MQTT_COMPAT_TOPICS_ENABLED=true`
   - `channels/{project}/messages/{imei}` and `.../{suffix}`
   - `devices/{imei}/telemetry` and `.../{suffix}`

## Runtime verification (code)

### Topic subscriptions
- Legacy subscriptions are always present in `internal/adapters/primary/mqtt_handler.go`:
  - `+/heartbeat`, `+/data`, `+/daq`, `+/ondemand`, `+/errors`
- Compat subscriptions are toggled by `MQTT_COMPAT_TOPICS_ENABLED` and now include subtopics:
  - `channels/+/messages/+`
  - `channels/+/messages/+/+`
  - `devices/+/telemetry`
  - `devices/+/telemetry/+`
  - `devices/+/errors`
  - `devices/+/errors/+`

### Ingestion identity and packet handling
- Identity normalization and inference from legacy/channels/devices topics exists in `internal/core/services/ingestion_service.go`.
- Missing `device_id`/`project_id` is resolved by IMEI lookup (`DeviceService.ResolveDeviceIdentity`) with Redis identity cache.
- `packet_type` is inferred from topic when not present, including compact suffix topics:
  - `channels/{project}/messages/{imei}/{suffix}`
  - `devices/{imei}/telemetry/{suffix}`
- Event time fallback works via `ts/timestamp/TIMESTAMP/time`, then server time.

## Executed tests

### Added tests
1. `internal/core/services/tests/ingestion_test.go`
   - `TestProcessPacket_LegacyTopics_GovtMinimalFull_AllPersist`
   - Sends three packet forms on legacy topics and verifies persistence envelope:
     - govt-only heartbeat
     - minimal data
     - full-envelope daq
   - Verifies normalized packet type + resolved identity.

2. `internal/core/services/ingestion_service_test.go`
   - Extended `TestDetectPacketType`
   - Verifies compact subtopic inference:
     - `channels/.../data` -> `pump`
     - `devices/.../telemetry/daq` -> `daq`

### Test runs (executed)
- `TestProcessPacket_LegacyTopics_GovtMinimalFull_AllPersist`: passed (with subtests)
- `TestDetectPacketType`: passed
- Summary from targeted runs: passed=4, failed=0

## Documentation verification

### Folder A: `Docs/firmware-integration-kusumc-legacy-only`
- Contains MQTT topics/payload contract, REST contract, lifecycle flows, pseudocode, troubleshooting, and test vectors.
- Legacy telemetry topic contract is explicit (`<imei>/{heartbeat,data,daq}`), including PumpData on `/data`.
- API surfaces and examples are present (bootstrap, credentials, command history/status/responses, errors).
- Added scope cross-link to compact migration docs in:
  - `for-firmware-agent/00-index.md`

### Folder B: `Docs/firmware-old-to-new-legacy`
- Contains explicit migration model for:
  - Govt Smallest
  - Minimal
  - Full Envelope
- Includes per-topic examples for heartbeat/data/daq.
- Includes compact topic examples (`channels/.../{suffix}`, `devices/.../{suffix}`) and suffix normalization table.
- Includes compulsory vs recommended field policy and operational outcomes.

## Consistency status
- Legacy mode (`MQTT_COMPAT_TOPICS_ENABLED=false`): **consistent and verified in tests** for govt/minimal/full payload ingestion path.
- Compat mode (`MQTT_COMPAT_TOPICS_ENABLED=true`): **supported and documented**, with compact subtopic inference test coverage.

## Notes / boundaries
- The executed tests are targeted ingestion-level tests (service path + persistence envelope assertion) and validate backend processing logic deterministically.
- Full infrastructure live-broker/live-database soak validation is separate from this report and can be run via integration/e2e pipelines if required.
