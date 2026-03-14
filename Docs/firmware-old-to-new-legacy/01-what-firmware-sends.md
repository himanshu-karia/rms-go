# What Firmware Sends (Decision Doc)

This document defines exactly what firmware should send in each supported mode.

## 1) Mode A (Recommended): Legacy + Minimal Extra

### Send this
- Full legacy payload body (heartbeat/data/daq) unchanged.
- Extra keys:
  - `packet_type`
  - `msgid` (or `MSGID`)
  - `ts` (epoch ms UTC)
  - `imei` (optional if `IMEI` already present)

### Do not force on firmware in this mode
- `project_id`
- `device_id`
- `protocol_id`
- `contractor_id`
- `supplier_id`
- `manufacturer_id`

These are enriched server-side from device/project mappings.

## 2) Mode B (Supported): Legacy + Full Envelope

### Send this
- Full legacy payload body.
- Full envelope keys:
  - `packet_type`
  - `project_id`
  - `protocol_id`
  - `contractor_id`
  - `supplier_id`
  - `manufacturer_id`
  - `device_id`
  - `imei`
  - `ts`
  - `msg_id` (or `msgid`)

### Requirement for Mode B
- Device must refresh identity/config context from server bootstrap and credential APIs when assigned values change.

## 3) Example (Heartbeat)

### Mode A (minimal extra)
```json
{
  "IMEI": "869630050762180",
  "TIMESTAMP": "2026-02-28 10:15:30",
  "DATE": "2026-02-28",
  "RSSI": "-72",
  "TEMP": "34.1",
  "VBATT": 12.4,
  "PST": 1,
  "packet_type": "heartbeat",
  "msgid": "869630050762180-000123",
  "ts": 1760870400123
}
```

### Mode B (full envelope)
```json
{
  "packet_type": "heartbeat",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "contractor_id": "cont-01",
  "supplier_id": "supp-01",
  "manufacturer_id": "mfg-01",
  "device_id": "dev-3f2b9c27",
  "imei": "869630050762180",
  "ts": 1760870400123,
  "msg_id": "hb-0001",
  "IMEI": "869630050762180",
  "TIMESTAMP": "2026-02-28 10:15:30",
  "DATE": "2026-02-28",
  "RSSI": "-72",
  "TEMP": "34.1",
  "VBATT": 12.4,
  "PST": 1
}
```

## 4) Firmware Team Directive

For current rollout, implement Mode A only.
- Keep code path simple.
- Avoid dependency on many dynamic envelope fields.
- Let backend resolve and enrich project/device/business identifiers.
