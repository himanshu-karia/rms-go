# Integration E2E Report (2026-02-25)

## Scope
- Stack: `docker-compose.integration.yml`
- Target package: `./tests/e2e` with `-tags=integration`
- Profile: legacy-only topics (`MQTT_COMPAT_TOPICS_ENABLED=false`)

## Commands Executed
1. `docker compose -f docker-compose.integration.yml down -v --remove-orphans`
2. `docker compose -f docker-compose.integration.yml up --build --abort-on-container-exit --exit-code-from test-runner test-runner`
3. `docker compose -f docker-compose.integration.yml run --rm test-runner`
4. Verbose run: `docker compose -f docker-compose.integration.yml run --rm --entrypoint /bin/bash test-runner -lc "./tests/e2e/run_integration_verbose.sh | tee ./tests/e2e/e2e-integration-verbose-2026-02-25.log"`

## Final Status
- Result: **FAIL**
- Primary failure class: MQTT auth failures for per-device credentials (`not Authorized` / `bad user name or password`).
- Latest baseline run summary: `FAIL ingestion-go/tests/e2e`

## Verbose Test Outcomes
Source log: `tests/e2e/e2e-integration-verbose-2026-02-25.log`

### Failed
- `TestDeviceCommandLifecycle`
- `TestDeviceLifecycle`
- `TestKusumFullCycle`
- `TestMQTTCredRotation`
- `TestMQTTRotationForcesDisconnect`
- `TestRMSMegaFlow` (subtest `telemetry_ingest_retention`)
- `TestSolarRMSFullCycle`
- `TestUIAndDeviceOpenFullCycle`

### Passed
- `TestDeviceConfigurationApply`
- `TestDeviceOpenAliasCoverage`
- `TestStory_FullCycle`
- `TestRMSMegaFlow` subtests: `scenario_seed_project_protocol_dna`, `device_lifecycle_bootstrap`, `command_catalog`, `commands_roundtrip`, `rules_virtual_sensors`, `analytics_dashboards`, `negative_validation`, `cleanup_consistency`, `auth_rbac`

### Skipped
- `TestBootstrapConnectPersist` (env-gated)
- `TestLiveBootstrapTLS` (env-gated)
- `TestRMSMegaFlow/masterdata_org_project_protocol_dna` (endpoint instability path)

## Changes Applied During Stabilization
- Added configurable per-phase timeouts and separate contexts in long lifecycle tests.
- Added support for both `statusCounts` and `status_counts` in device-open status assertion.
- Fixed `TestDeviceLifecycle` IMEI generation to use numeric IMEI format.
- Added provisioning wait before device MQTT use in selected tests.
- Added shared MQTT connect retry helper (`tests/e2e/mqtt_connect_retry_helper.go`) and wired it into common connect paths.
- Added richer failure diagnostics for command-status wait in `TestDeviceLifecycle`.

## Current Blocker
Despite retries and provisioning waits in updated paths, multiple tests still fail on MQTT auth for device credentials. This indicates an environment-level or provisioning consistency issue (credential generation/provisioning acceptance in EMQX) rather than a single test assertion mismatch.

## Suggested Next Step
Instrument provisioning + bootstrap consistency in backend/EMQX integration:
1. Log created username/password hash source and EMQX API response payload for each provisioning job.
2. Add a post-provision validation step in worker (attempt MQTT auth or EMQX user lookup + ACL verification) before marking job complete.
3. In tests, add a reusable `waitForDeviceMQTTAuth` helper keyed by IMEI/user prior to first device MQTT connect.
