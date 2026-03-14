# REST API Contract Map (Legacy-only)

NOTE:
- Firmware developers should use `for-firmware-agent/` as the primary source.
- This document is split into:
	- **Firmware-facing quick reference** (small set of endpoints firmware may call)
	- **Ops/tooling appendix** (imports, alias conventions, rate limits, and admin-facing notes)

For executable-style firmware request/response payload examples, see `09-device-api-samples.md`.

## Firmware-facing quick reference

### Required
- `GET /api/bootstrap?imei=<imei>` (header: `x-api-key`)

### Recommended (recovery when MQTT continuity is uncertain)
- `GET /api/device-open/commands/status?imei=<imei>`
- `GET /api/device-open/commands/history?imei=<imei>&limit=<n>`
- `GET /api/device-open/commands/responses?imei=<imei>&limit=<n>`

### Optional (gateway mode)
- `GET /api/device-open/nodes?imei=<gateway_imei>`

### Optional (VFD)
- `GET /api/device-open/vfd?imei=<imei>` (or `device_uuid=<uuid>`)

### Optional (device errors: MQTT fallback)
If firmware cannot publish to `<imei>/errors` via MQTT, it may POST the same error event via HTTP:
- `POST /api/device-open/errors?imei=<imei>`
- Aliases also supported:
	- `POST /api/devices/open/errors?imei=<imei>`
	- `POST /api/v1/device-open/errors?imei=<imei>`
	- `POST /api/v1/devices/open/errors?imei=<imei>`

Diagram (deployment modes):

![](diagrams/00-deployment-modes.flowchart.svg)

## Appendix: Ops/tooling notes (not required for firmware)

Source of truth:
- Complete extracted route list: `docs/route-compare.md`.

## 1) Device bootstrap and open endpoints

### Canonical prefixes (chosen)
- `/api/device-open/*`
- `/api/v1/device-open/*`

### Legacy aliases (supported)
- `/api/devices/open/*`
- `/api/v1/devices/open/*`

Base URL modes used in this doc:
- **HTTP (dev/integration)**: `http://<host>:8081` (direct Go service; also used by `docker-compose.integration.yml`).
- **HTTPS (prod-like)**: `https://<host>` (via Nginx on 443; `docker-compose.yml` publishes only 80/443/8883).

| Purpose | Legacy path | Go path(s) | Notes |
|---|---|---|---|
| Bootstrap | `/api/devices/open/bootstrap` | **Canonical:** `/api/bootstrap`  \
**Aliases:** `/api/device-open/bootstrap`, `/api/v1/device-open/bootstrap`, `/api/devices/open/bootstrap`, `/api/v1/devices/open/bootstrap` | Canonical bootstrap is guarded via API key (`x-api-key`). Alias routes redirect to `/api/bootstrap` with the same querystring. |
| Local credentials | `/api/devices/open/credentials/local` | `/api/device-open/credentials/local`, legacy alias | Mirrored |
| Government credentials | `/api/devices/open/credentials/government` | `/api/device-open/credentials/government`, legacy alias | Mirrored |
| VFD models | `/api/devices/open/vfd` | `/api/device-open/vfd`, legacy alias | Mirrored |
| Installation + beneficiary | `/api/devices/open/installations/:device_uuid` | `/api/device-open/installations/:device_uuid` (+ legacy + `/api/v1/...` aliases) | Returns installation context and beneficiary details for a device UUID. |
| Command history fallback | `/api/devices/open/commands/history` | `/api/device-open/commands/history`, `/api/v1/device-open/commands/history` (+ legacy aliases) | Open-device HTTP fallback for fetching recent commands by `imei` or `device_uuid` (temporary aliases `deviceUuid`, `deviceId` may be accepted) |
| Command responses fallback | `/api/devices/open/commands/responses` | `/api/device-open/commands/responses`, `/api/v1/device-open/commands/responses` (+ legacy aliases) | Open-device HTTP fallback for recent command response packets |
| Command status fallback | `/api/devices/open/commands/status` | `/api/device-open/commands/status`, `/api/v1/device-open/commands/status` (+ legacy aliases) | Open-device HTTP fallback for command counters/retry posture |
| Device error ingest (MQTT fallback) | (none) | `POST /api/device-open/errors` (+ legacy + `/api/v1/...` aliases) | Body matches `<imei>/errors` schema: `open_id`, `timestamp`, `error_code`, `error_data` |

