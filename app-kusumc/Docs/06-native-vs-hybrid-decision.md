# Decision Brief: Native Android App vs Hybrid/PWA (`ui-kusumc` extension)

Date: 2026-03-04

## 1) Decision question

Should we build a new Android app, or extend `ui-kusumc` with responsive UX and package it as hybrid/PWA?

## 2) Evaluation criteria

- BLE/BT hardware access reliability
- Background sync reliability
- Offline storage scale and resilience
- Local transport performance to device
- Security controls for field workflows
- Delivery speed and maintainability

## 3) Option A — Dedicated native Android app

Pros:
- Best BLE/BT/WiFi local transport support
- Better offline queue and background job control
- Stronger device-level security integration (Keystore, biometric options)
- Better UX for field workflows and unstable connectivity

Cons:
- Separate codebase and release pipeline
- Higher initial build cost

Fit for your requirement:
- Strong fit (especially offline log extraction + local command bridge)

## 4) Option B — Hybrid app / PWA from `ui-kusumc`

Pros:
- Faster initial UI reuse
- Single web codebase leverage

Cons:
- BLE/BT support is inconsistent across OEM devices and WebView/PWA runtime modes
- Background sync constraints on Android can hurt large log uploads
- Limited low-level transport control and recoverability

Fit for your requirement:
- Good for supervisory dashboards
- Weak for heavy field bridge flows

## 5) Recommendation

Recommended path: **Dual-track**
1) Build **native Android app** for field operations (bridge, command local IO, offline sync).
2) Keep enhancing `ui-kusumc` responsive layout for supervisory and admin use, and optionally package as PWA for read-mostly workflows.

## 6) Practical rollout plan

Phase 1 (now)
- Approve server contracts and mobile identity model
- Build Android MVP for bridge + sync

Phase 2
- Add advanced command workflows and reconciliation UX
- Expose ops diagnostics in web UI

Phase 3
- Evaluate if some app pages can be web-embedded where no hardware access is needed

## 7) Go/No-Go checklist for native app

Go when all true:
- Need BLE/BT robustly across field devices
- Need high-volume offline upload and resume
- Need deterministic background sync and local queueing

No-Go (hybrid-first) only if all true:
- Device transport is mostly network API, not BLE/BT
- Offline volume is low
- Field operations are read-heavy with minimal local command bridge

## 8) Current conclusion for RMS use case

For your described RMS workflows (read/write commands locally + fetch offline logs via BLE/BT/WiFi + upload on behalf of device), a **new native Android app is the correct primary approach**.
