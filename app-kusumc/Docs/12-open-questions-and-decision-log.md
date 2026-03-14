# Open Questions and Decision Log

Date: 2026-03-04
Status: Draft for architecture review

## Decisions recorded

D-001
- Decision: Primary app transport to server is HTTPS + WSS (not default direct MQTT creds on phone)
- Rationale: Stronger control/revocation/audit posture
- Status: Proposed

D-002
- Decision: Native Android app is primary for field bridge workflows
- Rationale: BLE/BT reliability + background sync resilience
- Status: Proposed

## Open questions

Q-001
- Should mobile command writes be allowed when device is simultaneously online via broker direct path?
- Options: hard-block | queue-only | allow-with-conflict-policy

Q-002
- Maximum packet batch size and retention target for low-storage phones?

Q-003
- Do we require payload signatures from app to server in Phase 1?

Q-004
- Should app support optional direct MQTTS mode in Phase 1 or defer to Phase 2?

Q-005
- Exact BLE/BT frame contract from RMS firmware (framing, checksum, pagination, compression)?

## Pending approvals

- Security review approval for mobile impersonation model
- Product approval for native-first budget/timeline
- Ops approval for retention and monitoring thresholds

## Next review meeting agenda

1) Contract freeze
2) RBAC policy for mobile bridge
3) Conflict strategy for commands
4) Pilot rollout plan and acceptance KPIs
