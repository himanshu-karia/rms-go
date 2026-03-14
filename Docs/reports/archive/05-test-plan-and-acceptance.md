# Test plan & acceptance criteria (KUSUMC)

## Goal
Prove that `go-kusumc + ui-kusumc + firmware(govt protocol)` is internally consistent and production-safe on `kusumc.hkbase.in`.

## Test layers

### 1) Backend unit tests
- `go test ./...`

### 2) Backend integration tests (recommended additions for KUSUMC)
You will likely need a KUSUMC-specific E2E suite that validates legacy topics:
- publish `<imei>/heartbeat` → persisted
- issue command → publish to `<imei>/ondemand` → response correlated
- rotate creds → broker disconnect → reconnect works

### 3) UI unit tests
- run the ui-kusumc test suite (Vitest)

### 4) UI E2E smoke
- login
- device list
- command send + status visible

## Acceptance criteria

### A) URL correctness
- Credential/bootstrap responses advertise `mqtts://kusumc.hkbase.in:8883`.

### B) MQTT legacy telemetry
- Device publishes to `<imei>/heartbeat` and it appears in telemetry history.

### C) Commands
- UI issues command → go-kusumc publishes `<imei>/ondemand` with `{msgid, cmd, payload}`.
- Device responds on `<imei>/ondemand` and server updates status.

### D) Resilience
- Broker restart does not permanently break ingest.
- Credential rotation forces disconnect and device reconnects with new creds.

### E) No platform leakage
- No dependency on channels topics.
- No requirement for multi-project routing metadata.
