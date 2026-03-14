# Mandatory Seed Contract (RMS-Go KUSUMC)

This page defines the **minimum data contract** required for `rms-go/go-kusumc` to operate on a fresh database.

## 1) Runtime model (what this product is)

`rms-go` is a **productized hybrid** runtime:
- **Declared/fixed rails**: database schema, core MQTT topic profile, core HTTP surface.
- **Bounded runtime config**: project/business records and optional Project DNA records.

It is **not** platform-level dynamic schema generation (no runtime table/column creation).

## 2) What is fixed vs runtime-defined

### 2.1 Fixed (code + SQL)
- DB schema and indexes are migration-defined (`go-kusumc/schemas/*.sql`).
- MQTT ingest profile is fixed by code/env (`strict_data_only` default topic profile).
- Core command/catalog and auth/role rails are predefined.

### 2.2 Runtime-defined records
- Org/project/protocol/vendor/device records.
- Credential history and provisioning job rows.
- Optional Project DNA rows (payload schemas, sensor specs, thresholds).

## 3) Minimum required seed/provisioning data (must exist)

For first real device onboarding and telemetry processing, ensure these records exist (via bootstrap SQL, APIs, or UI flows):

1. **Identity + access baseline**
   - At least one admin user.
   - Capability/role records and user-role/capability bindings.
   - At least one API key/JWT path usable by operators and integrations.

2. **Master hierarchy baseline**
   - Required hierarchy rows used by enrollment forms and validation:
     - states / authorities
     - vendor/org references
     - protocol-version references (when used by the selected workflow)

3. **Project and protocol binding**
   - A project row.
   - At least one protocol configuration for that project (broker host/port + pub/sub topic templates).

4. **Device and credential baseline**
   - Device row mapped to project.
   - Active local/government credential bundle and provisioning state.

5. **Command baseline**
   - Core command catalog entries (or equivalent project-scoped command set).

Without these five groups, production workflow will be incomplete even if the server boots.

## 4) Project DNA: mandatory or optional?

### 4.1 Strict runtime viability
Project DNA is **not strictly required** for basic packet acceptance and persistence on fixed legacy topics.

### 4.2 Operationally recommended
Project DNA is **strongly recommended** for:
- packet schema validation quality,
- sensor transformation behavior,
- threshold/alert semantics,
- predictable verified vs suspicious classification.

In short: system can ingest without DNA, but quality controls become weaker.

## 5) Topic and DB schema dynamism stance

- **DB schema**: declared, migration-owned, non-dynamic at runtime.
- **Topic schema**: fixed legacy profile rails, with controlled compat toggles (env-driven), not arbitrary runtime topic discovery.
- **Business config data**: runtime-seeded/provisioned records are expected and normal.

## 6) Practical go-live seed checklist

Before declaring an environment ready:
- migrations applied,
- admin auth path verified,
- hierarchy/master records present,
- project + protocol configured,
- at least one device credentialed,
- command catalog reachable,
- telemetry packet accepted on `<imei>/heartbeat` and visible in history,
- (recommended) Project DNA uploaded for that project.

## 7) Final interpretation for KUSUMC

`go-kusumc` is the declared/fixed replacement for legacy RMS operation, while keeping a **limited config layer** (records + optional DNA) to avoid hardcoding every operational detail in code.

## 8) Exact seeded sample values

See `MANDATORY-SEED-SAMPLE-KUSUMC.md` for the concrete seed dataset currently authored in SQL bootstrap and runtime seeder code.

For the curated production subset (excluding demo/sample records), see `PRODUCTION-ONLY-SEED-SET-KUSUMC.md`.