## 2) Telemetry ingest and history

| Purpose | Legacy | Go | Notes |
|---|---|---|---|
| Ingest telemetry | `/api/telemetry/ingest` | `/api/telemetry/ingest` and `/api/ingest` pathways | Behavior depends on auth/config |
| HTTPS mirror | `/api/telemetry/mirror/:topic_suffix` | `/api/telemetry/mirror/:topic_suffix`, `/api/v1/telemetry/mirror/:topic_suffix` | Mirrored |
| Device history | `/api/telemetry/devices/:device_uuid/history` | Same | Mirrored |
| Device latest | `/api/telemetry/devices/:device_uuid/latest` | Same | Mirrored |

## 3) Commands

| Purpose | Legacy | Go | Notes |
|---|---|---|---|
| Issue command | `/api/devices/:device_uuid/commands` | Same + `/api/commands/send` | Mirrored with extra API |
| Ack command | `/api/devices/:device_uuid/commands/ack` | Same | Mirrored |
| Command history | `/api/devices/:device_uuid/commands/history` | Same | Mirrored |

## 3.1) Device configuration apply (HTTP queue + MQTT delivery)

The UI uses HTTP endpoints to queue and track configuration, but the backend delivers configuration to the device via the standard MQTT command pipeline.

MQTT delivery behavior:
- Command topic: `<imei>/ondemand`
- Command name: `apply_device_configuration`
- Correlation rule: server publishes the command with `msgid=config_id`.
- Device response: echo the command `msgid` (recommended). If supported, also include `correlation_id=config_id`.

| Purpose | Go path | Notes |
|---|---|---|
| Queue configuration (creates config_id and publishes MQTT command) | `POST /api/devices/:idOrUuid/configuration`  \
`POST /api/v1/devices/:idOrUuid/configuration` | Body is device-configuration JSON; server persists the configuration row and publishes `apply_device_configuration` to the device.
| Get pending configuration | `GET /api/devices/:idOrUuid/configuration/pending`  \
`GET /api/v1/devices/:idOrUuid/configuration/pending` | Returns latest `pending` config record for the device (or `204` when none).
| Ack configuration (manual/legacy ack path) | `POST /api/devices/:idOrUuid/configuration/ack`  \
`POST /api/v1/devices/:idOrUuid/configuration/ack` | Accepts an ack payload and marks the pending record acknowledged. Devices should prefer MQTT response correlation; this route exists for compatibility and tools.


## 4) Bulk import and jobs

| Purpose | Legacy | Go | Status |
|---|---|---|---|
| Device import | `/api/devices/import` | `/api/devices/import` | Present |
| Device config import | `/api/devices/configuration/import` | `/api/devices/configuration/import` | Present, request-style differs |
| Govt credential import | `/api/devices/government-credentials/import` | Same route exists | Present; now accepts bulk JSON array and legacy CSV wrappers |
| Import job list/get/errors/retry | `/api/devices/import/jobs*` | Same family of routes | Present |
| Govt import jobs (list/get/errors/retry) | Legacy dedicated route exists | `/api/devices/government-credentials/import/jobs*` (+ `/api/v1/...` aliases) + generic `/api/devices/import/jobs?type=government_credentials_import` | Present |

## 5) Contract compatibility notes
- Keep firmware-facing APIs stable using legacy aliases where needed.
- For CSV imports, document exact content-type/payload shape expected by current Go handlers before client rollout.
- Government credential imports (CSV and JSON-array modes) persist import jobs; CSV responses return `job_id` for success/partial/failure and jobs remain queryable via import-job routes.
- Government credential import responses set `X-Import-Job-Id` when a job is persisted, giving clients a uniform job-tracking hook across JSON-array and CSV modes.
- For command APIs, treat correlation ID and msgid mapping as required for reliable response tracking.
- For device configuration apply, treat `config_id` as the correlation ID; echo `msgid=config_id` (recommended) and optionally include `correlation_id=config_id`.

