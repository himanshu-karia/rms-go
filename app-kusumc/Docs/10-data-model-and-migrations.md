# Data Model and Migration Plan (Server Support)

Date: 2026-03-04

## 1) New entities

## 1.1 mobile_clients
- id (UUID PK)
- user_id (UUID FK users)
- org_id (UUID FK organizations)
- device_fingerprint (TEXT, unique)
- device_name (TEXT)
- platform (TEXT)
- status (TEXT: pending_approval|approved|revoked)
- metadata (JSONB)
- created_at, updated_at

## 1.2 mobile_client_assignments
- id
- mobile_client_id
- project_id
- device_id (nullable if project-wide)
- created_at

## 1.3 bridge_sessions
- id
- mobile_client_id
- user_id
- project_id
- device_id
- transport (ble|bt|wifi)
- status (active|closed|expired)
- started_at
- ended_at
- metadata (JSONB)

## 1.4 mobile_ingest_batches
- id
- bridge_session_id
- total_packets
- accepted
- duplicates
- rejected
- status
- created_at
- completed_at
- metadata

## 1.5 mobile_ingest_events
- id
- batch_id
- project_id
- device_id
- idempotency_key
- status (accepted|duplicate|rejected)
- reason_code
- raw_payload (JSONB)
- created_at

## 2) Indexing recommendations

- mobile_clients(device_fingerprint)
- mobile_client_assignments(mobile_client_id, project_id, device_id)
- bridge_sessions(device_id, status, started_at desc)
- mobile_ingest_events(project_id, device_id, idempotency_key)
- unique (project_id, device_id, idempotency_key)

## 3) Migration sequencing

Migration A
- Create new tables and indexes

Migration B
- Add audit views/materialized summary as needed

Migration C
- Add retention policies for raw mobile ingest artifacts (optional)

## 4) Backfill / compatibility

- No destructive changes to existing telemetry tables
- Mobile bridge writes into existing ingest pipeline + additive metadata
- Legacy web/device flows remain unchanged

## 5) Retention policy suggestion

- mobile_ingest_events raw payload: 30-90 days (configurable)
- summarized batch records: 1 year+
- audit logs: per compliance requirement
