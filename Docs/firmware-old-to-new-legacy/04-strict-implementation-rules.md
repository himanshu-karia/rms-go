# Strict Firmware Implementation Rules

This checklist is intended for direct firmware implementation and test sign-off.

## 1) Mandatory topic lanes

- Heartbeat: `<imei>/heartbeat`
- PumpData: `<imei>/data` (canonical)
- DAQ: `<imei>/daq`

Do not publish PumpData on `<imei>/pump` for new firmware.

## 2) Mandatory fields (minimal mode)

For every telemetry packet (heartbeat/data/daq), firmware must send:
- `IMEI` or `imei`
- `packet_type`
- `msgid` (or `MSGID`)
- `ts` (epoch milliseconds UTC)
- Legacy packet-family payload keys

## 3) msgid rules

- Must be unique per device stream.
- Recommended pattern: `<imei>-<boot_id>-<counter>`.
- Counter must be monotonic within a boot.
- If device reboots, change `boot_id` to avoid reusing old IDs.
- Never use wall-clock-only `msgid` as sole uniqueness source.

## 4) Time rules

- Use `ts` as canonical event time key.
- Keep `TIMESTAMP` only for legacy readability if needed.
- If both `ts` and `TIMESTAMP` are present, they must represent the same moment.
- Always send UTC.
- Avoid sending multiple conflicting time fields.

## 5) packet_type rules

- Always send explicit `packet_type`:
  - heartbeat packet: `heartbeat`
  - data packet: `pump`
  - daq packet: `daq`
- Do not rely only on topic inference in production firmware.

## 6) Type and key stability

- Preserve legacy key names exactly (`TIMESTAMP`, `IMEI`, etc.).
- Keep value types stable across firmware versions.
- Avoid adding random top-level keys not agreed in contract.

## 7) Retry/replay behavior

- If retransmitting same physical packet, keep same `msgid`.
- If generating a new packet, generate a new `msgid`.
- Use bounded retry counts to avoid replay storms.

## 8) Throughput-safe publishing

- Keep nominal 15-minute cadence unless server instructs otherwise.
- Add random jitter (few seconds) per device around interval boundaries to reduce synchronized spikes.
- Use QoS/ack strategy consistent with deployment profile.

## 9) Validation checklist before release

- Heartbeat packet accepted and visible in history.
- Data packet accepted as `packet_type=pump`.
- DAQ packet accepted as `packet_type=daq`.
- Duplicate replay with same `msgid` inside short window is suppressed.
- No conflicting `ts` vs `TIMESTAMP` values.

## 10) Escalation rule

If firmware cannot provide reliable `msgid` + `ts`, stop rollout and fix firmware first.
Backend-side inference cannot safely replace poor identity/time quality at scale.