## UI niceties (notes)
- Persist and display the full trigger payload for each alert (for operator trust/debuggability).
- Minimum useful fields to show on alert/event rows: `severity`, `device` (IMEI), `first_seen`, `last_seen`, `count`.
- If you want `first_seen`/`last_seen`/`count` to be **true aggregation** (instead of per-event defaults), pick the grouping key/window (for example: `device_id + error_code` within 15 minutes vs “active until resolved”).

## 6) Government credential import examples

### A) JSON-array mode (201 Created)

Request:

```bash
curl -i -X POST "http://localhost:8081/api/devices/government-credentials/import" \
	-H "Authorization: Bearer <token>" \
	-H "Content-Type: application/json" \
	-d '[
		{
			"device_id": "resolved-dev-1",
			"protocol_id": "proto-1",
			"username": "user1",
			"password": "pass1"
		}
	]'
```

Response (example):

```http
HTTP/1.1 201 Created
X-Import-Job-Id: 8d6f2a42-a7cc-4eb3-aeb1-5e0301e6de47
Content-Type: application/json

[
	{
		"device_id": "resolved-dev-1",
		"protocol_id": "proto-1",
		"username": "user1",
		"password": "pass1"
	}
]
```

### B) CSV mode (207 Partial Success)

Request:

```bash
curl -i -X POST "http://localhost:8081/api/devices/government-credentials/import" \
	-H "Authorization: Bearer <token>" \
	-H "Content-Type: text/csv" \
	--data-binary $'device_id,protocol_id,username,password\nresolved-dev-1,proto-1,user1,pass1\nresolved-dev-2,,user2,pass2\n'
```

Response (example):

```http
HTTP/1.1 207 Multi-Status
X-Import-Job-Id: 8d6f2a42-a7cc-4eb3-aeb1-5e0301e6de47
Content-Type: application/json

{
	"success_count": 1,
	"error_count": 1,
	"errors": [
		"row 3 (resolved-dev-2): protocol_id required"
	],
	"job_id": "8d6f2a42-a7cc-4eb3-aeb1-5e0301e6de47"
}
```

Track job details:

```bash
curl -H "Authorization: Bearer <token>" \
	"http://localhost:8081/api/devices/government-credentials/import/jobs/8d6f2a42-a7cc-4eb3-aeb1-5e0301e6de47"
```

