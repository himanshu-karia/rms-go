# Packet Examples (Govt Smallest vs Minimal vs Full Envelope)

This document gives canonical payload examples for telemetry packets only:
- heartbeat
- data (PumpData)
- daq

Use this for firmware implementation.

## 1) Naming of formats

### A) Govt Smallest
- Legacy govt packet only.
- No extra envelope fields from rms-go.

### B) Minimal (recommended rollout)
- Govt packet + minimal extra fields used by rms-go:
  - `packet_type`
  - `msgid` (or `MSGID`)
  - `ts` (epoch ms UTC)

### C) Full Envelope
- Govt packet + complete envelope fields:
  - `packet_type`, `project_id`, `protocol_id`, `contractor_id`, `supplier_id`, `manufacturer_id`, `device_id`, `imei`, `ts`, `msg_id`

## 2) Heartbeat (`<imei>/heartbeat`)

### A) Govt Smallest
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:15:30",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "RTCDATE": "2026-02-28",
  "RTCTIME": "10:15:30",
  "LAT": "18.5204",
  "LONG": "73.8567",
  "RSSI": "-71",
  "STINTERVAL": "15",
  "POTP": "123456",
  "COTP": "654321",
  "GSM": "1",
  "SIM": "1",
  "NET": "1",
  "GPRS": "1",
  "SD": "1",
  "ONLINE": "1",
  "GPS": "1",
  "GPSLOC": "1",
  "RF": "1",
  "TEMP": "33.2",
  "SIMSLOT": "1",
  "SIMCHNGCNT": "0",
  "FLASH": "1",
  "BATTST": "1",
  "VBATT": 12.5,
  "PST": 1
}
```

### B) Minimal
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:15:30",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "RSSI": "-71",
  "TEMP": "33.2",
  "VBATT": 12.5,
  "PST": 1,
  "packet_type": "heartbeat",
  "msgid": "869630050762180-b1-000001",
  "ts": 1761339330000
}
```

### C) Full Envelope
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
  "ts": 1761339330000,
  "msg_id": "hb-000001",
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:15:30",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "RSSI": "-71",
  "TEMP": "33.2",
  "VBATT": 12.5,
  "PST": 1
}
```

## 3) Data / PumpData (`<imei>/data`)

### A) Govt Smallest
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:00",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "PDKWH1": "14.2",
  "PTOTKWH1": "1298.7",
  "POPDWD1": "9200",
  "POPTOTWD1": "880000",
  "PDHR1": "5.4",
  "PTOTHR1": "4560.2",
  "POPKW1": "2.3",
  "MAXINDEX": "4096",
  "INDEX": "2188",
  "LOAD": "1",
  "STINTERVAL": "15",
  "POTP": "123456",
  "COTP": "654321",
  "PMAXFREQ1": "50",
  "PFREQLSP1": "20",
  "PFREQHSP1": "50",
  "PCNTRMODE1": "AUTO",
  "PRUNST1": "RUN",
  "POPFREQ1": "42.5",
  "POPI1": "4.1",
  "POPV1": 230,
  "PDC1V1": 410,
  "PDC1I1": "6.8",
  "PDCVOC1": "520",
  "POPFLW1": "38.2"
}
```

### B) Minimal
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:00",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "PDKWH1": "14.2",
  "PTOTKWH1": "1298.7",
  "POPDWD1": "9200",
  "POPTOTWD1": "880000",
  "POPFREQ1": "42.5",
  "POPI1": "4.1",
  "POPV1": 230,
  "packet_type": "pump",
  "msgid": "869630050762180-b1-000002",
  "ts": 1761339360000
}
```

### C) Full Envelope
```json
{
  "packet_type": "pump",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "contractor_id": "cont-01",
  "supplier_id": "supp-01",
  "manufacturer_id": "mfg-01",
  "device_id": "dev-3f2b9c27",
  "imei": "869630050762180",
  "ts": 1761339360000,
  "msg_id": "pump-000001",
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:00",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "PDKWH1": "14.2",
  "PTOTKWH1": "1298.7",
  "POPDWD1": "9200",
  "POPTOTWD1": "880000",
  "POPFREQ1": "42.5",
  "POPI1": "4.1",
  "POPV1": 230
}
```

## 4) DAQ (`<imei>/daq`)

### A) Govt Smallest
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:30",
  "MAXINDEX": "4096",
  "INDEX": "2189",
  "LOAD": "1",
  "STINTERVAL": "15",
  "MSGID": "daq-raw-0001",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "POTP": "123456",
  "COTP": "654321",
  "AI11": "12.4",
  "AI21": "6.2",
  "AI31": "0.0",
  "AI41": "1.0",
  "DI11": "1",
  "DI21": "0",
  "DI31": "1",
  "DI41": "0",
  "DO11": "1",
  "DO21": "0",
  "DO31": "0",
  "DO41": "1"
}
```

