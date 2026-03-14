# Minimal Contract (Firmware-First)

This is the only contract firmware agent should implement for current migration.

## 1) Scope

Applies to telemetry uplink packets only:
- heartbeat (`<imei>/heartbeat`)
- data / PumpData (`<imei>/data`)
- daq (`<imei>/daq`)

OnDemand is intentionally excluded from this document.

## 2) Required fields

For every telemetry packet, firmware must provide:
- Legacy payload body fields for that packet family.
- `packet_type` (string): `heartbeat` | `pump` | `daq`.
- `msgid` (or `MSGID`) unique per device for replay window safety.
- `ts` (number): epoch milliseconds UTC.
- `IMEI` or `imei`.

## 3) Optional fields

Firmware may send but is not required to send:
- `project_id`
- `device_id`
- `protocol_id`
- `contractor_id`
- `supplier_id`
- `manufacturer_id`

Backend fills these from server-side mapping when absent.

## 4) Uniqueness and replay guidance

- `msgid` must be monotonic/unique per device stream.
- Recommended structure: `<imei>-<boot_id>-<counter>`.
- Do not reuse old counters after reset unless boot_id changes.

## 5) Time guidance

- Use `ts` as single canonical event time key.
- Keep legacy `TIMESTAMP` only for backward readability if needed.
- If both are present, they must represent the same instant.

## 6) Minimal examples

### Heartbeat
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
  "msgid": "869630050762180-b1-000123",
  "ts": 1760870400123
}
```

### PumpData (`/data`)
```json
{
  "IMEI": "869630050762180",
  "TIMESTAMP": "2026-02-28 10:16:00",
  "DATE": "2026-02-28",
  "PDKWH1": "3.42",
  "PTOTKWH1": "1523.10",
  "POPDWD1": "1140",
  "POPFREQ1": "49.8",
  "POPI1": "7.2",
  "POPV1": 229,
  "packet_type": "pump",
  "msgid": "869630050762180-b1-000124",
  "ts": 1760870460000
}
```

### Daq
```json
{
  "IMEI": "869630050762180",
  "TIMESTAMP": "2026-02-28 10:16:30",
  "DATE": "2026-02-28",
  "AI11": "12.2",
  "DI11": "1",
  "DO11": "0",
  "packet_type": "daq",
  "MSGID": "869630050762180-b1-000125",
  "ts": 1760870490000
}
```

## 7) Implementation decision

For this migration, firmware agent should implement minimal contract only.
All additional envelope fields are backend-managed enrichment.
