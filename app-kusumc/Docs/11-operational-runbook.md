# Operational Runbook (App + Mobile Bridge)

Date: 2026-03-04

## 1) Day-1 checklist

- Enable mobile bridge feature flag in non-prod
- Seed test users and project/device assignments
- Validate `/api/mobile/*` health and auth
- Verify dashboards for bridge sessions and ingest outcomes

## 2) Day-2 monitoring

Watch metrics:
- active bridge sessions
- packet acceptance ratio
- duplicate ratio spikes
- rejected reason distribution
- per-client failure rates

Alert thresholds (initial)
- rejected > 5% for 10 min
- duplicate > 20% for 10 min
- auth/scope failures > baseline by 3x

Alert thresholds (OPS-03 refined)
- mobile ingest error rate > 3% for 5 min (warn), > 7% for 10 min (critical)
- command ack lag p95 > 20s for 10 min
- outbox pending queue growth > 2x baseline for 15 min
- re-bootstrap failure rate > 2% for 10 min

## 3) Incident playbooks

## 3.1 High reject rate
- Check reason_code histogram
- Validate schema/config sync for project
- Verify app version and packet source firmware version

## 3.2 Duplicate storm
- Check app retry loops / connectivity flaps
- Verify idempotency-key generation logic
- Confirm server unique index and conflict handling

## 3.3 Unauthorized bridge attempts
- Check assignment drift and revoked clients
- Rotate user tokens and revoke suspicious clients

## 3.4 Command reconciliation mismatch
- Compare command timeline by command_id/msgid
- Verify session/device mapping and local clock skew

## 3.5 Credential rotation regression
- Validate old credential rejection events in broker/auth logs
- Confirm bootstrap refresh latency and reconnect success ratio
- Verify telemetry resumes with new credentials and no duplicate storm

## 3.6 Mobile sync backlog growth
- Inspect `outbox_events` pending/failed trend
- Trigger immediate sync worker and observe retry posture
- If persistent, switch to controlled ingest mode and capture samples for root-cause

## 4.1 Replay and recovery procedure (OPS-01/02)

1. Identify affected window and project/device scope.
2. Freeze destructive maintenance operations for impacted scope.
3. Export failed/rejected payload sample and command timeline window.
4. Re-run targeted replay path using staged replay tooling.
5. Validate replay persistence via API and DB checks.
6. Re-enable normal traffic and monitor for 30 minutes.

Recovery evidence bundle:
- request IDs, session IDs, device IDs
- replay batch IDs and counts
- before/after persistence counts
- alert timeline and closure notes

## 4) Emergency controls

- Disable bridge session creation via feature flag
- Keep existing web/device direct flows active
- Optionally block specific client IDs quickly

## 5) Support artifacts

- Standard support export should include:
  - bridge_session_id
  - batch summaries
  - reject sample payloads
  - app version/build
  - device firmware version
