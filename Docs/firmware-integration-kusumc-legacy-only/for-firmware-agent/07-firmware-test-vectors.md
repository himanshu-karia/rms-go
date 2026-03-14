# Firmware Test Vectors (Legacy-only, Full Packet Set)

Use these vectors to validate firmware against the complete MQTT packet contract in `03-mqtt-topics-and-payloads.md`.

## 1) Topic matrix

- Telemetry uplink: `<imei>/heartbeat`, `<imei>/data`, `<imei>/daq`
- PumpData uplink topic: `<imei>/data` only
- Command downlink: `<imei>/ondemand`
- Command response uplink: `<imei>/ondemand`
- Error uplink: `<imei>/errors`

## 2) Test profile constants

- `imei`: `869630050762180`
- `device_id`: `dev-3f2b9c27`
- `project_id`: `pm-kusum-solar-pump-msedcl`
- `protocol_id`: `rms-v1`
- `contractor_id`: `cont-01`
- `supplier_id`: `supp-01`
- `manufacturer_id`: `mfg-01`

## 3) Vector H1 — Full Heartbeat packet

Topic: `869630050762180/heartbeat`

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
  "VD": "1",
  "TIMESTAMP": "2026-02-25T10:00:00Z",
  "DATE": "2026-02-25",
  "IMEI": "869630050762180",
  "ASN": "ASN-001",
  "RTCDATE": "2026-02-25",
  "RTCTIME": "10:00:00",
  "LAT": "18.5204",
  "LONG": "73.8567",
  "RSSI": "-67",
  "STINTERVAL": "5",
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
  "TEMP": "31.2",
  "SIMSLOT": "1",
  "SIMCHNGCNT": "0",
  "FLASH": "1",
  "BATTST": "1",
  "VBATT": 12.6,
  "PST": 1
}
```

Expected:
- Accepted and persisted as heartbeat packet.
- `msg_id=hb-0001` deduplicates replay attempts.

## 4) Vector P1 — Full PumpData packet (data topic)

Topic: `869630050762180/data`

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
  "ts": 1760870401123,
  "msg_id": "pump-0001",
  "VD": "1",
  "TIMESTAMP": "2026-02-25T10:00:01Z",
  "DATE": "2026-02-25",
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
  "STINTERVAL": "5",
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

Expected:
- Accepted and persisted as pump packet.
- Numeric/string mixed legacy typing accepted.

## 5) Vector P2 — PumpData packet (data lane variant)

Topic: `869630050762180/data`

Payload: same schema as Vector P1, change:
- `packet_type`: `pump`
- `msg_id`: `data-0001`

Expected:
- Treated as PumpData (`packet_type=pump`) in backend storage/UI filters.

## 6) Vector D1 — Full DaqData packet

Topic: `869630050762180/daq`

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
  "ts": 1760870402123,
  "msg_id": "daq-0001",
  "VD": "1",
  "TIMESTAMP": "2026-02-25T10:00:02Z",
  "MAXINDEX": "4096",
  "INDEX": "2189",
  "LOAD": "1",
  "STINTERVAL": "5",
  "MSGID": "daq-raw-001",
  "DATE": "2026-02-25",
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

Expected:
- Accepted and persisted as DAQ packet.

## 7) Vector C1 — OnDemand command (set_ping_interval_sec)

Topic: `869630050762180/ondemand` (server → device)

```json
{
  "msgid": "cmd-0001",
  "COTP": "654321",
  "POTP": "123456",
  "timestamp": "2026-02-25T10:00:03Z",
  "type": "ondemand_cmd",
  "cmd": "set_ping_interval_sec",
  "DO1": 0,
  "interval_sec": 60
}
```

Expected device action:
- Update heartbeat interval to 60 seconds.
- Publish command response with same `msgid`.

## 8) Vector R1 — OnDemand response (ACK)

Topic: `869630050762180/ondemand` (device → server)

```json
{
  "timestamp": "2026-02-25T10:00:04Z",
  "status": "ack",
  "DO1": 0,
  "PRUNST1": "RUN",
  "msgid": "cmd-0001",
  "code": 0,
  "applied": {
    "interval_sec": 60
  }
}
```

Expected:
- Backend correlates with command transaction `cmd-0001`.
- Command status transitions to acknowledged/completed flow.

## 9) Vector R2 — OnDemand response (WAIT)

Topic: `869630050762180/ondemand` (device → server)

```json
{
  "timestamp": "2026-02-25T10:00:04Z",
  "status": "wait",
  "DO1": 0,
  "PRUNST1": "STOP",
  "msgid": "cmd-0002",
  "code": 2,
  "message": "next periodic publish is soon; skipping immediate burst"
}
```

Expected:
- Backend records non-terminal response semantics (`wait` / retry policy dependent).

## 10) Vector E1 — Error packet (`/errors`)

Topic: `869630050762180/errors`

```json
{
  "open_id": "dev-3f2b9c27",
  "timestamp": 1760870405456,
  "error_code": "SD_MISSING",
  "error_data": {
    "storage": "sd",
    "fallback": "internal_flash",
    "retention_days": 5,
    "free_kb": 0
  }
}
```

Expected:
- Error event ingested and visible to monitoring/diagnostics path.

## 11) Vector F1 — Forwarded telemetry packet (gateway mode)

Gateway topic: `<gateway_imei>/data` (example `356000000000999/data`)

```json
{
  "packet_type": "forwarded_data",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "contractor_id": "cont-01",
  "supplier_id": "supp-01",
  "manufacturer_id": "mfg-01",
  "device_id": "dev-node-111",
  "imei": "356000000000999",
  "ts": 1760870406123,
  "msg_id": "fwd-0001",
  "TEMP": 29.8,
  "FLOW": 12.1,
  "metadata": {
    "forwarded": true,
    "origin_node_id": "field-node-111",
    "origin_imei": "356000000000111",
    "route": {
      "path": ["field-node-111", "gateway-001"],
      "hops": 1,
      "ingress": "mesh"
    }
  }
}
```

Expected:
- Ingested as forwarded packet with route metadata preserved.

## 12) Validation checklist (pass criteria)

- Packet accepted on correct topic.
- Mandatory identity/time fields present (`IMEI`/`imei`, timestamp, message id).
- Packet persisted and queryable by `msgid`/`msg_id`.
- Duplicate replay with same message id is deduplicated.
- Command response correlates to request via `msgid`.
- Error packets remain isolated from regular telemetry analytics.
- Forwarded packet metadata remains intact in storage.

## 13) Useful diagram

![](../diagrams/11-send-immediate-decision.flowchart.svg)
