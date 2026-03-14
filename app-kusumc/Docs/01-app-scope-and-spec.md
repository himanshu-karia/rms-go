# App Scope of Work and Product Spec (Android)

Date: 2026-03-04

## 1) Objective

Build an Android app for RMS field operations that can:
- Authenticate users
- Pair/connect to RMS devices over BLE/BT/WiFi LAN
- Read and write commands locally
- Fetch offline logs from device memory
- Push those logs to server using app identity while preserving original device payload
- Provide remote viewer (telemetry/health) and simulator access

## 2) In-scope features

## 2.1 Authentication and phone onboarding
- User login (same org/project RBAC model as web)
- Add/enroll phone as a managed client device
- Bind phone to user and allowed projects/devices
- Token lifecycle: login token, refresh token, local secure storage

## 2.2 Device connectivity (field mode)
- BLE scan/connect
- Classic Bluetooth fallback (where BLE not available)
- WiFi direct/LAN socket mode
- Connection diagnostics (RSSI, MTU/throughput, retries)

## 2.3 Device command operations
- Read command dictionary/capabilities from server
- Send command to local device (online local path)
- Read response/ack from local device
- Queue retry for intermittent links

## 2.4 Offline log extraction
- Pull historical packets/logs from RMS device memory
- Preserve source packet exactly (no schema mutation)
- Tag each pulled packet with local metadata (phone ID, pull session ID, timestamp)

## 2.5 Uplink to server (on behalf of device)
- Upload logs via HTTPS batch or WSS stream
- Optional MQTT mode only if explicitly provisioned
- Server performs dedup + verification + audit attribution (device + phone + user)

## 2.6 Remote viewer (already requested)
- Device list and status
- Latest telemetry snapshots
- Recent command history/status
- Offline sync job status

## 2.7 Simulator (already requested)
- Trigger/launch simulator session
- Send test commands
- View simulated responses

## 3) Out of scope (Phase 1)

- Full admin console replacement
- Complex data analytics visualization parity with full web
- Firmware update binaries over phone (unless explicitly added as Phase 2)

## 4) User roles and permissions

- Field Operator
  - Pair/connect device, read/write commands, extract logs, push sync
- Supervisor
  - Field Operator rights + remote viewer history + reconciliation reports
- Admin
  - Phone enrollment approval, credential assignment policy, audit review

## 5) Core user journeys

1) Login → select project/device → connect via BLE/BT/WiFi → read live status
2) Trigger command locally → capture ack/response → sync command result to server
3) Pull offline logs from device → run local validation → upload → server confirms ingest count
4) View sync report (accepted/rejected/duplicate packets)

## 6) Non-functional requirements

- Offline-first local queue (at least 50k packets buffered)
- At-least-once upload with idempotency key per packet
- End-to-end encryption in transit
- Secure local secrets storage (Android Keystore)
- Crash-safe sync resume
- Battery-aware background sync policy

## 7) Acceptance criteria (Phase 1)

- Can onboard/login and enroll phone successfully
- Can connect to at least one device transport (BLE or WiFi) with retry
- Can issue command and receive response locally
- Can extract >=10k packets and sync with >=99.5% successful acceptance
- Can show rejected/duplicate reason breakdown from server
- Full audit trail: who uploaded what, on behalf of which device

## 8) Delivery phases

- Phase A (4-6 weeks): login, enrollment, remote viewer, base device connection
- Phase B (4-6 weeks): command read/write + local queue
- Phase C (4-6 weeks): offline log pull + robust sync + reconciliation UX
- Phase D (2-4 weeks): hardening, security, observability, UAT
