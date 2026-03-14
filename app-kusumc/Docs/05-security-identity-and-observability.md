# Security, Identity, and Observability Spec

Date: 2026-03-04

## 1) Identity model

Principals:
- Human user (existing auth/RBAC)
- Mobile client (enrolled phone)
- RMS device (existing device identity)

Every mobile action must carry all three dimensions where applicable:
- `actor_user_id`
- `mobile_client_id`
- `target_device_id`

## 2) Authentication and authorization

- JWT auth for app
- Refresh token rotation
- Device-scope authorization checks for bridge session creation
- Policy checks:
  - project membership
  - device assignment
  - phone approval status

## 3) Credential strategy

Preferred strategy:
- App does not receive broad EMQX broker credentials.
- App uses HTTPS/WSS JWT-based session.

Optional strategy (explicitly enabled only):
- Assign per-phone MQTT creds with strict ACL
- Immediate revocation support
- Short TTL credentials with rotation

## 4) Secure storage and key management

- Android Keystore for token and key material
- Certificate pinning for HTTPS/WSS where operationally feasible
- Rooted-device policy (warn/block by configuration)

## 5) Threat model summary

Major risks:
- Stolen phone/session token misuse
- Unauthorized device impersonation
- Payload tampering before upload
- Replay attacks on batch ingest

Controls:
- Phone enrollment + approval
- Signed bridge session token with short expiry
- Idempotency and replay window checks
- Audit every on-behalf operation
- Optional payload integrity signature (`X-Payload-Signature`)

## 6) Audit and compliance events

Server audit events required:
- `mobile.client.registered`
- `mobile.client.approved`
- `mobile.bridge.session.opened`
- `mobile.bridge.packet.ingest.accepted`
- `mobile.bridge.packet.ingest.rejected`
- `mobile.bridge.command.submitted`
- `mobile.bridge.command.result.reported`

Each event fields:
- actor user
- phone/client
- device
- project
- trace id
- timestamp

## 7) Observability requirements

Metrics:
- active bridge sessions
- packets/sec by source transport
- accept/reject/duplicate ratio
- command round-trip latency
- per-phone failure rates

Logs/tracing:
- Correlate by `trace_id`, `bridge_session_id`, `device_id`
- Structured logs for ingest reason codes

Alerts:
- High reject ratio per phone
- Excess duplicate bursts
- Repeated unauthorized scope attempts
