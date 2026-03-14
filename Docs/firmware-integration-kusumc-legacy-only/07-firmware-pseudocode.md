# Firmware Pseudocode Templates

## What changed since last refresh (2026-02-18)
- Reconfirmed pseudocode remains aligned with compatibility contract (bootstrap, self telemetry, forwarded telemetry, command response).
- Kept forwarded payload template consistent with current metadata-route requirements.
- Preserved correlation fallback semantics in command response template.
- Added HTTP command-history fallback pseudocode for MQTT interruption recovery.

## 1) Bootstrap and connect
```text
bootstrap = HTTP GET /api/bootstrap?imei={IMEI}  (header: x-api-key)
if bootstrap invalid: backoff and retry

// Notes:
// - Alias routes `/api/device-open/bootstrap`, `/api/v1/device-open/bootstrap`, `/api/devices/open/bootstrap`, and `/api/v1/devices/open/bootstrap` exist but redirect to `/api/bootstrap`.
// - In prod-like mode, call `https://<host>/api/bootstrap` (TLS via Nginx).

mqtt.connect(
  host=bootstrap.primary_broker.host,
  port=bootstrap.primary_broker.port,
  username=bootstrap.primary_broker.username,
  password=bootstrap.primary_broker.password,
  clientId=bootstrap.primary_broker.client_id
)

subscribe(commandTopic from bootstrap.primary_broker.subscribe_topics)
publishTopic = first bootstrap.primary_broker.publish_topics
```

## 2) Publish self telemetry
```text
payload = {
  imei: IMEI,
  project_id: PROJECT_ID,
  msgid: newMsgId(),
  packet_type: "heartbeat",
  timestamp: nowMillis(),
  RSSI: readRSSI(),
  BATT: readBattery(),
  TEMP: readTemp()
}

mqtt.publish(topic=publishTopic, payload=json(payload), qos=1)

// Recommended: store-first (write-ahead) then send.
// For the full store-and-forward policy, see `for-firmware-agent/09-device-events-and-storage.md`.
```

## 3) Publish forwarded telemetry (gateway mode)
```text
// Optional: pull forwarding config from server (which child nodes to forward)
nodes = HTTP GET /api/device-open/nodes?imei={GATEWAY_IMEI}

child = readFromChildNode()

payload = {
  imei: GATEWAY_IMEI,
  project_id: PROJECT_ID,
  msgid: newMsgId(),
  packet_type: "forwarded_data",
  timestamp: nowMillis(),

  TEMP: child.temp,
  FLOW: child.flow,

  metadata: {
    forwarded: true,
    origin_imei: child.imei,
    origin_node_id: child.nodeId,
    route: {
      path: child.pathPlusGateway,
      hops: child.hops,
      ingress: child.transport,
      received_at: child.rxTimestampISO
    }
  }
}

mqtt.publish(topic=publishTopic, payload=json(payload), qos=1)
```

## 4) Command consume and response
```text
onCommandMessage(commandPayload):
  // Govt legacy command shape:
  // { msgid, timestamp, type:"ondemand_cmd", cmd, <params at top-level> }
  if commandPayload.type != "ondemand_cmd":
    return

  cmdName = commandPayload.cmd
  params = commandPayload minus {"msgid","timestamp","type","cmd"}
  result = executeCommand(cmdName, params)

  // Govt legacy response shape uses status, and may omit correlation keys.
  // Strong recommendation: echo msgid from the command for deterministic correlation.
  response = {
    timestamp: nowMillis(),
    status: result.status, // "ack" | "wait" | "failed"
    code: result.status == "wait" ? 2 : (result.ok ? 0 : 1),
    msgid: commandPayload.msgid
    // plus any command-specific response keys at top level
  }

  mqtt.publish(responseTopic, json(response), qos=1)

// Special case: send_immediate
// If next periodic publish is <= 30s away, respond with code=2 and status="wait".
```

## 5) HTTP fallback for missed commands
```text
if mqttSessionWasInterrupted:
  backlog = HTTP GET /api/device-open/commands/history?imei={IMEI}&limit=20
  for each cmd in backlog.commands:
    cmdKey = cmd.correlation_id or cmd.msgid
    if not alreadyProcessed(cmdKey):
      executeCommand(cmd)
      emitAckOrResponse(cmd)
```

## 6) Publish device errors / offline-rule alerts
```text
event = {
  open_id: bootstrap.identity.uuid,
  timestamp: nowMillis(),
  error_code: "SD_MISSING",
  error_data: { reason: "no card" },
  severity: "warning"
}

persistLocalQueue("errors", event)

ok = false
if mqtt.isConnected():
  ok = mqtt.publish(topic=f"{IMEI}/errors", payload=json(event), qos=1)
  if ok: markQueuedSent("errors", event)

if not ok:
  HTTP POST /api/device-open/errors?imei={IMEI} body=json(event)
```

For concrete payload examples, see `09-device-api-samples.md`.
