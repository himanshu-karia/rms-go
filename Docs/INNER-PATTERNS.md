# Inner Patterns & Hidden Logic

This document captures non-obvious behaviors that matter during maintenance.

## 1) Profile-gated compatibility, not implicit behavior
- `MQTT_TOPIC_PROFILE=strict_data_only` is the intended default.
- Compatibility (`legacy_compat` with `/pump` subscribe path) should be explicit per environment.

Why it matters:
- Prevents accidental broad topic behavior in production.

## 2) Compose-profile-aware URL defaults in scripts
- Integration profile defaults to direct API URL (`http://localhost:8081`).
- Main compose profile defaults to TLS front-door (`https://localhost`).

Why it matters:
- Eliminates false-negative E2E failures caused by wrong base URL assumptions.

## 3) Host fallback in test runners is intentional
When the runtime container lacks Go toolchain, runner falls back to host `go test` and seeds required fixtures.

Why it matters:
- Prevents brittle coupling between runtime image composition and test execution.

## 4) Dead-letter queue as reliability boundary
Overflow events are considered recoverable workload, not terminal drops:
- write to dead-letter queue
- record counters
- replay automatically/manual

Why it matters:
- Supports operational recovery without silent data loss patterns.

## 5) Correlation fallback for legacy ondemand responses
Legacy devices may omit response correlation fields; system attempts best-effort matching to latest outstanding request per device.

Why it matters:
- Works with legacy firmware but introduces ambiguity under high command concurrency.
- Operational guidance: avoid multiple simultaneous outstanding commands per same device where possible.

## 6) Canonical vs archival docs discipline
- Active implementation guidance must live in canonical docs.
- Planning-era docs are archived under `Docs/reports/archive/`.

Why it matters:
- Reduces onboarding drift and contradictory instructions.