### B) Minimal
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:30",
  "MSGID": "daq-raw-0001",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "AI11": "12.4",
  "AI21": "6.2",
  "DI11": "1",
  "DO11": "1",
  "packet_type": "daq",
  "msgid": "869630050762180-b1-000003",
  "ts": 1761339390000
}
```

### C) Full Envelope
```json
{
  "packet_type": "daq",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "contractor_id": "cont-01",
  "supplier_id": "supp-01",
  "manufacturer_id": "mfg-01",
  "device_id": "dev-3f2b9c27",
  "imei": "869630050762180",
  "ts": 1761339390000,
  "msg_id": "daq-000001",
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:30",
  "MSGID": "daq-raw-0001",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "AI11": "12.4",
  "AI21": "6.2",
  "DI11": "1",
  "DO11": "1"
}
```

## 5) Scale note for 1M devices @ 15-minute telemetry cadence

Assuming each device sends heartbeat + data + daq once per 15 minutes:
- Packets per device per day: `3 * 96 = 288`
- Total packets/day for 1,000,000 devices: `288,000,000`
- Average ingest rate: about `3,333` packets/sec across 24h

Operational recommendation:
- Use Minimal mode for initial large-scale rollout to reduce payload size and firmware complexity.
- Add Full Envelope only after provisioning/bootstrap refresh loops are proven stable.

## 6) Compact topic examples (when `MQTT_COMPAT_TOPICS_ENABLED=true`)

Payload contract does not change. Only topic family changes.

### A) Channels family (project-aware)

Publish topic:
- `channels/pm-kusum-solar-pump-msedcl/messages/869630050762180/heartbeat`

Example payload (Minimal):
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:15:30",
  "IMEI": "869630050762180",
  "RSSI": "-71",
  "TEMP": "33.2",
  "VBATT": 12.5,
  "packet_type": "heartbeat",
  "msgid": "869630050762180-b1-000101",
  "ts": 1761339330000
}
```

Example payload (Full Envelope on data suffix):

Publish topic:
- `channels/pm-kusum-solar-pump-msedcl/messages/869630050762180/data`

```json
{
  "packet_type": "pump",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "contractor_id": "cont-01",
  "supplier_id": "supp-01",
  "manufacturer_id": "mfg-01",
  "device_id": "dev-3f2b9c27",
  "imei": "869630050762180",
  "ts": 1761339360000,
  "msg_id": "pump-000201",
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:00",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "PDKWH1": "14.2",
  "PTOTKWH1": "1298.7",
  "POPDWD1": "9200",
  "POPTOTWD1": "880000",
  "POPFREQ1": "42.5",
  "POPI1": "4.1",
  "POPV1": 230
}
```

### B) Devices family (device-centric)

Publish topic:
- `devices/869630050762180/telemetry/daq`

Example payload (Govt Smallest):
```json
{
  "VD": "1",
  "TIMESTAMP": "2026-02-28 10:16:30",
  "DATE": "2026-02-28",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "AI11": "12.4",
  "AI21": "6.2",
  "DI11": "1",
  "DO11": "1"
}
```

Suffix to normalized `packet_type` mapping:

| Topic suffix | Normalized packet_type |
| --- | --- |
| `heartbeat` | `heartbeat` |
| `data` | `pump` |
| `pump` | `pump` |
| `daq` | `daq` |
| `errors` | `device_error` |

Notes:
- If suffix is present (`.../heartbeat`, `.../data`, `.../daq`), backend can infer packet type from topic.
- Sending `packet_type` is still recommended for consistency across legacy and compact topic families.
- Keep one canonical topic family per fleet segment to reduce ACL/provisioning complexity.

## 7) Field policy (compulsory vs strongly recommended)

For this backend, **Govt Smallest is accepted** even when the device does **not** send:
- `msgid`
- `ts`
- `project_id`
- `device_id`

The backend can infer/fill missing identity from IMEI/topic and can fallback event time to server receive time.

### What is compulsory from device (minimum)
- Valid JSON payload
- IMEI (either in payload like `IMEI`/`imei`, or inferable from topic `<imei>/...`)
- Topic aligned to packet stream (`<imei>/heartbeat`, `<imei>/data`, `<imei>/daq`)

### What is strongly recommended from device (production quality)
- `msgid` (unique per packet; monotonic/collision-safe)
- `ts` (UTC epoch milliseconds)
- `packet_type` (when topic can be ambiguous)

### Pros, cons, outcomes

**If device sends Govt Smallest only (no `msgid`, no `ts`)**
- Pros: smallest payload, least firmware changes, fastest migration.
- Cons: weaker end-to-end dedupe/correlation on retries; event-time quality depends on payload timestamp parse or server receive time fallback.
- Outcome: ingestion works, but analytics/audit reliability is lower under retry/reorder/network jitter conditions.

**If device sends Minimal (`msgid` + `ts` + `packet_type`)**
- Pros: stronger idempotency, better tracing, accurate event-time analytics/alerts.
- Cons: slight payload and firmware complexity increase.
- Outcome: recommended default for production rollout.
