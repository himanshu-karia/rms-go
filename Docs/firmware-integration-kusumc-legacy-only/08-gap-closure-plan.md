# Gap Closure Plan (to reach stable 95%+ parity)

NOTE: This document is **platform execution planning** and is **not required** for legacy-only firmware development.
Firmware developers should use `for-firmware-agent/`.

## What changed since last refresh (2026-02-18)
- Updated priority framing to reflect that query alias compatibility hardening is now broadly in place.
- Retained legacy-only strategy for MQTT topic continuity.
- Kept parity verification and rollout controls as active remaining execution tracks.
- Added dedicated device API sample catalog and command-history fallback to contract-lock guidance.

## Priority 1: Lock firmware-facing contract
- Freeze canonical MQTT envelope fields and command correlation requirements.
- Publish strict list of allowed metadata forwarding keys.
- Keep legacy topic support as the stable default.
- Keep device-side REST examples up to date in `09-device-api-samples.md` (including open-device command-history fallback).

## Priority 2: CSV/API contract parity
- Align import request contract behavior with legacy expectations (or add compatibility adapters).
- Decide and implement government-credential import job parity strategy.

## Priority 3: Verification harness
- Add parity tests that replay legacy sample traffic into Go stack.
- Validate command lifecycle with ack/resp correlation and retries.
- Validate forwarded telemetry from child nodes with routing metadata.

## Priority 4: Rollout and deprecation controls
- Mark compatibility aliases as required/optional.
- Add deprecation timeline only after production metrics confirm safe migration.
