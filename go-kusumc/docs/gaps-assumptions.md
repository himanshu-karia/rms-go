# Gaps & assumptions (frontend ↔ backend alignment)

This document lists the **current integration gaps** and the **assumptions** that must hold for the system to work in each mode.

It is intentionally grounded in repo code:
- Frontend “Studio LIVE” MQTT + device registration: `new-frontend/apps/web/src/features/studio/adapters/LiveBackend.ts`
- Frontend devices API: `new-frontend/packages/core/src/api/devices.ts`
- Go server route wiring: `unified-go/cmd/server/main.go`
- Go device controller: `unified-go/internal/adapters/primary/http/device_controller.go`

Related deep dives:
- MQTT provisioning + ACL: `provisioning-emqx-acl.md`
- Ingestion payload strictness: `payload-contract.md`
- Telemetry pipeline: `dataflow-telemetry-archive.md`
- Rules + virtual sensors wiring: `rules-virtual-sensors.md`

## Mode A — Frontend-only (mocked backend)

### Assumptions
- You are using the Studio mock backend path (not calling real `/api/*`).
- No device provisioning/ACL is required (because there is no real broker auth being enforced).
- Admin pages are either not used, or are also mocked.

### Gaps (what breaks if assumptions aren’t true)
- **Admin pages call real HTTP APIs**:
  - `DevicesPage.tsx` loads via `deviceApi.getInventory()` → `GET /api/devices` (via core client base `/api`).
  - `RulesPage.tsx` loads via `rulesApi.list()` and `deviceApi.getInventory()`.
  - If you run “frontend-only” without a backend, these pages will error unless you provide a mock server or switch them to a mock adapter.

### Practical implications
- “Mock mode” today is best scoped to Studio simulation UX.
- If you want a true “frontend-only demo,” you need either:
  - a mocked HTTP layer for `@hk/core` APIs, or
  - a dev mock server that implements the endpoints the pages call.

## Mode B — Frontend + Go backend (real APIs)

### Assumptions
- Vite dev proxy is used so the frontend can call Go at `http://localhost:8081` via `/api`.
- Auth is satisfied for protected routes (JWT or API key as wired in Go).
- MQTT broker is reachable at the URL used by the frontend, and the broker path `/mqtt` is routed correctly.

### Gaps (blockers)

#### 1) Devices list/inventory API mismatch (HTTP)
- Frontend expects:
  - `GET /api/devices` returning either an array OR an object like `{ devices, total, page, pages }`.
  - Fields shaped like Mongo-ish `_id`, plus `uuid`, `model_id`, etc (`new-frontend/packages/core/src/api/devices.ts`).
- Go backend currently provides:
  - `POST /api/devices` only (`protected.Post("/devices", ...)` in `unified-go/cmd/server/main.go`).
  - No `GET /api/devices` route exists.

Impact:
- Admin inventory (`DevicesPage.tsx`) and rules page device dropdown (`RulesPage.tsx`) cannot populate from Go.

Resolution options (pick one direction)
- **Implement parity in Go**: add `GET /api/devices` (and shape) to match `deviceApi.getInventory`.
- **Align frontend to Go**: change `deviceApi.list/getInventory` + pages to use Go’s actual endpoints and response shapes.

#### 2) Provisioning ACL topic pattern does not match runtime topics (MQTT)
- Runtime topics used by frontend LIVE + Go ingestion/commands:
  - Telemetry publish: `channels/{project_id}/messages/{imei}` (frontend publishes; Go subscribes)
  - Commands subscribe: `channels/{project_id}/commands/{imei}` (Go publishes; frontend subscribes)
- Provisioning worker currently grants different topics:
  - publish: `projects/{project_id}/devices/{imei}/+`
  - subscribe: `projects/{project_id}/devices/{imei}/cmd`

Impact:
- If EMQX ACL enforcement is enabled for device identities, provisioned devices will be denied on the actual `channels/...` topics.

See: `provisioning-emqx-acl.md` for the full “code truth” and the minimal recommended ACL mapping.

### Gaps (risks / drift)

#### 3) Device create payload naming drift (`attributes` vs `metadata`)
- Go `POST /api/devices` expects JSON:
  - `{ name, imei, projectId, attributes }` (`device_controller.go`).
- Frontend create forms and models commonly use `metadata` (`DevicesPage.tsx`, `@hk/core` Device type).

Impact:
- Device enrollment may silently drop extra fields unless the frontend maps `metadata` → `attributes` (or Go accepts `metadata`).

#### 4) Rules UI schema drift vs backend rule engine
- `RulesPage.tsx` includes fields like `executionLocation` and advanced trigger `formula`.
- The Go rules path described in `rules-virtual-sensors.md` is centered on server-side evaluation and MQTT alert publishing.

Impact:
- The UI may successfully POST fields that the backend ignores, rejects, or stores inconsistently depending on the rule repository implementation.

#### 5) “Strict payload” is undermined by inconsistent sensor keys (`param` vs `id`)
- `payload-contract.md` documents that strict verification currently uses one key (`param`) while transformation uses another (`id`).

Impact:
- Packets can be marked suspicious (or pass) depending on how project DNA defines sensors, even if telemetry is otherwise valid.

#### 6) Dual ingestion stacks exist (integration complexity)
- `rules-virtual-sensors.md` notes two stacks: core/services (wired) and engine/pipeline (present, not wired in `cmd/server`).

Impact:
- Feature work can accidentally target the non-wired stack.

### Assumptions you should explicitly decide (policy)
- **Which telemetry topic family is canonical**?
  - `channels/{project_id}/messages/{imei}` vs `devices/{imei}/telemetry`
  - Pick one for devices, then align provisioning ACL and documentation.
- **What is the security posture for `/api/ingest`**?
  - Today it is effectively “open” (identity captured if key present). Decide if production should enforce API key/JWT.

## Quick checklist (what must align)

### For a functional end-to-end demo
- HTTP:
  - Device listing/inventory endpoint exists and the response shape matches UI expectations.
  - Device create accepts the fields the UI sends (`metadata`/`attributes` mapped).
- MQTT:
  - Topics match across device publish, server subscribe, server publish, device subscribe.
  - Provisioning ACL topic patterns authorize those exact topics.
- Payload:
  - Telemetry JSON includes `imei` and (practically) `project_id`, with sensor values at top-level keys.
  - Project DNA sensor keys use one canonical identifier consistently.
