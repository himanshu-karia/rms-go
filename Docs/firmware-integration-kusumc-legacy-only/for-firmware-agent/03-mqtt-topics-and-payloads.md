# MQTT Topics and Payloads (Legacy-only, Full Packet Contract)

This file is the complete firmware-facing MQTT contract derived from reference RMS JSON topic docs and aligned to current legacy behavior.

## 1) Topic Model

### 1.1 Legacy per-IMEI topics (device-native)
Device topic prefix is the device IMEI.

Pattern:
- `<imei>/heartbeat`
- `<imei>/data` (PumpData payload family)
- `<imei>/daq`
- `<imei>/ondemand` (commands + responses)
- `<imei>/errors` (error/alert channel)

Compatibility note:
- Backend may still accept legacy `<imei>/pump` traffic for backward compatibility, but firmware contract remains: PumpData uplink topic: `<imei>/data` only.

Example:
- `869630050762180/heartbeat`
- `869630050762180/data`
- `869630050762180/daq`
- `869630050762180/ondemand`
- `869630050762180/errors`

### 1.2 Unified server topic layout (gateway/platform side)
Where applicable in unified broker routing:
- Telemetry uplink: `channels/{project_id}/messages/{imei}`
- Command downlink: `channels/{project_id}/commands/{imei}`
- Command response/ack uplink: via messages path (or mapped by broker rule)

Firmware should continue publishing native legacy topics unless instructed otherwise for a deployment.

## 2) Unified Envelope (recommended for all packets)

All packets should include the fixed envelope and then packet-specific fields.

```json
{
  "packet_type": "heartbeat|pump|daq|ondemand_cmd|ondemand_rsp",
  "project_id": "<assigned by server>",
  "protocol_id": "<rms protocol code>",
  "contractor_id": "<org/contractor>",
  "supplier_id": "<supplier>",
  "manufacturer_id": "<rms manufacturer>",
  "device_id": "<uuid assigned by server>",
  "imei": "<device imei>",
  "ts": "<iso8601 or epoch ms>",
  "msg_id": "<unique message id>"
}
```

## 3) Fixed Envelope Parameter Dictionary

| Key | Description | Notes/Unit |
|---|---|---|
| `packet_type` | Logical packet discriminator | `heartbeat|pump|daq|ondemand_cmd|ondemand_rsp` |
| `project_id` | Provisioned project identifier | String |
| `protocol_id` | RMS protocol family/version | String |
| `contractor_id` | Contractor/partner id | String |
| `supplier_id` | Supplier id | String |
| `manufacturer_id` | Manufacturer id | String |
| `device_id` | UUID assigned by server | String |
| `imei` | Device IMEI | String |
| `ts` | Event time | ISO8601 or epoch ms |
| `msg_id` | Message id for dedup/tracing | String |

### 3.1 Timestamp semantics (important)
- Analytics/storage time uses packet event time from payload (`ts`, or `timestamp`, or `TIMESTAMP`).
- Backend ingest time is still captured separately as metadata (`ingested_at`) for live/operational diagnostics.
- Firmware should send UTC time and prefer epoch milliseconds in `ts`.

## 4) Packet Definitions

### 4.1 HeartbeatData

#### 4.1.1 JSON shape
```json
{
  "VD": "string",
  "TIMESTAMP": "string",
  "DATE": "string",
  "IMEI": "string",
  "ASN": "string",
  "RTCDATE": "string",
  "RTCTIME": "string",
  "LAT": "string",
  "LONG": "string",
  "RSSI": "string",
  "STINTERVAL": "string",
  "POTP": "string",
  "COTP": "string",
  "GSM": "string",
  "SIM": "string",
  "NET": "string",
  "GPRS": "string",
  "SD": "string",
  "ONLINE": "string",
  "GPS": "string",
  "GPSLOC": "string",
  "RF": "string",
  "TEMP": "string",
  "SIMSLOT": "string",
  "SIMCHNGCNT": "string",
  "FLASH": "string",
  "BATTST": "string",
  "VBATT": 0.0,
  "PST": 0
}
```

