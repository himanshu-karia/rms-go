# Provisioning & MQTT Credential Lifecycle Plan

## Goals
- Issue per-device MQTT credentials (client_id, username, password) with project-scoped publish/subscribe topics derived from Project DNA.
- Persist credential history with lifecycle (pending/applied/failed) and retryable provisioning jobs that reconcile after broker restarts.
- Ensure EMQX authn/authz sources exist before ACL writes; provide service-account bootstrap.
- Return credentials + topic map in provisioning/bootstrap responses so devices see DNA topic changes without redeploy.

## Scope
- Backend changes in `unified-go` only (no schema for frontend), but add contract for UI/IoT devices to consume bundle and provisioning state.
- Multi-project safe: topics and ACLs are namespaced by project/device; DNA drives topic resolution per project/protocol.
- Tests: unit where feasible, plus integration/e2e via docker-compose integration stack.

## Design
- **Topic derivation**: Add resolver that reads Project DNA (existing `project_dna` payload rows/edge rules/automation flows) to compute publish/subscribe topics per device and protocol. If DNA adds new topics, resolver outputs updated lists; provisioning/bootstrap responses include them.
- **Credential bundle**: Struct containing `client_id`, `username`, `password`, `endpoints[]`, `publish_topics[]`, `subscribe_topics[]`, `protocol_version`, `project_id`, `device_id`, timestamps.
- **Persistence**: New `credential_history` table storing bundle JSON, lifecycle state, `mqtt_access_applied` flag, attempt counters, and metadata (protocol selector, issued_by). Device row keeps latest bundle under attributes for quick fetch.
- **Provisioning job**: Extend `mqtt_provisioning_jobs` to link to `credential_history` entry and store next-attempt/backoff. Worker loads bundle from history, not device row.
- **Worker**: Ensure EMQX authz source exists, upsert user, replace ACLs with bundle topics, mark credential lifecycle `applied` and job `completed`, or `failed` with backoff. Optional reconciliation loop: scan pending/failed and requeue.
- **Service bootstrap**: CLI/command to provision backend service account in EMQX using same ensure-authz-source logic.
- **Bootstrap response**: `/api/devices` and `/api/bootstrap` return latest credential bundle + topic map and relevant DNA snippets (e.g., payload schema version, firmware channel) so devices adapt to new topics.

## Data Model Changes
- Add table `credential_history` (id, device_id, bundle JSON, lifecycle ENUM, lifecycle_history JSON, mqtt_access_applied BOOL, attempt_count INT, last_error TEXT, created_at, updated_at).
- Update `mqtt_provisioning_jobs` to reference `credential_history_id` and store next_attempt_at, last_error, attempt_count.
- Migration to backfill existing devices: generate bundle per device from current attributes and enqueue jobs.

## API Changes
- `POST /api/devices`: return bundle + provisioning state; write credential_history + job.
- `/api/bootstrap`: include bundle, topic map, DNA metadata (schema version, firmware channel) for the device.

## Worker Flow
1) Claim next job (pending/failed and ready).  
2) Load bundle from `credential_history`.  
3) Ensure EMQX authz source; upsert user; replace ACLs with publish/subscribe topics (optionally client-bound).  
4) On success: mark job completed; credential lifecycle applied; set `mqtt_access_applied=true`.  
5) On failure: increment attempt, set next_attempt_at with exponential backoff, persist error.

## Service Account Bootstrap
- Add command (e.g., `cmd/server` init or separate CLI) to call EMQX adapter `ProvisionDevice+UpdateACL` for backend service account, using configured topics.

## Tests
- Unit: topic resolver from DNA, bundle builder, worker happy-path and failure backoff.  
- Integration/e2e: 
  - Provision device → assert bundle fields, lifecycle pending → worker applies → lifecycle applied.  
  - MQTT publish on derived topic succeeds (EMQX ACL).  
  - Restart broker → reconciliation/job retry reapplies ACL.  
  - DNA topic change → bootstrap returns updated topics.

## Risks / Mitigations
- **Schema migration downtime**: run sequential migrations with defaults; guard code to handle missing history rows initially.  
- **ACL divergence after EMQX reset**: reconciliation loop + job retry.  
- **Topic drift from DNA**: single resolver source; responses always use resolver output.

## Rollout Steps
1) Apply DB migrations.  
2) Deploy backend with new provisioning flow.  
3) Run service-account bootstrap command.  
4) Run integration/e2e suite against docker-compose integration stack.
