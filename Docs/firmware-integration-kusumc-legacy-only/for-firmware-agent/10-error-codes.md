# Canonical `error_code` Enum (Legacy-only)

Firmware must emit a stable `error_code` on `<imei>/errors` (or via `POST /api/device-open/errors` when MQTT is down).

## Naming rules
- UPPER_SNAKE_CASE
- Stable once released (do not rename; deprecate instead)
- Prefer specific root causes over vague “FAIL”

## Severity guidance (default)
- `info`: expected recoverable state (e.g. temporary offline)
- `warning`: degraded behavior / retrying / partial data loss risk
- `critical`: safety risk, repeated hard failures, or device likely non-functional

## Canonical codes

### Connectivity / Time
- `SIM_MISSING`
- `SIM_LOCKED`
- `APN_AUTH_FAILED`
- `NO_SIGNAL`
- `NO_DATA_SESSION`
- `DNS_FAILED`
- `NTP_SYNC_FAILED`
- `CLOCK_JUMP_DETECTED`

### MQTT
- `MQTT_CONNECT_FAILED`
- `MQTT_AUTH_REJECTED`
- `MQTT_ACL_DENIED`
- `MQTT_SUBSCRIBE_FAILED`
- `MQTT_PUBLISH_FAILED`
- `MQTT_DISCONNECT_LOOP`

### HTTP (bootstrap / recovery)
- `HTTP_BOOTSTRAP_401`
- `HTTP_BOOTSTRAP_403`
- `HTTP_BOOTSTRAP_5XX`
- `HTTP_CREDENTIAL_REFRESH_FAILED`

### Power / Reset
- `BATTERY_LOW`
- `BROWNOUT_RESET`
- `WATCHDOG_RESET`
- `SOLAR_CHARGE_FAULT`

### Storage
- `SD_MISSING`
- `SD_READ_ONLY`
- `SD_IO_ERROR`
- `FS_CORRUPTION`
- `LOW_STORAGE`

### RS485 / VFD
- `RS485_TIMEOUT`
- `RS485_CRC_ERROR`
- `RS485_FRAMING_ERROR`
- `RS485_NO_RESPONSE`
- `VFD_FAULT`

### Firmware / Config
- `CONFIG_PARSE_ERROR`
- `CONFIG_APPLY_FAILED`
- `SENSOR_CALIBRATION_MISSING`

## `error_data` expectations
Keep `error_data` small and structured; recommended keys:
- `count`, `window_sec` (for rate/counters)
- `reason` (short string)
- `bus`, `slave_id`, `register` (RS485)
- `http_status`, `endpoint` (HTTP)
- `phase` (state machine stage)
