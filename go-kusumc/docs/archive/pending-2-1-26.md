# Pending DNA Work (2026-01-02)

## Must-Do Items (not implemented yet)
- (None)

## Nice-to-Haves / Polish
- (None)

## Done This Cycle
- OpenAPI + typed SDK: DNA spec captured in `docs/openapi-dna.yaml`; generated types and typed client wired into `@hk/core` and DNAPage CSV import now uses the client.
- Operator playbook: documented DNA lifecycle in `docs/operator-playbook-dna.md` (edit, preview, publish, rollback, reverify, CSV import, checks, recovery).
- Feature-flag consumers: ingestion/rules/automation now read `config:bundle:{projectId}` when available.
- Reverify alerting: added Prometheus rules (`infra/monitor/alerts.reverify.yml`) and dashboard stub (`infra/monitor/grafana-reverify-dashboard.json`); Prom config now loads alert rules.
- Migration rollout: applied audit columns to `dna_sensor_versions` (created_by/published_by/rolled_back_by) via SQL patch.
- Rules/Automation UI polish: DNAPage now scopes Rules CTA with projectId query and shows automation graph size with link to builder.
- Thresholds UX/validation: device suggestions wired to API; stricter numeric/range/dupe checks; added inline helper text.
- Reverification job wiring: HTTP trigger (`POST /api/reverify/:projectId`), scheduler envs (`REVERIFY_PROJECTS`, `REVERIFY_INTERVAL_MINUTES`), metrics at `/api/metrics`, CLI (`cmd/reverify`), Redis-persisted counters, cumulative per-project metrics.

## Frontend Admin Audit (2026-01-03)
- Telemetry/monitoring: `TelemetryMonitorPage` and `RmsDashboardPage` depend on `/telemetry` (live via `useTelemetryStore` + history fetch), query params for device/from/live. `AnalyticsPage` uses `telemetryApi.getHistory`; export button is stub.
- Automation/rules/dna: `RulesPage` uses `rulesApi.*`; `DNAPage` uses `dnaApi` and `configApi` (sensors/thresholds/versions, automation summary). `ProjectBuilderPage` uses `projectApi.create` and `configApi.get/saveAutomationFlow`; builder canvases are FE-only.
- Access/identity: `APIKeysPage` hits `/api/admin/apikeys`; `UsersPage` hits `/api/admin/users` (admin-only, invite stub). Admin auth assumed via bearer token.
- Compliance/reporting: `CompliancePage` downloads Excel via `GET /api/reports/{projectId}/compliance`; “System Health PDF” card disabled.
- Domain modules: Maintenance (`/api/maintenance/work-orders` + resolve), Inventory/Logistics (geofences/products/stock, asset timeline at `/api/logistics/assets/{id}/timeline`; `LogisticsPage` disabled), Traffic (`/api/traffic/*` with simulate POST), Healthcare (`/api/healthcare/*` sessions/patients), Agriculture (`/api/agriculture/advice` + rules seed), Simulator (`/api/bootstrap?imei=…` for broker/topic/creds), GIS map (`deviceApi.list` expects coords fallback), Network planner (Leaflet-only, no persistence).
- Stubs/gaps: export in analytics, logistics module disabled, users invite stub, health PDF “coming soon”, digital twin/custom dashboard placeholders, network planner not persisted.

## Runtime smoke (local)
Validates the PM-KUSUM go-live flows against the running stack via real HTTP calls:
- Master Data CRUD
- Maintenance work-order create + resolve (accepts `{notes}`)
- Telemetry CSV export download
- Compliance report download (requires `manager` role)

From `unified-go/`:
```powershell
./scripts/smoke.ps1
```

Options:
```powershell
./scripts/smoke.ps1 -BaseUrl https://localhost -ProjectId smoke_proj -SkipComposeDown
```

See also: `docs/deferredForV3.md` (non-PM-KUSUM verticals + enablement plan).
