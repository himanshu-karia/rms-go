# RMS-Go Handover Readiness Checklist

Last updated: 2026-03-01

Use this checklist as the final pre-handover gate for `rms-go/`.

## 1) Scope and boundaries
- [ ] Canonical workspace confirmed: `rms-go/go-kusumc`, `rms-go/ui-kusumc`, `rms-go/Docs`
- [ ] Non-canonical sibling folders marked as reference-only (`unified-go`, `new-frontend`, `refer-rms-deploy`)
- [ ] `Docs/HANDOVER-CANONICAL-INDEX.md` reflects active handover set

## 2) Runtime and compose behavior
- [ ] Core RMS stack starts from `docker-compose.yml` without LoRaWAN profile
- [ ] Optional LoRaWAN stack starts only with `--profile lorawan`
- [ ] Integration stack (`docker-compose.integration.yml`) remains ChirpStack-independent
- [ ] Compose lifecycle helpers exist for both PowerShell and Bash (`up/down core` + `up/down lorawan`)

## 3) Contracts and compatibility policy
- [ ] Legacy firmware path treated as normative for handover (`firmware-integration-kusumc-legacy-only/for-firmware-agent/*`)
- [ ] MQTT topic/payload contract doc is discoverable from canonical index
- [ ] REST API contract doc is discoverable from canonical index
- [ ] Payload/bootstrap backend contracts are linked (`go-kusumc/docs/payload-contract.md`, `go-kusumc/docs/mqtt-bootstrap-contract.md`)

## 4) Validation evidence
- [ ] Latest project status reflects current health and test baseline (`Docs/PROJECT-STATUS.md`)
- [ ] At least one recent integration run evidence is linked (`Docs/e2e-review-rms-go-2026-02-26.md` or newer)
- [ ] Govt protocol verification evidence is linked (`Docs/govt-protocol-verification-kusumc.md`)
- [ ] Inventory/parity verification evidence is linked (`Docs/rms-go-inventory-parity-verification.md`)

## 5) Diagram and architecture hygiene
- [ ] Mermaid sources (`.mmd`) and SVG exports (`.svg`) exist for active architecture/flow diagrams
- [ ] Diagram refresh process is documented (`Docs/DIAGRAM-BUILD-REFRESH-GUIDE.md`)
- [ ] Diagram owners and refresh trigger are identified for next release cycle

## 6) Handover package contents
- [ ] Entry docs included: `Docs/HANDOVER-CANONICAL-INDEX.md`, `Docs/PROJECT-STATUS.md`, `go-kusumc/README.md`, `go-kusumc/scripts/README.md`
- [ ] Contract traceability matrix included (`Docs/CONTRACT-TRACEABILITY-MATRIX.md`)
- [ ] Open risks and caveats documented (`Docs/08-risks-and-guardrails.md`)
- [ ] Ownership and first-week action list assigned to receiving team

## Signoff
- Handed over by: ____________________
- Received by: ____________________
- Date: ____________________
- Notes: ____________________