List filters compatibility:
- Import job list endpoints accept `type`, `jobType`, `job_type`, `importType`, and `import_type` as equivalent query aliases.
- Device list endpoint accepts `projectId`/`project_id` and `includeInactive`/`include_inactive` aliases.
- Analytics endpoints accept aliases for key filters: `projectId`/`project_id`, `packet_type`/`packetType`, `from`/`start`, `to`/`end`, `exclude_quality`/`excludeQuality`, and device selectors (`device`, `deviceId`, `device_id`, `deviceUuid`, `device_uuid`, `imei`).
- Device list and analytics history endpoints accept pagination aliases `page`/`pageNumber` and `limit`/`pageSize`.
- Audit endpoint accepts camelCase and snake_case aliases for scoped filters: `afterId`/`after_id`, `actorId`/`actor_id`, `stateId`/`state_id`, `authorityId`/`authority_id`, `projectId`/`project_id`.
- Alerts endpoint accepts `projectId`/`project_id` for project scope and `status`/`status_filter` for status filtering.
- User groups list endpoint accepts `stateId`/`state_id`, `authorityId`/`authority_id`, and `projectId`/`project_id` aliases.
- Command catalog admin list accepts `projectId`/`project_id` and `deviceId`/`device_id`/`imei` aliases for capability scoping.
- Telemetry export endpoint accepts `projectId`/`project_id`, `imei`/`deviceId`/`device_id`, `start`/`from`, `end`/`to`, `packetType`/`packet_type`, `quality`/`data_quality`, and `exclude_quality`/`excludeQuality` aliases for scope and filter selection.
- Master data list endpoint accepts `projectId` and `project_id` aliases for project scoping.
- Admin list/search filters accept camelCase and snake_case aliases for scoped keys such as `stateId`/`state_id`, `authorityId`/`authority_id`, `projectId`/`project_id`, `groupId`/`group_id`, `roleKey`/`role_key`, and `serverVendorId`/`server_vendor_id`.
- Admin list/search also accepts `status`/`status_filter` aliases for account-status filtering.
- Vertical/domain list endpoints now accept aliases for key filters, including `projectId`/`project_id`, `accountStatus`/`account_status`, `installationUuid`/`installation_id`, `deviceUuid`/`device_id`/`deviceId`, `status`/`status_filter`, `manufacturerId`/`manufacturer_id`, `protocolVersionId`/`protocol_version_id`, and `includeSoftDeleted`/`include_soft_deleted`.
- Device and beneficiary list endpoints accept `search` and `q` aliases for text query filtering.
- Cursor-paginated list endpoints support `cursor`, `afterId`, and `after_id` aliases where cursor-style pagination is used.
- Admin list, user-groups list, and vertical list handlers accept `limit` and `pageSize` as equivalent page-size aliases.
- Simulator session list endpoint accepts `limit`/`pageSize` for page size and `cursor`/`afterId`/`after_id` for cursor pagination.
- Simulator session list endpoint accepts `status`/`status_filter` aliases for status filtering.
- Audit list endpoint accepts `limit`/`pageSize` for page size alongside `afterId`/`after_id`/`cursor` cursor aliases.
- Device configuration import endpoint accepts `projectId`/`project_id` aliases for project scope.

## 7) Common query alias conventions

- **Canonical:** snake_case on the wire (HTTP query + HTTP JSON + MQTT JSON).
- **Temporary compatibility:** some handlers may still accept legacy camelCase aliases during migration (target removal date: **2026-04-01**).
- **Strict mode (optional):** when `STRICT_SNAKE_WIRE=true` is enabled on the server, camelCase query keys and camelCase JSON keys are rejected on `/api/*`.
- Avoid sending both forms of the same key in a single request.
- **Search aliases:** Text search commonly accepts `search` and `q`.
- **Page aliases:** Page-number pagination commonly accepts `page` and `pageNumber`.
- **Limit aliases:** Page-size pagination commonly accepts `limit` and `pageSize`.
- **Cursor aliases:** Cursor pagination commonly accepts `cursor`, `afterId`, and `after_id`.
- **Cursor precedence:** When multiple cursor keys are present, behavior is implementation-defined; prefer `cursor` only.
- **Boolean aliases:** Boolean toggles support paired forms when present (for example `includeSoftDeleted` + `include_soft_deleted`).

## 8) Device identifier naming convention

- Canonical query keys:
	- `imei` (firmware primary identifier)
	- `device_uuid` (device UUID)
	- `device_id` (device internal UUID)
- Some endpoints may temporarily accept camelCase aliases like `deviceUuid`/`deviceId`, but clients should not rely on them.
- Note: strict mode (`STRICT_SNAKE_WIRE=true`) will reject camelCase aliases.

## 9) API rate-limit profile (Nginx edge)

- Rate limiting is applied at Nginx by client IP.
- **Default `/api/*` profile**: `120 requests/minute`, burst `60` (`nodelay`).
- **Device/bootstrap profile** (higher burst tolerance): `300 requests/minute`, burst `120` (`nodelay`) for:
	- `/api/bootstrap`
	- Canonical: `/api/device-open/*`, `/api/v1/device-open/*`
	- Legacy aliases (supported): `/api/devices/open/*`, `/api/v1/devices/open/*`
- Purpose: avoid false throttling during device reconnect/bootstrap cycles while retaining tighter control on non-device API surfaces.
