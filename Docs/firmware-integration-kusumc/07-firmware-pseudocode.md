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

// The server may advertise multiple publish topics (legacy + compat).
// Select based on the firmware profile you are running.
legacyTelemetryTopic = first topic in bootstrap.primary_broker.publish_topics that matches "{imei}/(heartbeat|pump|data|daq|ondemand)"
compatTelemetryTopic = first topic in bootstrap.primary_broker.publish_topics that contains "/messages/" or endsWith("/telemetry")
legacyErrorsTopic = first topic in bootstrap.primary_broker.publish_topics that endsWith("/errors") and does NOT startWith("devices/")
compatErrorsTopic = first topic in bootstrap.primary_broker.publish_topics that startsWith("devices/") and endsWith("/errors")

publishTopic = legacyTelemetryTopic or compatTelemetryTopic or first bootstrap.primary_broker.publish_topics
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

// Recommended: write-ahead log so telemetry/events are not lost during brief outages.
// Persist locally first, then attempt publish; on failure, enqueue for retry.
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
  correlation = commandPayload.correlation_id or commandPayload.msgid

  // Canonical command name is command.name; legacy key is cmd
  cmdName = commandPayload.command.name or commandPayload.cmd
  params = commandPayload.command.params or commandPayload.payload
  result = executeCommand(cmdName, params)

  response = {
    imei: IMEI,
    project_id: PROJECT_ID,
    msgid: newMsgId(),
    correlation_id: correlation,
    packet_type: "ondemand_rsp",
    timestamp: nowMillis(),
    status: result.status, // "ack" | "wait" | "failed"
    code: result.ok ? 0 : 1,
    payload: result.data,
    ts: nowMillis()
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
    if not alreadyProcessed(cmd.correlation_id):
      executeCommand(cmd)
      emitAckOrResponse(cmd)
```

## Store-and-forward (implementable template)
```text
// Goal:
// - No loss for errors; best-effort no loss for telemetry.
// - Stable msgid per event across retries (lets server/UI dedupe).

queueEntry = {
  id: string,               // stable local id, e.g., UUID
  lane: "errors"|"telemetry",
  topic: string,
  payload: object,
  created_at_ms: int64,
  attempts: int,
  next_retry_at_ms: int64
}

// Persistent storage (flash/SD)
enqueue(entry)
peekNextEligible(nowMs) -> entry?  // ordered by lane priority then created_at_ms
markSent(entryId)
markRetry(entryId, attempts, nextRetryAtMs)

// Write-ahead on every event
function publishWithWAL(lane, topic, payload):
  if payload.msgid empty:
    payload.msgid = newMsgId()   // generate once, then keep stable
  if payload.timestamp empty:
    payload.timestamp = nowMillis()

  entry = { id:newUUID(), lane, topic, payload, created_at_ms:nowMillis(), attempts:0, next_retry_at_ms:0 }
  enqueue(entry)

  if mqtt.isConnected():
    tryFlushBurst(maxMsgs=10, maxDurationMs=1500)

// Flush loop on reconnect + periodically while connected
function tryFlushBurst(maxMsgs, maxDurationMs):
  start = nowMillis()
  sent = 0
  consecutiveFails = 0

  while mqtt.isConnected() and sent < maxMsgs and (nowMillis()-start) < maxDurationMs:
    entry = peekNextEligible(nowMillis())
    if entry is null: break

    ok = mqtt.publish(topic=entry.topic, payload=json(entry.payload), qos=1)
    if ok:
      markSent(entry.id)
      sent += 1
      consecutiveFails = 0
      sleep(50ms) // throttle so live telemetry isn’t starved
    else:
      consecutiveFails += 1
      backoffMs = min(60_000, 1000 * (2 ^ min(entry.attempts, 6)))
      jitterMs = random(0..250)
      markRetry(entry.id, entry.attempts+1, nowMillis()+backoffMs+jitterMs)
      if consecutiveFails >= 3: break

// Queue policy (keep it simple):
// - Never drop errors unless storage is critically low.
// - If queue is too large, drop oldest telemetry first (not errors).
```

For concrete payload examples, see `09-device-api-samples.md`.

## 6) Publish device errors (dedicated lane) + HTTP fallback
```text
event = {
  open_id: bootstrap.identity.uuid,
  timestamp: nowMillis(),
  error_code: "RS485_CRC_ERROR",
  error_data: { slave_id: 1, count: 12, window_sec: 300 },
  severity: "warning"
}

persistLocalQueue("errors", event)

// MQTT preferred (legacy topic for legacy firmware)
errorsTopic = legacyErrorsTopic or compatErrorsTopic
ok = false
if mqtt.isConnected() and errorsTopic not empty:
  ok = mqtt.publish(topic=errorsTopic, payload=json(event), qos=1)
  if ok: markQueuedSent("errors", event)

// HTTP fallback when MQTT is down or repeatedly failing
if not ok:
  HTTP POST /api/device-open/errors?imei={IMEI}  body=json(event)
```
