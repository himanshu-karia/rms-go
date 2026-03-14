# PM Kusum Solar Pump (MSEDCL) Smoke Test Guide

PowerShell-friendly steps to validate ingest and command round-trips after rollout. Replace placeholders before running.

## 0) Inputs to set
- `$base` = backend HTTP base, e.g., `http://localhost:8081`
- `$project` = `pm-kusum-solar-pump-msedcl`
- `$imei` = device IMEI (string)
- `$deviceId` = returned from provisioning
- `$protocolId`, `$contractorId`, `$supplierId`, `$manufacturerId` as needed (optional in envelope except where required)
- MQTT broker host/port/user/pass (from `/api/bootstrap` or infra)

## 1) Provision device
```powershell
$payload = @{ project_id=$project; imei=$imei; name="Test Pump"; protocol_id=$protocolId; contractor_id=$contractorId; supplier_id=$supplierId; manufacturer_id=$manufacturerId }
Invoke-RestMethod -Method Post -Uri "$base/api/devices" -ContentType "application/json" -Body ($payload | ConvertTo-Json)
```
Capture `device_id` and store in `$deviceId`.

## 2) Bootstrap
```powershell
Invoke-RestMethod -Method Get -Uri "$base/api/bootstrap?imei=$imei"
```
Confirm topics include `channels/$project/messages/$imei` and envelope keys match the schema.

## 3) Sample payloads (edit as needed)
- Common envelope keys (required): `packet_type`, `project_id`, `protocol_id`, `device_id`, `imei`, `ts`
- Optional envelope keys: `contractor_id`, `supplier_id`, `manufacturer_id`, `msg_id`

Heartbeat example:
```json
{
  "packet_type": "heartbeat",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "device_id": "<deviceId>",
  "imei": "<imei>",
  "ts": 1735410000,
  "VD": "1.0.0",
  "TIMESTAMP": "2024-12-28T12:30:00Z",
  "RSSI": -70,
  "GPS": "0",
  "TEMP": 31
}
```

Pump example:
```json
{
  "packet_type": "pump",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "device_id": "<deviceId>",
  "imei": "<imei>",
  "ts": 1735410060,
  "TIMESTAMP": "2024-12-28T12:31:00Z",
  "POPKW1": 1.8,
  "PDC1V1": 320.5,
  "PDC1I1": 4.2,
  "POPFLW1": 12.3
}
```

DAQ example:
```json
{
  "packet_type": "daq",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "device_id": "<deviceId>",
  "imei": "<imei>",
  "ts": 1735410120,
  "TIMESTAMP": "2024-12-28T12:32:00Z",
  "AI11": 221.1,
  "AI21": 220.7,
  "AI31": 219.9,
  "DO11": 1,
  "DO21": 0
}
```

On-demand command (uplink) example:
```json
{
  "packet_type": "ondemand_cmd",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "device_id": "<deviceId>",
  "imei": "<imei>",
  "ts": 1735410180,
  "msg_id": "cmd-001",
  "timestamp": "2024-12-28T12:33:00Z",
  "type": "switch",
  "cmd": "DO1",
  "DO1": 1
}
```

On-demand response (downlink) example:
```json
{
  "packet_type": "ondemand_rsp",
  "project_id": "pm-kusum-solar-pump-msedcl",
  "protocol_id": "rms-v1",
  "device_id": "<deviceId>",
  "imei": "<imei>",
  "ts": 1735410190,
  "msg_id": "cmd-001",
  "timestamp": "2024-12-28T12:33:10Z",
  "status": "ok",
  "DO1": 1,
  "PRUNST1": 1
}
```

## 4) Publish test messages to MQTT
Example using `mosquitto_pub` (adjust auth/host/port):
```powershell
$host = "localhost"
$port = 1883
$topic = "channels/$project/messages/$imei"
$hb = Get-Content "heartbeat.json" -Raw
mosquitto_pub -h $host -p $port -t $topic -m $hb
```
Repeat for pump/daq payloads. Verify backend logs show `status=verified` and no unknown fields.

## 5) Command round-trip via HTTP + MQTT
1) Create a command via API (backend publishes to command topic):
```powershell
$cmd = @{ packet_type="ondemand_cmd"; project_id=$project; protocol_id="rms-v1"; device_id=$deviceId; imei=$imei; ts=[int][double]::Parse(((Get-Date).ToUniversalTime()-[datetime]"1970-01-01").TotalSeconds.ToString("F0")); msg_id="cmd-$(Get-Random)"; type="switch"; cmd="DO1"; DO1=1 }
Invoke-RestMethod -Method Post -Uri "$base/api/commands" -ContentType "application/json" -Body ($cmd | ConvertTo-Json)
```
2) Simulate device response on MQTT (publish `ondemand_rsp` with same `msg_id` to `channels/$project/messages/$imei`).
3) Check command status:
```powershell
Invoke-RestMethod -Method Get -Uri "$base/api/commands?project_id=$project&imei=$imei"
```
Expect the `msg_id` row to move from pending to ack/ok.

## 6) Timescale verification
Query latest rows:
```powershell
psql "postgresql://tsuser:tspass@localhost:5432/iotdb" -c "select packet_type, imei, ts, payload->>'msg_id' as msg_id from telemetry where project_id = '$project' order by ts desc limit 10;"
```

## 7) Redis/hot cache (optional)
If cache is enabled, GET the hot cache endpoint (adjust path if your API exposes one) to confirm recent payloads are present.

## 8) Acceptance checklist
- [ ] `heartbeat` ingested as verified
- [ ] `pump` ingested as verified
- [ ] `daq` ingested as verified
- [ ] Command created -> message published -> response ingested -> command status updated
- [ ] Timescale rows present with correct packet_type and msg_id
- [ ] EMQX bridge/ACL enforced (no publish outside project scope)
