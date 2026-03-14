# Device Events, Errors, and Store-and-Forward (Legacy-only)

This document defines what the device should log locally, what it should send live when connectivity exists, and what should be stored and flushed later.

## 1) Default commands sent to device (today)
The current UI→backend command pipeline publishes govt-legacy ondemand commands on `<imei>/ondemand`.

Default command set (seeded as core):
- `reboot`
- `rebootstrap`
- `set_ping_interval_sec` (param: `interval_sec`)
- `send_immediate`
- `apply_device_configuration` (params: `config_id`, `config`)

Govt legacy command shape (server → device):
```json
{
  "msgid": "<uuid>",
  "timestamp": 1760870400123,
  "type": "ondemand_cmd",
  "cmd": "set_ping_interval_sec",
  "interval_sec": 60
}
```

Govt legacy response shape (device → server) — echo msgid recommended:
```json
{
  "timestamp": 1760870400456,
  "status": "ack",
  "code": 0,
  "msgid": "<uuid>"
}
```

Command history / “chat-like” trail:
- Backend persists command requests and responses and exposes history via device-open fallback endpoints (see `04-rest-api-contract.md`).

## 2) Event taxonomy (what to detect)
Think of events as a separate stream from telemetry measurements.

Recommended categories:
- Connectivity: SIM missing, SIM locked, APN failure, no signal, no data session, DNS fail, NTP fail
- MQTT: auth rejected, ACL denied on publish, subscribe fail, repeated disconnect loops, keepalive timeouts
- HTTP: bootstrap 401/403/5xx, TLS cert validation failures
- Power: battery low, brownout/reset, watchdog reset, solar charge fault
- Storage: SD missing, SD read-only, SD I/O error, filesystem corruption, low space
- RS485/VFD: bus timeout, CRC errors, no response, framing/parity errors, VFD fault codes
- Sensors: out-of-range values, stuck sensor, calibration missing
- Time: RTC drift, time jump detected, timestamp monotonicity broken
- Firmware: config apply failed, config parse error, OTA/firmware update failed (if used)
- Security: repeated invalid commands, tamper switch (if present)

## 3) What to send live vs store and flush later
Rule of thumb:
- Send live when 4G is up and MQTT is connected.
- Always store locally first (write-ahead), then mark as sent when acked by local queue logic.

**Send live (and store):**
- RS485 communication errors (counts/rates)
- Battery low / power faults
- SD card errors (missing, I/O errors)
- MQTT auth/ACL errors (with backoff state)
- “config apply failed” and reason

**Store-first, flush-later:**
- SIM not detected / no internet / offline-only diagnostics
- Boot reasons and reset causes when no connectivity
- Long offline telemetry backlog

## 4) Storage strategy (no data loss goal)
Two-tier store:
1) **SD card (preferred):** large ring buffer for telemetry + events.
2) **Internal flash fallback:** small ring buffer for at least ~5 days (configurable) of:
   - telemetry summaries (or full telemetry if feasible)
   - events/errors at full fidelity

Principles:
- Store telemetry and events in an append-only log with rollover (ring) + checksums.
- Separate queues by priority:
  - P0: events/errors
  - P1: heartbeat/health
  - P2: data/daq telemetry
- When SD missing or unhealthy:
  - continue logging to internal flash automatically
  - raise an SD-missing event immediately when online

Suggested retention policy:
- Internal flash: keep last 5 days minimum (or last N records) — always for P0/P1, best-effort for P2.
- SD: keep as much as space allows, with daily partitioning or rolling segments.

## 5) Flush protocol (when connectivity returns)
- On reconnect, flush oldest-first, but do not starve live telemetry:
  - Send a small batch of backlog, then one live publish, repeat.
- Include per-record identifiers locally (sequence number + local timestamp) to avoid duplicates.

## 6) How to represent events on the wire (device → server)
Publish device errors and offline-rule alerts on a dedicated topic:
- `<imei>/errors`

Payload schema:
- `open_id`: device UUID from bootstrap identity (recommended)
- `timestamp`: ms epoch
- `error_code`: stable string enum (see `10-error-codes.md`)
- `error_data`: dynamic JSON object

Example:
```json
{
  "open_id": "dev-9c6b8de2",
  "timestamp": 1760870400456,
  "error_code": "RS485_CRC_ERROR",
  "error_data": {
    "bus": "rs485",
    "slave_id": 1,
    "count": 12,
    "window_sec": 300
  }
}
```

## 7) UI expectations (what should be visible)
Minimum useful UI surfaces:
- Device timeline / latest: last seen, last error code, last connectivity state
- Event log: filter by severity/category, show message + first_seen/last_seen + counts
- Storage health: SD present, SD free %, internal flash usage %, backlog size
- RS485 health: error rate, last good read timestamp, VFD fault codes (if any)
