# App-KUSUMC Docs Pack

Date: 2026-03-04
Scope: Android app + server support for RMS mobile operations (device bridge over BLE/BT/WiFi, command read/write, offline log upload, remote viewer).

## Document set

1. `01-app-scope-and-spec.md`
   - Functional scope and product spec for Android app
   - Modules, user journeys, non-functional requirements, acceptance criteria

2. `02-server-scope-and-backlog.md`
   - Required backend capabilities to support app workflows
   - Data model and implementation backlog

3. `03-contracts-http-mqtts-wss.md`
   - HTTPS API contracts
   - MQTTS/WSS contract design for mobile uplink/downlink
   - Payload examples and error codes

4. `04-device-bridge-offline-sync-spec.md`
   - Detailed spec for local device bridge (BLE/BT/WiFi)
   - Log collection, dedup, replay, and “publish on behalf of device” semantics

5. `05-security-identity-and-observability.md`
   - App identity model (phone enrollment, credentials, token flows)
   - Threat model, ACL approach, auditability, observability

6. `06-native-vs-hybrid-decision.md`
   - Decision framework: native Android app vs responsive hybrid/PWA extension of `ui-kusumc`
   - Recommended phased rollout

7. `07-openapi-mobile-bridge-draft.yaml`
   - Draft OpenAPI spec for mobile bridge endpoints

8. `08-delivery-plan-and-estimates.md`
   - Workstreams, milestones, staffing assumptions, and rollout strategy

9. `09-test-strategy-and-uat.md`
   - Unit/integration/E2E/security test strategy and UAT criteria

10. `10-data-model-and-migrations.md`
    - Server-side schema additions and migration sequencing

11. `11-operational-runbook.md`
    - Production monitoring, incident playbooks, emergency controls

12. `12-open-questions-and-decision-log.md`
    - Architecture decisions, unresolved questions, approval checklist

## Recommended reading order

1) App scope
2) Server scope
3) Contracts
4) Bridge sync details
5) Security/observability
6) Delivery decision
7) Delivery/test/ops execution docs

## Key decision summary

- Primary recommendation: implement a **dedicated Android app** for field operations.
- Keep `ui-kusumc` responsive improvements for supervisory use, but do not depend on hybrid/PWA for BLE/BT-heavy field workflows.
- Preferred transport from app to server: **WSS or HTTPS ingestion APIs with JWT + device impersonation scope**, instead of assigning broad MQTT broker credentials to phones.
