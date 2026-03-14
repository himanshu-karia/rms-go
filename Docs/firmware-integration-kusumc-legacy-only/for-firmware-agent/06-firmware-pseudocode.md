# Firmware Pseudocode (Legacy-only)

## Runtime constants and helpers
```text
IMEI = device.imei
PROJECT_ID = provisioned.project_id
PROTOCOL_ID = provisioned.protocol_id
CONTRACTOR_ID = provisioned.contractor_id
SUPPLIER_ID = provisioned.supplier_id
MANUFACTURER_ID = provisioned.manufacturer_id
DEVICE_ID = provisioned.device_id

function nowMs() -> int
function nowISO() -> string
function uuid4() -> string

function buildEnvelope(packetType):
  return {
    packet_type: packetType,
    project_id: PROJECT_ID,
    protocol_id: PROTOCOL_ID,
    contractor_id: CONTRACTOR_ID,
    supplier_id: SUPPLIER_ID,
    manufacturer_id: MANUFACTURER_ID,
    device_id: DEVICE_ID,
    imei: IMEI,
    ts: nowMs(),
    msg_id: uuid4()
  }
```

## Bootstrap + MQTT connect
```text
bootstrap = HTTP GET /api/bootstrap?imei={IMEI}  (header: x-api-key)

mqtt.connect(
  host=bootstrap.primary_broker.host,
  port=bootstrap.primary_broker.port,
  username=bootstrap.primary_broker.username,
  password=bootstrap.primary_broker.password,
  clientId=bootstrap.primary_broker.client_id
)

mqtt.subscribe(topic=f"{IMEI}/ondemand")

// On auth reject / not authorized:
// - retry once
// - then refresh creds via HTTP and reconnect
onMqttAuthRejected():
  sleep(2s)
  if mqtt.connect() succeeds: return

  // Prefer credential-refresh endpoint when available; else re-bootstrap.
  creds = HTTP GET /api/device-open/credentials/local?imei={IMEI}
  endpointUrl = ""
  if creds.credential.endpoints exists and len(creds.credential.endpoints) > 0:
    ep = creds.credential.endpoints[0]
    if ep.url exists and ep.url != "":
      endpointUrl = ep.url
    else:
      endpointUrl = f"{ep.protocol}://{ep.host}:{ep.port}"

  if creds not available:
    bootstrap = HTTP GET /api/bootstrap?imei={IMEI}
    endpointUrl = bootstrap.primary_broker.endpoints[0]
    username = bootstrap.primary_broker.username
    password = bootstrap.primary_broker.password
    clientId = bootstrap.primary_broker.client_id
  else:
    username = creds.credential.username
    password = creds.credential.password
    clientId = creds.credential.client_id

  mqtt.connect(url=endpointUrl, username=username, password=password, clientId=clientId)
  mqtt.subscribe(topic=f"{IMEI}/ondemand")
```

## Build full packet payloads
```text
function buildHeartbeatPayload():
  p = buildEnvelope("heartbeat")
  p.VD = "1"
  p.TIMESTAMP = nowISO()
  p.DATE = dateToday()
  p.IMEI = IMEI
  p.ASN = device.asn
  p.RTCDATE = rtc.date
  p.RTCTIME = rtc.time
  p.LAT = gps.lat
  p.LONG = gps.long
  p.RSSI = modem.rssi
  p.STINTERVAL = cfg.sendIntervalMin
  p.POTP = otp.prev
  p.COTP = otp.current
  p.GSM = modem.gsmConnected
  p.SIM = modem.simDetected
  p.NET = modem.networkAttached
  p.GPRS = modem.gprsConnected
  p.SD = storage.sdPresent
  p.ONLINE = 1
  p.GPS = gps.moduleReady
  p.GPSLOC = gps.locked
  p.RF = rf.ready
  p.TEMP = sensors.boardTempC
  p.SIMSLOT = modem.simSlot
  p.SIMCHNGCNT = modem.simChangeCount
  p.FLASH = storage.flashOk
  p.BATTST = power.batteryPresent
  p.VBATT = power.batteryV
  p.PST = power.supplyState
  return p

function buildPumpPayload():
  p = buildEnvelope("pump")
  p.VD = "1"
  p.TIMESTAMP = nowISO()
  p.DATE = dateToday()
  p.IMEI = IMEI
  p.ASN = device.asn
  p.PDKWH1 = metrics.energyTodayKwh
  p.PTOTKWH1 = metrics.energyTotalKwh
  p.POPDWD1 = metrics.waterTodayL
  p.POPTOTWD1 = metrics.waterTotalL
  p.PDHR1 = metrics.runtimeTodayHr
  p.PTOTHR1 = metrics.runtimeTotalHr
  p.POPKW1 = metrics.powerKw
  p.MAXINDEX = storage.maxIndex
  p.INDEX = storage.currentIndex
  p.LOAD = storage.loadState
  p.STINTERVAL = cfg.sendIntervalMin
  p.POTP = otp.prev
  p.COTP = otp.current
  p.PMAXFREQ1 = vfd.maxFreqHz
  p.PFREQLSP1 = vfd.lowFreqHz
  p.PFREQHSP1 = vfd.highFreqHz
  p.PCNTRMODE1 = vfd.controlMode
  p.PRUNST1 = vfd.runState
  p.POPFREQ1 = vfd.outputFreqHz
  p.POPI1 = vfd.outputCurrentA
  p.POPV1 = vfd.outputVoltageV
  p.PDC1V1 = pv.inputVoltageV
  p.PDC1I1 = pv.inputCurrentA
  p.PDCVOC1 = pv.openCircuitVoltageV
  p.POPFLW1 = metrics.flowLpm
  return p

function buildDaqPayload():
  p = buildEnvelope("daq")
  p.VD = "1"
  p.TIMESTAMP = nowISO()
  p.MAXINDEX = storage.maxIndex
  p.INDEX = storage.currentIndex
  p.LOAD = storage.loadState
  p.STINTERVAL = cfg.sendIntervalMin
  p.MSGID = uuid4()
  p.DATE = dateToday()
  p.IMEI = IMEI
  p.ASN = device.asn
  p.POTP = otp.prev
  p.COTP = otp.current
  p.AI11 = io.ai1
  p.AI21 = io.ai2
  p.AI31 = io.ai3
  p.AI41 = io.ai4
  p.DI11 = io.di1
  p.DI21 = io.di2
  p.DI31 = io.di3
  p.DI41 = io.di4
  p.DO11 = io.do1
  p.DO21 = io.do2
  p.DO31 = io.do3
  p.DO41 = io.do4
  return p
```

