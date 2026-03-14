# Risks & guardrails (KUSUMC product line)

## Top risks

### 1) Fork drift
Risk: go-kusumc and unified-go diverge; fixes don’t propagate.
Guardrail:
- define a backport policy (security fixes always; others case-by-case).

### 2) Broker ACL mismatch
Risk: devices can’t publish/subscribe on legacy topics after cutover.
Guardrail:
- add automated tests that validate ACL by attempting publish/subscribe.

### 3) Public URL mismatch
Risk: credentials advertise wrong host/port after deploying to kusumc.hkbase.in.
Guardrail:
- smoke test bootstrap/credential endpoints in CI using production-like env.

### 4) Mixed payload keys
Risk: govt payloads have mixed casing/legacy key variants.
Guardrail:
- normalization layer in ingestion (msgid variants; timestamp variants).

### 5) Long-term support burden
Risk: RMS becomes a forever-maintained legacy product line.
Guardrail:
- keep scope minimal; avoid importing platform-only features.

## Non-goals as guardrails
- Do not add channels topic model to KUSUMC.
- Do not add forwarded/routed packet complexity.
- Do not add multi-project platform features.