#### 4.1.2 Parameter details
| Key | Description | Unit |
|---|---|---|
| `VD` | Virtual Device Index/Group | N/A |
| `TIMESTAMP` | RTC timestamp | N/A |
| `DATE` | Local storage date | N/A |
| `IMEI` | IMEI | N/A |
| `ASN` | Application serial number | N/A |
| `RTCDATE` | RTC date | N/A |
| `RTCTIME` | RTC time | N/A |
| `LAT` | Latitude | Degrees |
| `LONG` | Longitude | Degrees |
| `RSSI` | Signal strength | N/A |
| `STINTERVAL` | Periodic interval | Minutes |
| `POTP` | Previous OTP | N/A |
| `COTP` | Current OTP | N/A |
| `GSM` | GSM connected | N/A |
| `SIM` | SIM detected | N/A |
| `NET` | Device in network | N/A |
| `GPRS` | GPRS connected | N/A |
| `SD` | SD card detected | N/A |
| `ONLINE` | Device online | N/A |
| `GPS` | GPS module status | N/A |
| `GPSLOC` | GPS lock status | N/A |
| `RF` | RF module status | N/A |
| `TEMP` | Device temperature | Celsius |
| `SIMSLOT` | SIM slot | N/A |
| `SIMCHNGCNT` | SIM change count | N/A |
| `FLASH` | Flash status | N/A |
| `BATTST` | Battery input status | N/A |
| `VBATT` | Battery voltage | V |
| `PST` | Power supply status | N/A |

### 4.2 PumpData (`/data`)

#### 4.2.1 JSON shape
```json
{
  "VD": "string",
  "TIMESTAMP": "string",
  "DATE": "string",
  "IMEI": "string",
  "ASN": "string",
  "PDKWH1": "string",
  "PTOTKWH1": "string",
  "POPDWD1": "string",
  "POPTOTWD1": "string",
  "PDHR1": "string",
  "PTOTHR1": "string",
  "POPKW1": "string",
  "MAXINDEX": "string",
  "INDEX": "string",
  "LOAD": "string",
  "STINTERVAL": "string",
  "POTP": "string",
  "COTP": "string",
  "PMAXFREQ1": "string",
  "PFREQLSP1": "string",
  "PFREQHSP1": "string",
  "PCNTRMODE1": "string",
  "PRUNST1": "string",
  "POPFREQ1": "string",
  "POPI1": "string",
  "POPV1": 0,
  "PDC1V1": 0,
  "PDC1I1": "string",
  "PDCVOC1": "string",
  "POPFLW1": "string"
}
```

#### 4.2.2 Parameter details
| Key | Description | Unit |
|---|---|---|
| `VD` | Virtual Device Index/Group | N/A |
| `TIMESTAMP` | RTC timestamp | N/A |
| `DATE` | Local storage date | N/A |
| `IMEI` | IMEI | N/A |
| `ASN` | Application serial number | N/A |
| `PDKWH1` | Today generated energy | KWH |
| `PTOTKWH1` | Cumulative generated energy | KWH |
| `POPDWD1` | Daily water discharge | Litres |
| `POPTOTWD1` | Total water discharge | Litres |
| `PDHR1` | Pump day run hours | Hrs |
| `PTOTHR1` | Pump cumulative run hours | Hrs |
| `POPKW1` | Output active power | KW |
| `MAXINDEX` | Max local storage index | N/A |
| `INDEX` | Local storage index | N/A |
| `LOAD` | Local storage load status | N/A |
| `STINTERVAL` | Periodic interval | Minutes |
| `POTP` | Previous OTP | N/A |
| `COTP` | Current OTP | N/A |
| `PMAXFREQ1` | Maximum frequency | Hz |
| `PFREQLSP1` | Lower limit frequency | Hz |
| `PFREQHSP1` | Upper limit frequency | Hz |
| `PCNTRMODE1` | Control mode status | N/A |
| `PRUNST1` | Run status | N/A |
| `POPFREQ1` | Output frequency | Hz |
| `POPI1` | Output current | A |
| `POPV1` | Output voltage | V |
| `PDC1V1` | DC input voltage | DC V |
| `PDC1I1` | DC current | DC I |
| `PDCVOC1` | DC open-circuit voltage | DC V |
| `POPFLW1` | Flow speed | LPM |

### 4.3 DaqData (`/daq`)

#### 4.3.1 JSON shape
```json
{
  "VD": "string",
  "TIMESTAMP": "string",
  "MAXINDEX": "string",
  "INDEX": "string",
  "LOAD": "string",
  "STINTERVAL": "string",
  "MSGID": "string",
  "DATE": "string",
  "IMEI": "string",
  "ASN": "string",
  "POTP": "string",
  "COTP": "string",
  "AI11": "string",
  "AI21": "string",
  "AI31": "string",
  "AI41": "string",
  "DI11": "string",
  "DI21": "string",
  "DI31": "string",
  "DI41": "string",
  "DO11": "string",
  "DO21": "string",
  "DO31": "string",
  "DO41": "string"
}
```