## Publish telemetry packets
```text
heartbeat = buildHeartbeatPayload()
pump = buildPumpPayload()
daq = buildDaqPayload()

mqtt.publish(topic=f"{IMEI}/heartbeat", payload=json(heartbeat), qos=1)
mqtt.publish(topic=f"{IMEI}/data", payload=json(pump), qos=1)  // PumpData uplink topic: <imei>/data only
mqtt.publish(topic=f"{IMEI}/daq", payload=json(daq), qos=1)

// If publish is rejected (ACL/not authorized), treat it like auth rejection and refresh creds.
```

## Publish device errors / offline-rule alerts (store-first)
```text
// Codes: see `10-error-codes.md`
event = {
  open_id: bootstrap.identity.uuid,
  timestamp: nowMs(),
  error_code: "MQTT_AUTH_REJECTED",
  error_data: { count: 3, window_sec: 60 },
  severity: "warning"
}

persistLocalQueue("errors", event)

ok = false
if mqtt.isConnected():
  ok = mqtt.publish(topic=f"{IMEI}/errors", payload=json(event), qos=1)
  if ok: markQueuedSent("errors", event)

// HTTP fallback when MQTT is down or repeatedly failing
if not ok:
  HTTP POST /api/device-open/errors?imei={IMEI} body=json(event)
```

## Forwarded telemetry (gateway mode)
```text
function buildForwardedPacket(originNode, route, telemetry):
  p = buildEnvelope("forwarded_data")
  p.imei = gateway.imei                // gateway identity topic owner
  p.msgid = uuid4()                    // keep legacy alias too
  p.timestamp = nowMs()                // keep legacy alias too
  p.metadata = {
    forwarded: true,
    origin_node_id: originNode.id,
    origin_imei: originNode.imei,
    route: {
      path: route.path,
      hops: route.hops,
      ingress: route.ingress
    }
  }
  merge telemetry into p
  return p

fwd = buildForwardedPacket(node, route, { TEMP: 29.8, FLOW: 12.1 })
mqtt.publish(topic=f"{gateway.imei}/data", payload=json(fwd), qos=1)
```

## Optional: VFD / RS485 read loop
```text
vfd = HTTP GET /api/device-open/vfd?imei={IMEI}
rs485.configure(
  baud=vfd.rs485.baud,
  parity=vfd.rs485.parity,
  stopBits=vfd.rs485.stop_bits,
  slaveId=vfd.rs485.slave_id
)

every 10s:
  regs = rs485.readHoldingRegisters(addresses from vfd.realtime_parameters)
  pump = buildPumpPayload()
  merge mapRegsToTelemetryKeys(regs, vfd.realtime_parameters) into pump
  mqtt.publish(topic=f"{IMEI}/data", payload=json(pump), qos=1)
```

## Consume ondemand command + respond (full correlation)
```text
onMessage(topic, payload):
  if topic != f"{IMEI}/ondemand": return
  if payload.type != "ondemand_cmd": return

  cmdName = payload.cmd
  params = payload minus {msgid, timestamp, type, cmd}

  result = execute(cmdName, params)

  response = {
    timestamp: nowISO(),
    status: result.status,     // ack|wait|failed
    DO1: result.do1,
    PRUNST1: result.runState,
    msgid: payload.msgid,      // echo for deterministic correlation
    code: result.code,
    applied: result.applied
  }

  // add any command-specific output keys at top-level
  mqtt.publish(topic=f"{IMEI}/ondemand", payload=json(response), qos=1)
```

## HTTP fallback for missed commands
```text
if mqttSessionWasInterrupted:
  backlog = HTTP GET /api/device-open/commands/history?imei={IMEI}&limit=20
  for cmd in backlog.commands:
    if not alreadyProcessed(cmd.msgid):
      execute(cmd)
      emitResponse(cmd)
```

## Validation mapping
```text
Verify against 07-firmware-test-vectors.md:
- H1 => buildHeartbeatPayload + publish <imei>/heartbeat
- P1/P2 => buildPumpPayload + publish <imei>/data
- D1 => buildDaqPayload + publish <imei>/daq
- C1/R1/R2 => ondemand command/response flow
- E1 => publish <imei>/errors
- F1 => buildForwardedPacket + publish <gateway_imei>/data
```
