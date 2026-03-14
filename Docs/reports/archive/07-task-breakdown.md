# Task breakdown (implementation checklist)

This is the work list to implement go-kusumc + ui-kusumc after the code folders are created.

## 0) Repo structure
- [ ] Create `go-kusumc/` by copying `unified-go/`
- [ ] Create `ui-kusumc/` by copying `old-ui-copy/version-a-frontend/`
- [ ] Rename internal identifiers (package names, docker image names, compose service names)

## 1) Backend: legacy topic support
- [ ] Subscribe to `+/{heartbeat,pump,data,daq,ondemand}`
- [ ] Implement topic→packet_type mapping
- [ ] Implement msgid normalization (`msgid|msg_id|MSGID`)
- [ ] Ensure project scope resolution for packets (if needed)

## 2) Backend: legacy command publishing
- [ ] Publish commands to `<imei>/ondemand` in legacy payload shape
- [ ] Accept responses on `<imei>/ondemand` and correlate by msgid
- [ ] Map status strings to canonical persisted statuses

## 3) Backend: provisioning and ACL templates
- [ ] Update EMQX bootstrap to apply legacy ACL
- [ ] Ensure per-device publish/subscribe topic lists match govt protocol
- [ ] Ensure rotate creds forces disconnect and still works

## 4) Backend: URL/env
- [ ] Ensure env supports public hostnames for `kusumc.hkbase.in`
- [ ] Ensure Nginx reverse proxy routes `/api/*` correctly

## 5) UI: base URL and WS URL
- [ ] Ensure API base is same-origin or env-driven
- [ ] Ensure MQTT WS URL is env-driven (if simulator used)

## 6) Tests
- [ ] Add KUSUMC integration tests for legacy MQTT ingest and ondemand correlation
- [ ] Run UI unit + E2E smoke

## 7) Documentation
- [ ] Freeze protocol docs in a KUSUMC-specific firmware folder
- [ ] Link KUSUMC docs from root readme
