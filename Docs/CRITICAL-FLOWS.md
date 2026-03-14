# Critical Flows (Cause → Effect)

## 1) Device command roundtrip
1. UI sends command to API (`/api/commands/send`).
2. Backend persists request and publishes on `<imei>/ondemand`.
3. Device responds on `<imei>/ondemand`.
4. Backend correlates response by `correlation_id/msgid`; fallback applies when absent.
5. Command status updates in DB and is visible to UI.

Failure implications:
- Missing correlation fields can cause ambiguity for concurrent outstanding commands.

## 2) Telemetry ingest and persistence
1. MQTT/HTTP telemetry enters ingest service.
2. Identity normalization resolves `imei`/topic inference.
3. Packet type inferred from payload and/or topic suffix.
4. Verification/schema checks run.
5. Packet persisted and available in history/latest endpoints.

Failure implications:
- Schema mismatch or unresolved project scope can block downstream rule evaluation.

## 3) Overflow and recovery path
1. Ingest buffer pressure exceeds threshold.
2. Packet is dead-lettered to Redis queue.
3. Counters increment for visibility.
4. Replay worker retries automatically per configured interval/batch.
5. Operators can trigger manual replay endpoint when needed.

Failure implications:
- Without replay tuning, backlog can persist under sustained pressure.

## 4) Live telemetry stream authorization
1. Client requests live token for a device.
2. Ticket stored with TTL (Redis-backed).
3. Stream endpoint validates token + device relation.
4. Stream emits live data.

Failure implications:
- If token is not validated, unauthorized stream access becomes possible.

## 5) Provisioning and broker ACL sync
1. Device create/rotate credentials triggers provisioning job.
2. Worker updates broker user + ACL.
3. Sessions may be reset to enforce credential changes.
4. Device reconnects with new credentials.

Failure implications:
- ACL drift can break command/telemetry continuity until resync completes.
