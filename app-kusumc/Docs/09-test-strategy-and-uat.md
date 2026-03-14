# Test Strategy and UAT Plan (App + Server)

Date: 2026-03-04

## 1) Test layers

1. Unit tests
- Android: transport adapters, queue state machine, idempotency key generation
- Go: auth scope checks, ingest reason-code logic, bridge session lifecycle

2. Integration tests
- App ↔ API contract tests against staging
- Bridge session + packet ingest batch flow
- Command submit/result correlation flow

3. End-to-end tests
- Real device or hardware simulator
- BLE/BT/WiFi local extraction + upload + reconciliation

4. Security tests
- Token replay attempts
- Unauthorized project/device access
- Phone revocation and immediate denial

## 2) Core scenarios

- S1: Happy path sync (1k packets)
- S2: Large sync (10k/50k packets)
- S3: Mid-sync app crash and resume
- S4: Duplicate replay from device memory
- S5: Mixed valid/invalid packet batch
- S6: Offline command and delayed result upload
- S7: Session expiry during transfer

## 3) KPI targets

- Packet acceptance >= 99.5% on valid payloads
- Duplicate false-positive rate < 0.1%
- App crash recovery data loss = 0 packets
- Median command roundtrip upload latency < 3s (excluding local device execution)

## 4) UAT script

- Step 1: Enroll phone and login
- Step 2: Connect to assigned RMS device via BLE or WiFi
- Step 3: Run command read/write and verify server command timeline
- Step 4: Pull historical logs and sync
- Step 5: Validate reconciliation summary and rejected reason details
- Step 6: Revoke phone and confirm access denial

## 4.1 QA-04 field UAT pass/fail rubric

| Area | Check | Pass condition |
|---|---|---|
| Mobile Auth | OTP request+verify | Token issued, assignments returned |
| User Role APIs | Create device, assign beneficiary, installation | API success + DB records visible |
| Device Bootstrap | Local credentials fetch | Valid broker endpoint + creds |
| MQTT cycle | Connect, publish, subscribe, response loop | Pub/sub and ack loop observed |
| Persistence | Telemetry + command timeline | Records available via APIs and DB checks |
| Cred rotation | Old creds rejected after rotate | Device reconnect fails with old creds |
| Recovery | Re-bootstrap with new creds | Reconnect and traffic resume |
| Mobile ingest | Idempotent replay behavior | Duplicate response replayed safely |

Fail rules:
- Any P0 row above failing is an overall UAT fail.
- Any data mismatch between API and DB persistence is an overall UAT fail.

## 4.2 Internal automation mechanism (ADB + long E2E)

Automation entrypoints:
- `app-kusumc/scripts/long-automated-mobile-auth-to-persistence.ps1`
- `app-kusumc/scripts/mobile-adb-smoke.ps1`

Run commands:

```powershell
cd rms-go/app-kusumc
./scripts/long-automated-mobile-auth-to-persistence.ps1 -ServerBaseUrl https://rms-iot.local:7443 -InstallDebug -RunAdbSmoke
```

What this run covers:
- backend stack refresh from source
- user-role + device-role integration tests (`TestRMSMegaFlow`, `TestDeviceCommandLifecycle`, `TestKusumFullCycle`)
- mobile bridge tests (`TestMobileIngest_IdempotencyReplay`, `TestMobileCommandStatus_Mapping`)
- Android debug build/test readiness
- optional ADB install and app launch smoke/log capture

Sequence reference:
- `Docs/13-internal-test-automation-and-sequence.md`

## 5) Entry/exit criteria

Entry
- All contract endpoints available in staging
- Test data fixtures seeded

Exit
- All P0/P1 scenarios pass
- No open critical defects
- UAT sign-off by product + operations