#### 4.3.2 Parameter details
| Key | Description | Unit |
|---|---|---|
| `VD` | Virtual Device Index/Group | N/A |
| `TIMESTAMP` | RTC timestamp | N/A |
| `MAXINDEX` | Max local storage index | N/A |
| `INDEX` | Local storage index | N/A |
| `LOAD` | Local storage load status | N/A |
| `STINTERVAL` | Periodic interval | Minutes |
| `MSGID` | Message transaction id | N/A |
| `DATE` | Local storage date | N/A |
| `IMEI` | IMEI | N/A |
| `ASN` | Application serial number | N/A |
| `POTP` | Previous OTP | N/A |
| `COTP` | Current OTP | N/A |
| `AI11` | Analog input 1 | N/A |
| `AI21` | Analog input 2 | N/A |
| `AI31` | Analog input 3 | N/A |
| `AI41` | Analog input 4 | N/A |
| `DI11` | Digital input 1 | N/A |
| `DI21` | Digital input 2 | N/A |
| `DI31` | Digital input 3 | N/A |
| `DI41` | Digital input 4 | N/A |
| `DO11` | Digital output 1 | N/A |
| `DO21` | Digital output 2 | N/A |
| `DO31` | Digital output 3 | N/A |
| `DO41` | Digital output 4 | N/A |

### 4.4 OnDemandCommand (`/ondemand`, server → device)

#### 4.4.1 JSON shape
```json
{
  "msgid": "string",
  "COTP": "string",
  "POTP": "string",
  "timestamp": "string",
  "type": "string",
  "cmd": "string",
  "DO1": 0
}
```

#### 4.4.2 Parameter details
| Key | Description | Unit |
|---|---|---|
| `msgid` | Message transaction id | N/A |
| `COTP` | Current OTP | N/A |
| `POTP` | Previous OTP | N/A |
| `timestamp` | Timestamp | N/A |
| `type` | Message type | N/A |
| `cmd` | Command type | N/A |
| `DO1` | Digital output 1 (pump control) | N/A |

### 4.5 OnDemandResponse (`/ondemand`, device → server)

#### 4.5.1 JSON shape
```json
{
  "timestamp": "string",
  "status": "string",
  "DO1": 0,
  "PRUNST1": "string"
}
```

#### 4.5.2 Parameter details
| Key | Description | Unit |
|---|---|---|
| `timestamp` | Command timestamp | N/A |
| `status` | Command status | N/A |
| `DO1` | Digital output 1 (pump control) | Status |
| `PRUNST1` | Pump run status | Status |

## 5) Error / Alert Packet (`<imei>/errors`)

Use this for device errors and offline-rule alert forwarding.

```json
{
  "open_id": "dev-9c6b8de2",
  "timestamp": 1760870400456,
  "error_code": "SD_MISSING",
  "error_data": {
    "storage": "sd",
    "fallback": "internal_flash",
    "retention_days": 5
  }
}
```

Fields:
- `open_id`: device UUID from bootstrap identity (recommended)
- `timestamp`: epoch ms
- `error_code`: stable string enum
- `error_data`: dynamic JSON object with context

## 6) Forwarded Telemetry (Gateway / mesh)

Publish on gateway identity topic (typically `<gateway_imei>/data`) and include routing metadata.

Required metadata for forwarded packets:
- `metadata.forwarded = true`
- `metadata.origin_node_id` (preferred) or `metadata.origin_imei`
- `metadata.route.path` (array)
- `metadata.route.hops` (int)
- `metadata.route.ingress` (string)

Example:
```json
{
  "imei": "356000000000999",
  "project_id": "proj_alpha",
  "msgid": "fwd-001",
  "packet_type": "forwarded_data",
  "timestamp": 1760870400123,
  "TEMP": 29.8,
  "FLOW": 12.1,
  "metadata": {
    "forwarded": true,
    "origin_node_id": "field-node-111",
    "route": {
      "path": ["field-node-111", "gateway-001"],
      "hops": 1,
      "ingress": "mesh"
    }
  }
}
```

## 7) Storage Guidance (implementation note)

- **Single hypertable**: all packet types in one table keyed by `packet_type`, `project_id`, `device_id`, `ts`, `msg_id`.
- **Per-topic hypertable**: one table per packet family (`heartbeat`, `pump`, `daq`, `ondemand`).


Use whichever is configured in your backend deployment; payload contract remains the same.

## 8) Quick Rules for Firmware

- Always include IMEI identity (`IMEI` and/or `imei`) and message id (`msgid` or `msg_id`).
- Preserve field names exactly as listed for legacy compatibility.
- PumpData uplink topic: `<imei>/data` only.
- For commands, consume and publish on `<imei>/ondemand`.
- Keep publish cadence conservative to avoid broker/API throttle side effects in constrained environments.
