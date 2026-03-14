# DNA Operator Playbook

This playbook documents the lifecycle for managing DNA bundles (sensors + thresholds) per project, including safety checks, publishing, rollback, and reverification.

## Roles
- **Editor**: prepares sensor/threshold changes (CSV or UI), drafts versions.
- **Publisher**: reviews, publishes, and triggers reverification.
- **Observer**: monitors metrics/dashboards and alerts.

## Preconditions
- Access to DNAPage in admin UI and API token for DNA endpoints.
- Project ID known (e.g., `pm-kusum-solar-pump-msedcl`).
- Redis and Timescale/DB online; reverify service configured.
- If using DB audit columns, ensure `v6_dna_sensor_versions_audit.sql` applied.

## Edit (draft)
1) Export current sensors (optional): UI "Export CSV" or `GET /api/project-dna/{projectId}/sensors/export`.
2) Edit sensors/thresholds:
   - Sensors: CSV or UI table; keep `param` unique and required.
   - Thresholds: project scope or device overrides; ensure warn_low <= warn_high.
3) Save drafts in UI (does not publish) or prepare CSV.
4) Upload new version (draft): UI "Upload Version" or `POST /api/project-dna/{projectId}/sensors/versions` with CSV.
   - Optionally set `label` (e.g., `Jan02-volt-tune`).

## Preview
- List versions: UI Versions pane or `GET /api/project-dna/{projectId}/sensors/versions`.
- Diff two versions: UI "Diff vs previous" (fetches CSVs) to see added/removed/changed params.
- Fetch CSV for inspection: `GET /api/project-dna/{projectId}/sensors/versions/{versionId}/csv`.
- Validate thresholds source: UI shows `source` (default vs override-merged) when loading.

## Publish
1) Confirm target version ID from list.
2) Publish: UI "Publish" or `POST /api/project-dna/{projectId}/sensors/versions/{versionId}/publish`.
3) Verify publish result: response includes `applied` and `version_id`; UI status updates.
4) Optional: Export sensors post-publish to confirm applied bundle.

## Rollback
- Identify previous stable version ID.
- Rollback: UI "Rollback" or `POST /api/project-dna/{projectId}/sensors/versions/{versionId}/rollback`.
- Confirm sensors after rollback via export.

## Reverify (post-publish/rollback)
- Trigger reverify per project: `POST /api/reverify/{projectId}` (guarded by `REVERIFY_TOKEN` if set).
- Scheduler: ensure `REVERIFY_PROJECTS` and `REVERIFY_INTERVAL_MINUTES` are configured for periodic checks.
- Metrics: `GET /api/metrics` exposes per-project cumulative counters (processed, recovered) and last-run gauges. Add alerts/dashboards (pending).

## Overrides (device-level thresholds)
- Device overrides: UI Device scope or `PUT /api/project-dna/{projectId}/thresholds/{deviceId}` with thresholds array.
- List devices with overrides: `GET /api/project-dna/{projectId}/thresholds/devices`.
- Fallback: project thresholds are used when no device override exists.

## CSV import (sensors)
- Endpoint: `POST /api/project-dna/{projectId}/sensors/import` with CSV file.
- Response includes `imported`, `updated`, and optional `errors`; UI shows these counts.
- CSV columns should include at least `param` and `label`; optional: unit, min, max, resolution, required, notes, topic_template.

## Checks before publishing
- No duplicate `param` values in sensors.
- Threshold rows have `param` and warn_low <= warn_high.
- Device overrides only where needed; avoid conflicting origins unless intentional.
- Version label is meaningful (date + purpose).

## Recovery steps
- If bad publish: rollback to last known-good version, then reverify.
- If thresholds are incorrect for a device: push device override, then reverify.
- If metrics show persistent failures: inspect ingestion logs and revert to prior bundle.

## References
- OpenAPI spec: `docs/openapi-dna.yaml`.
- Frontend: DNAPage uses `@hk/core` DNA client for all operations.
- CLI: `cmd/reverify` triggers reverification.
