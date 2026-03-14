# UI coverage matrix (Frontend vs Backend)

This is a “what can we configure where?” matrix.

Legend:
- **UI: Full** = UI can create/read/update/delete the capability (or the practical equivalents).
- **UI: Partial** = UI exists but missing key operations/fields.
- **UI: None** = backend/API exists but UI doesn’t expose it.

## Core provisioning + bootstrap configuration

| Capability | Backend support (routes) | Current UI surface | UI coverage | Notes / gaps | Minimal UI to add |
|---|---|---|---|---|---|
| Project DNA (sensors list) | `GET/PUT /api/project-dna/:projectId/sensors` | `DNAPage` | Full | Includes CSV import + versions. | — |
| Project DNA (thresholds, project + device overrides) | `GET/PUT /api/project-dna/:projectId/thresholds` + `PUT /api/project-dna/:projectId/thresholds/:deviceId` + `GET /api/project-dna/:projectId/thresholds/devices` | `DNAPage` | Full | Device-scope supported via UI toggle. | — |
| Project DNA versions (publish/rollback) | `GET /api/project-dna/:projectId/sensors/versions` + create/publish/rollback + CSV download | `DNAPage` | Full | — | — |
| Bootstrap config fetch | `GET /api/bootstrap?imei=...` (public) | `DeviceDetailPage` (Bootstrap preview) | Full | Read-only preview via bootstrap refresh. | — |
| Unified MQTT cred rotation | `POST /api/devices/:id/rotate-creds` (protected) | `DeviceDetailPage` | Full | Rotate action shown; refreshes bootstrap. | — |
| Device provisioning status (credential history, attempts, last error) | stored server-side; exposed via bootstrap response provisioning fields | `DeviceDetailPage` | Full | Shows provisioning status + credential history id. | — |

## Broker profiles (protocol definitions)

| Capability | Backend support (routes) | Current UI surface | UI coverage | Notes / gaps | Minimal UI to add |
|---|---|---|---|---|---|
| Protocol profiles per project (primary/govt) | `POST /api/projects/:id/protocols`, `GET /api/projects/:id/protocols`, `DELETE /api/protocols/:id` | `ProtocolProfilesPage` | Full | Shows IDs for CSV helpers; create/delete primary+govt profiles. | — |
| Govt/vendor protocol version modeling | same as above (use `kind=govt`, `server_vendor_org_id`, topics, endpoint) | `ProtocolProfilesPage` | Full | Metadata/version tagging optional; can extend via metadata later. | — |

## Per-device govt credentials

| Capability | Backend support (routes) | Current UI surface | UI coverage | Notes / gaps | Minimal UI to add |
|---|---|---|---|---|---|
| Govt creds (per device) CRUD | `POST /api/devices/:id/govt-creds`, `GET /api/devices/:id/govt-creds`, `POST /api/devices/govt-creds/bulk` | `DeviceDetailPage` panel + CSV import | Full | Panel supports select govt protocol + upsert/view creds. CSV still supported. | — |

## Device inventory + basic metadata

| Capability | Backend support (routes) | Current UI surface | UI coverage | Notes / gaps | Minimal UI to add |
|---|---|---|---|---|---|
| List/search devices | `GET /api/devices` | `DevicesPage` | Full | Uses server table + search. | — |
| Create device | `POST /api/devices` | `DevicesPage` (Enroll) | Full | Creates device with name/project/metadata. | — |
| Update device name/project/metadata | `PUT /api/devices/:idOrUuid` | `DeviceEditPage` | Full | Supports name/project plus metadata JSON editor (tags/notes merged into metadata). | — |
| Bulk import devices | `POST /api/devices/import` | `BulkImportModal` | Full | Supports `protocol_id` + govt fields. | Consider adding inline helper: where to find `protocol_id`. |

## Context linking (installation/beneficiary/VFD)

| Capability | Backend support (routes) | Current UI surface | UI coverage | Notes / gaps | Minimal UI to add |
|---|---|---|---|---|---|
| Beneficiaries CRUD | `/api/beneficiaries` (protected) | `BeneficiariesPage` | Full | Create/list beneficiaries per project. | — |
| Installations CRUD | `/api/installations` (protected) | `InstallationsPage` | Full | Link device → beneficiary → project + location. | — |
| VFD manufacturers/models | `/api/projects/:projectId/vfd/manufacturers`, `/api/projects/:projectId/vfd/models` (+ import) | `VfdCatalogPage` | Full | Create manufacturers/models and import artifacts JSON per model. | — |
| Protocol ↔ VFD assignments | `/api/projects/:projectId/protocols/:protocolId/vfd-assignments` (+ revoke) | `VfdCatalogPage` | Full | Assign/revoke models to protocols per project. | — |

## Observability / day-2 ops

| Capability | Backend support (routes) | Current UI surface | UI coverage | Notes / gaps | Minimal UI to add |
|---|---|---|---|---|---|
| Telemetry history + export | `GET /api/telemetry` and `GET /api/telemetry/export` | `TelemetryMonitorPage`, `AnalyticsPage` | Full | `AnalyticsPage` already opens `/api/telemetry/export`. | — |
| Compliance report | `GET /api/reports/:id/compliance` | `CompliancePage` | Full | — | — |
| Reverify trigger | `POST /api/reverify/:projectId` (dev token optional) | `DNAPage` (Reverify button + optional token) | Full | Fires backend reverify for selected project; optional dev token input. | — |

## Recommended minimal UI additions (highest ROI)
1) `Admin → Protocol Profiles` (create/list/delete primary+govt profiles).
2) `DeviceDetailPage` panels:
   - “Bootstrap preview” (read-only)
   - “Provisioning status” (read-only)
   - “Rotate MQTT creds” action
   - “Govt broker creds” (upsert/view)

This keeps DNA + project-wide config “fill once” and device secrets “per device”, matching the backend model.
