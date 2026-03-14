# Local Custom URLs Migration Plan (HTTPS + MQTTS)

Date: 2026-03-05

## Decision
Adopt custom local endpoints to avoid localhost/port conflicts across parallel stacks.

- HTTPS API base: `https://rms-iot.local:7443`
- MQTTS broker: `mqtts://rms-iot.local:18883`
- MQTT WSS: `wss://rms-iot.local:7443/mqtt`

Production endpoint split (dedicated subdomains):
- UI: `https://ui-kusumc.hkbase.in`
- API: `https://api-kusumc.hkbase.in`
- MQTT/MQTTS: `mqtt-kusumc.hkbase.in`

## Why
- Avoids collisions with other services using `localhost:443`, `localhost:8883`, and mixed local stacks.
- Keeps RMS test/runtime targeting deterministic.
- Reduces accidental cross-talk with unrelated local services.

## Config files to update
- `rms-go/go-kusumc/.env.example`
- `rms-go/go-kusumc/.env.local`
- `rms-go/go-kusumc/docker-compose.yml`
- `rms-go/go-kusumc/infra/nginx/certs/localhost-certs/server.cnf`

## Code files to update
- `rms-go/app-kusumc/RMSMQTT1 - L/app/src/main/java/com/autogridmobility/rmsmqtt1/data/mobile/MobileApiClient.kt`
- `rms-go/app-kusumc/RMSMQTT1 - L/app/src/main/java/com/autogridmobility/rmsmqtt1/data/mobile/AdminApiClient.kt`
- `rms-go/app-kusumc/RMSMQTT1 - L/app/src/main/java/com/autogridmobility/rmsmqtt1/transport/command/HttpCommandTransport.kt`
- `rms-go/app-kusumc/RMSMQTT1 - L/app/src/main/java/com/autogridmobility/rmsmqtt1/utils/MqttPreferencesManager.kt`
- `rms-go/app-kusumc/RMSMQTT1 - L/app/src/main/res/xml/network_security_config.xml`
- `rms-go/app-kusumc/scripts/mobile-adb-inject-server-otp.ps1`
- `rms-go/app-kusumc/scripts/mobile-ui-auditor.ps1`
- `rms-go/app-kusumc/scripts/long-automated-mobile-auth-to-persistence.ps1`

## Test and runner files to update
- `rms-go/go-kusumc/tests/e2e/profile_sessions_integration_test.go`
- `rms-go/go-kusumc/scripts/run-integration.ps1`
- `rms-go/go-kusumc/scripts/run-e2e-ordered.ps1`
- `rms-go/go-kusumc/scripts/run-integration.sh`
- `rms-go/go-kusumc/scripts/run-e2e-ordered.sh`

## Docs to update
- `rms-go/go-kusumc/README.md`
- `rms-go/go-kusumc/docs/README.md`

## Current implementation status
- ✅ Config defaults updated in env and compose for custom host/ports.
- ✅ Android default API + MQTT endpoints updated.
- ✅ Android TLS domain allowlist includes `rms-iot.local`.
- ✅ Integration runner/test defaults updated for custom endpoints.
- ✅ Production env now models dedicated `ui/api/mqtt` KUSUMC subdomains.
- ✅ Docker compose now supports env/path-driven TLS cert selection for nginx + EMQX.
- ✅ Backend/docs examples updated.
- ⚠️ Local DNS/hosts mapping still required on each machine (`rms-iot.local -> 127.0.0.1`).
- ⚠️ Local certificate should be regenerated/reissued with SAN containing `rms-iot.local` before strict TLS validation.

## Provisioning/device test impact
Yes, this affects provisioning/bootstrap-driven tests and real device bootstrap because host/port in bootstrap payload and test defaults change.

Required alignment:
- `MQTT_PUBLIC_HOST=rms-iot.local`
- `MQTT_PUBLIC_PORT=18883`
- `MQTT_PUBLIC_PROTOCOL=mqtts`
- `BASE_URL=https://rms-iot.local:7443`
- `BOOTSTRAP_URL=https://rms-iot.local:7443/api/bootstrap`

## Implementation plan
1. Update env and compose defaults to custom host/ports.
2. Update Android hardcoded defaults + allowed TLS domains.
3. Update integration/e2e defaults and docs examples.
4. Re-generate/re-issue local cert using SAN containing `rms-iot.local`.
5. Ensure local DNS/hosts mapping for `rms-iot.local` points to local machine.
6. Run focused validation (`go test` targeted, Android build, key integration checks).

## Post-change operator checklist
1. Add host mapping on dev machine:
   - `127.0.0.1 rms-iot.local`
2. Recreate cert from `server.cnf` and replace:
   - `infra/nginx/certs/localhost-certs/rms-local.crt`
   - `infra/nginx/certs/localhost-certs/rms-local.key`
3. Import CA cert into trusted store (host + emulator/device if needed).
4. Restart stack:
   - `docker compose down --remove-orphans`
   - `docker compose up -d --build`
5. Validate:
   - `https://rms-iot.local:7443/api/health`
   - bootstrap endpoint and one live MQTTS connect/publish flow.

## Production cert switch (fresh root certs)
Fresh cert source (repo root): `certs/wildcard.hkbase.in/`

Compose/env-driven knobs:
- `DEPLOY_CERTS_DIR` (host path mounted into containers)
- `NGINX_SSL_CERT`, `NGINX_SSL_KEY`
- `EMQX_SSL_CERTFILE`, `EMQX_SSL_KEYFILE`
- `EMQX_WSS_CERTFILE`, `EMQX_WSS_KEYFILE`

Default production wiring is in `rms-go/go-kusumc/.env.prod`.

## Automation helper
Use script: `rms-go/go-kusumc/scripts/setup-custom-local-domain.ps1`

Examples:
- Full setup: `./scripts/setup-custom-local-domain.ps1`
- Only cert + CA import: `./scripts/setup-custom-local-domain.ps1 -ReissueCert -ImportCa`
- Only hosts update: `./scripts/setup-custom-local-domain.ps1 -UpdateHosts`
