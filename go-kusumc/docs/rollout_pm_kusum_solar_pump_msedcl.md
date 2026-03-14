# PM Kusum Solar Pump (MSEDCL) Rollout Steps

Use these commands as a runbook to stand up the project end-to-end. Replace placeholders for hostnames, creds, and IDs as needed. All commands are PowerShell-friendly.

## 0) Prereqs
- Backend reachable at `http://localhost:8081` (adjust if different).
- EMQX Dashboard/API reachable (default `http://localhost:18083`), admin credentials available.
- Timescale/Postgres reachable (example DSN below).
- `psql`, `curl`/`Invoke-RestMethod`, and `jq` available in your shell.

## 1) Upsert DNA/config for payloadSchemas
- Artifact: `unified-go/docs/dna_pm_kusum_solar_pump_msedcl.json`
- Endpoint: replace `<DNA_CONFIG_ENDPOINT>` with your config service URL (or direct DB upsert if you manage DNA manually).

```powershell
$body = Get-Content "unified-go/docs/dna_pm_kusum_solar_pump_msedcl.json" -Raw
Invoke-RestMethod -Method Put -Uri "<DNA_CONFIG_ENDPOINT>/config/projects/pm-kusum-solar-pump-msedcl" -ContentType "application/json" -Body $body
```

## 2) Refresh runtime config (ConfigSync)
- If the server reads config on startup: restart `unified-go`.
- If you expose a sync endpoint (adjust path if different):

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8081/api/admin/config/sync"
```

## 3) Apply Timescale hypertable
- Artifact: `unified-go/docs/timescale_hypertable.sql`
- Example DSN placeholders: `postgresql://tsuser:tspass@localhost:5432/iotdb`

```powershell
psql "postgresql://tsuser:tspass@localhost:5432/iotdb" -f "unified-go/docs/timescale_hypertable.sql"
```

## 4) Import EMQX bridge rule
- Artifact: `unified-go/docs/emqx_rule_template.json`
- Replace placeholders inside the JSON for project/device lookups before posting.
- Default EMQX API: `http://localhost:18083/api/v5/rules`; auth Basic `admin:public` (adjust).

```powershell
$rule = Get-Content "unified-go/docs/emqx_rule_template.json" -Raw
Invoke-RestMethod -Method Post -Uri "http://localhost:18083/api/v5/rules" -Headers @{ Authorization = "Basic YWRtaW46cHVibGlj" } -ContentType "application/json" -Body $rule
```

## 5) Smoke checks
1) Provision a device via `/api/devices` with project/device identity fields; capture `device_id`.
2) Bootstrap: `GET /api/bootstrap?imei=<imei>` returns topics and envelope keys.
3) Publish sample heartbeat/data/daq to `channels/pm-kusum-solar-pump-msedcl/messages/<imei>`; expect `status=verified`.
4) Commands: POST `/api/commands` (packet_type `ondemand_cmd`); ingest `ondemand_rsp` and ensure `msg_id`/`msgid` correlation updates command status.
5) Verify Timescale row inserts in `telemetry` and Redis hot cache updates.

## 6) ACL tightening
- Allow publish/subscribe only on `channels/pm-kusum-solar-pump-msedcl/messages/<imei>` and `channels/pm-kusum-solar-pump-msedcl/commands/<imei>`.
- Deny wildcards broader than the project scope.

## 7) Rollback notes
- Delete the EMQX rule via the Rules API if needed.
- Drop the Timescale hypertable or delete rows for this project if rollback is required.
- Remove the project config entry from DNA if decommissioning.
