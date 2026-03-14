# RMS Final Plan – Control Centre, Commands, and Extensible Ingestion

## Objectives
- Provide a command & control centre with scoped command catalog (core, protocol, model, project) and per-device capabilities.
- Enable command execution with MQTT/HTTP transport, correlation, retries, and response handling.
- Keep ingestion flexible for packet extensions while allowing validation via schemas/DNA.
- Add UI flows for operators: pick device, choose command, build payload, send, observe history/ack/retry.

## Scope & Layers
- Catalog layers: core (Unified-IoT common), protocol-specific, device-model-specific, project-specific.
- Topics: commands on `channels/<project>/commands/<device>`; responses/acks on `channels/<project>/commands/<device>/resp` (or `/ack`), carrying `correlation_id`.
- DNA: continue to drive project/protocol bindings; optionally extend DNA to carry command catalogs or feature flags per project.

## Data Model (DB)
- `command_catalog`: id, name, scope (core|protocol|model|project), protocol_id?, model_id?, project_id?, payload_schema JSON, transport (mqtt|http), response_pattern_id?, created_at.
- `device_capabilities`: device_id, command_catalog_id (allowed commands per device/model/project binding).
- `project_command_overrides`: project_id, command_catalog_id, enabled bool (optional finer control).
- `command_requests`: id, device_id, project_id, command_id, payload JSON, status (queued|published|acked|failed|timeout), retries, correlation_id, created_at, published_at, completed_at.
- `command_responses`: correlation_id, device_id, project_id, raw_response JSON/text, parsed JSON, matched_pattern_id?, received_at.
- `response_patterns`: id, command_id, pattern_type (regex|jsonpath), pattern, success bool, extract JSON (field mappings), created_at.

## Services & APIs
- List commands (scoped by device/project/protocol/model), returning payload schema and transport.
- Enqueue command: validate payload against schema, create command_request, publish to MQTT/HTTP, set status=published with correlation_id.
- Response ingest: MQTT subscriber on resp topic; match correlation_id; classify via response_patterns; update command_request to acked/failed; store response.
- Retry worker: timeouts/backoff; update retries/status; stop after max retries; mark failed.
- Optional HTTP response webhook for devices that respond via HTTP.

## Ingestion Flexibility
- Ingestion already tolerates extra fields; keep schema files in `internal/config/payloadschema` and DNA per project for validation/whitelisting.
- For “strict + extendable”: update schema/DNA when new fields are allowed; ingestion won’t break on extras.

## UI (new-frontend)
- Control Centre page:
  - Device selector -> scoped command list (core + protocol + model + project).
  - Payload form auto-built from `payload_schema` (JSON schema → form).
  - Send command: show correlation_id; display status.
  - History pane: request/response timeline, status, retries; quick retry/resend.
- Chat-like view: render the same request/response stream per device; same API backend.

## MQTT/Transport Details
- Command publish: topic `channels/<project>/commands/<device>`, QoS 1; payload includes `command_id`, `correlation_id`, and validated body.
- Response/ack: topic `channels/<project>/commands/<device>/resp` (or `/ack`) with `correlation_id`, status, raw payload; backend subscriber records it.

## Testing
- Unit: payload validation, response pattern matching, state machine (queued→published→acked/failed/timeout), retry logic.
- Integration: enqueue -> publish -> simulate response on resp topic -> status=acked; missing response -> timeout/retry -> failed.
- UI: form renders from schema; send invokes API; history updates via polling or websocket.
- TLS: reuse existing strict TLS setup (localhost SAN cert) for HTTPS/MQTTS.

## Implementation Steps
1) DB migrations for catalog, capabilities, requests/responses, response_patterns.
2) Go domain/services: command catalog service, request service, response listener (MQTT), retry worker.
3) HTTP APIs: list commands, enqueue command, list history/responses.
4) MQTT subscriber for response topics; publish helper includes correlation_id.
5) Extend bootstrap (optional) to surface allowed commands per device/project.
6) Frontend Control Centre page (schema-driven forms, history, retry).
7) Integration tests for enqueue/response/retry; UI e2e smoke for send/history.

## Current State
- No control centre UI yet; MQTT command topics and ACLs exist; ingestion is schema-flexible; TLS is strict and passing all E2E tests. This plan completes RMS execution by adding catalog, history, retries, and UI.
