# Fixing Plan – 26-12-2025

_Reference scope: learn from `rms-deploy` without modifying it, and apply the lessons to `unified-go` so the PM-KUSUM Solar Pump project can be onboarded alongside future multi-project workloads._

---

## Phase 1 – Master Data Shape (Add the missing bits)

1. **Organisational hierarchy mapping**
   - Model RMS-specific actors (State → State Authority → Contractor → Vendor → RMS Manufacturer) inside `organizations` with `type` tags (`state`, `authority`, `contractor`, `vendor`, `manufacturer`).
   - Use `device_links` and `link_roles` to capture many-to-many relationships (e.g., contractor ↔ vendor ↔ RMS manufacturer).
   - Deliverable: a migration/seed script that ingests RMS hierarchy data and populates the new schema without breaking other projects.

2. **Beneficiary / installation graph**
   - Ensure `beneficiaries` and `installations` tables are filled for PM-KUSUM devices so the legacy “Beneficiary → Installation → Device” chain is preserved.
   - Map RMS device identifiers to the `devices` table (one row per IMEI) and attach installation metadata in `devices.attributes`.

3. **Protocol / VFD configuration capture**
   - For each third-party VFD manufacturer/model referenced in RMS, insert a `device_profiles` row with RS485, Modbus address map, parameter units, and resolution metadata.
   - Link protocols to projects/state authorities via a dedicated table or JSON config in `projects.config` to keep per-project protocol wiring identifiable.

4. **Bridging old to new**
   - Define an import routine that reads RMS Mongo collections (or CSV export) and writes to the new Postgres schema, enabling co-existence while we retire the old stack later.

---

## Phase 2 – Payload DNA & Verification (CSV-first approach)

1. **Packet taxonomy CSV (dynamic input)**
   - Extend `docs/rms-payload-parameters.csv` into a richer CSV (or multiple CSVs) with the following columns:
     - `packet_type` (heartbeat, pump, daq, ondemand_command, ondemand_response …)
     - `expected_for` (global | project:<id> | device:<imei> | device_profile:<id>)
     - `key`
     - `description`
     - `unit`
     - `required` (Y/N)
     - `max_length` (optional length constraint)
     - `notes`
   - Goal: the CSV becomes the single source for expected keys across projects/devices.

2. **Verification driven by config (not hardcoded)**
   - Treat the CSV as an authoring artefact: import it into `projects.config` or a dedicated admin table so the runtime still reads expectations from dynamic project DNA.
   - Compute both `missing_keys` and `unknown_keys` per packet type; persist the diagnostics (e.g., in telemetry metadata) without rejecting packets that future DNA may allow.
   - Allow “extra” keys to be stored in Timescale but enforce per-project limits via config (e.g., `config.payloadSchemas[*].unknownKeyMaxBytes`) to keep JSON sane while staying flexible.

3. **Sensor key consistency**
   - Resolve the current `param` vs `id` mismatch by selecting a canonical column (recommended: keep `param` as the external key) and updating the transformer + verifier + rules engine to use it across the board.

4. **Project-level settings**
   - Extend `projects.config` (or an associated config table) to reference the CSV-derived schema (e.g., `config.payloadSchemas[packet_type]` with pointers to expected keys and length limits).
   - Provide admin CLI/API to refresh the config from CSV/administrative UI so the ingestion pipeline always honours the latest DNA without code deploys.

---

## Phase 3 – MQTT Payload Planning (CSV-driven topics & envelopes)

1. **Topic taxonomy definition**
   - In the CSV, add columns to describe the topic template per packet type (e.g., `topic_template = channels/{project_id}/messages/{imei}` or `<IMEI>/data`).
   - Decide whether PM-KUSUM keeps `<IMEI>/<suffix>` while new projects use `channels/...`, or whether we introduce a universal `type` field with `channels/...` topics. Document the decision in the CSV.

2. **Provisioning ACL alignment**
   - From the same CSV, generate the allow-list of publish/subscribe topics for each device identity.
   - Update `mqtt_worker.go` to replace ACLs and append a deny-all rule, ensuring the generated ACL matches the topic templates for the packet types a device is expected to use.

3. **Envelope invariants**
   - Record in the CSV the mandatory envelope keys (e.g., `imei`, `project_id`, `device_id`, `msgid`, `timestamp`) plus any project/org scope fields (`org_scope`, `contractor_id`, etc.).
   - Ensure the ingestion pipeline enforces these invariants and indexes the relevant columns in Timescale (additional columns or JSONB indexes as needed).

4. **Timescale cleanup strategy**
   - Use the per-packet-type `max_length` or dedicated unknown-key length setting to cap the size of stored JSON; optionally log or queue clean-up tasks when packets exceed thresholds.

---

### Deliverables Summary

- Migration/seed scripts that map the RMS master data hierarchy into `unified-go` tables.
- A comprehensive CSV (or CSV family) capturing packet types, expected keys, topic templates, and per-project/device overrides.
- Ingestion/verification refactor to consume the CSV, compute missing/unknown keys, allow extra keys within configured limits, and keep Timescale sanitized.
- MQTT provisioning updates driven from the CSV so ACLs and topic usage stay consistent across projects.
- Documentation updates reflecting the above, ensuring PM-KUSUM can be onboarded while keeping the platform multi-project ready.
