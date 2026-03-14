# Device Bridge and Offline Sync Spec

Date: 2026-03-04

## 1) Purpose

Define how Android app reads RMS device data over BLE/BT/WiFi and syncs to server safely and losslessly.

## 2) Source transports

- BLE GATT profile (preferred where available)
- Classic Bluetooth serial profile
- WiFi LAN TCP/UDP/HTTP endpoint exposed by device

Transport abstraction in app:
- `DeviceTransportAdapter` interface
- Common frame model independent of physical transport

## 3) Packet handling model

For every packet pulled from device:
1) Keep exact payload bytes / JSON map
2) Parse only minimal routing metadata (`imei`, packet type/suffix)
3) Generate idempotency key from normalized hash
4) Store in local queue with state machine

Queue states:
- `captured`
- `staged`
- `uploading`
- `accepted`
- `duplicate`
- `rejected`
- `retry_wait`

## 4) Dedup and idempotency

Idempotency key inputs:
- `imei`
- packet timestamp (if present)
- packet body hash
- packet type/suffix

Server must treat repeated keys as duplicates (not hard errors).

## 5) Upload behavior

- Batch size adaptive (start 200 packets)
- Retry with exponential backoff on transient failures
- Partial success support
- Resume from last unsynced cursor after crash/restart

## 6) Command read/write bridge

## 6.1 Read capabilities
- Fetch command catalog from server
- Merge with local device-reported capabilities

## 6.2 Write command flow
1) User selects command
2) App sends command to local device
3) App receives ack/response
4) App reports command result to server with correlation IDs

## 7) Conflict and integrity rules

- If server says device is currently connected directly and command collision policy is strict:
  - app submits as queued command, not immediate dispatch
- If payload schema fails on server:
  - packet remains in rejected list with reason code
  - operator can export rejected packet file for diagnostics

## 8) Sync UX requirements

- Show progress: uploaded/accepted/duplicate/rejected counts
- Show estimated time remaining
- Allow pause/resume
- Show last successful sync checkpoint

## 9) Performance targets

- Sustained upload target: >= 50 packets/sec over mobile network
- Local extraction target: >= 200 packets/sec from device memory (transport dependent)
- 10k packet sync completion target: < 5 minutes on stable 4G/WiFi

## 10) Test matrix

- BLE weak-signal scenarios
- Mid-sync app crash and restart
- Duplicate replay from device
- Server 429 throttling behavior
- Mixed valid/invalid packet batch
