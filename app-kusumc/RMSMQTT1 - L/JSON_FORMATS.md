# JSON Formats Used in MQTT Communication

## HeartbeatData
```json
{
  "VD": "string",
  "TIMESTAMP": "string",
  "DATE": "string",
  "IMEI": "string",
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

## PumpData
```json
{
  "VD": "string",
  "TIMESTAMP": "string",
  "DATE": "string",
  "IMEI": "string",
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

## DaqData
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

## OnDemandCommand
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

## OnDemandResponse
```json
{
  "timestamp": "string",
  "status": "string",
  "DO1": 0,
  "PRUNST1": "string"
}
```
