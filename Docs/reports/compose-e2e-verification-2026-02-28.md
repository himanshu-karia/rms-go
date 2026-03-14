# Compose + E2E Verification (2026-02-28)

## Scope executed
1. `docker compose` teardown (integration stack)
2. fresh `up --build` with dependency sequencing
3. DB seed/init verification
4. full integration E2E run (`go test -tags=integration ./tests/e2e` via compose `test-runner`)
5. failure diagnosis + fix + rerun

## Compose/dependency checks
- Compose files validated and service graph inspected from `docker-compose.integration.yml`.
- Dependency sequencing is present and effective:
  - `server` depends on healthy `redis`, `timescaledb`, `emqx`, and successful `emqx-bootstrapper`.
  - `timescaledb`, `redis`, `emqx` include health checks.
  - `test-runner` waits for `server` and also blocks on DB/server readiness in entrypoint.
- Seed/init path verified from logs and DB state:
  - Seeder ran and created PM-KUSUM hierarchy/project.
  - Admin users `Him` and `Hadi` present.

## E2E execution result
- Initial full E2E run failed at:
  - `TestRMSMegaFlow/auth_rbac`
  - reason: `GET /api/projects` returned 500.

## Root cause
- `project_repo` scanned nullable DB fields (`type`, `location`, `config`) into non-null string/interface without SQL fallback, causing:
  - `can't scan NULL into *string`

## Fix applied
- File updated:
  - `go-kusumc/internal/adapters/secondary/project_repo.go`
- Change:
  - Added SQL `COALESCE(...)` for nullable project fields in `GetProject`, `ListProjects`, and `GetProjectWithConfig` queries.

## Post-fix verification
- Live API sanity:
  - `GET /api/projects` now returns HTTP 200.
- Full E2E rerun:
  - `ok  ingestion-go/tests/e2e  22.608s`

## Notes
- Current compose setup is functionally correct for build/start dependency order.
- `test-runner` installs OS packages on every run (`apt-get` in entrypoint); this is valid but slow. Optional optimization: prebuild a test-runner image with those dependencies baked in.
