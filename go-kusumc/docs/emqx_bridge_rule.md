# EMQX Bridge Rule for RMS → go-kusumc

Purpose: republish RMS topics `<IMEI>/{heartbeat|data|daq|ondemand}` to ingestion topic `channels/{project_id}/messages/{imei}` while injecting required envelope keys.

## Assumptions
- `project_id` is known per device (from provisioning DB). Use EMQX resource or SQL lookup if available; otherwise set a default and override later.
- Device already sends upper-case RMS payload fields; we only add envelope fields, not transform body.
- We add `packet_type` based on the topic suffix.

## Rule SQL (conceptual)
```sql
SELECT
  payload as body,
  topic as orig_topic,
  substr(topic, 1, 15) as imei,
  case
    when topic LIKE '%/heartbeat' then 'heartbeat'
    when topic LIKE '%/data' then 'data'
    when topic LIKE '%/daq' then 'daq'
    when topic LIKE '%/ondemand' then 'ondemand_cmd'
    else 'unknown'
  end as packet_type
FROM
  "$events/client_received"
WHERE
  topic =~ '(.*)/(heartbeat|data|daq|ondemand)'
```

## Action (republish)
Transform payload to inject envelope (pseudocode):
```json
{
  "packet_type": ${packet_type},
  "project_id": ${lookup_project_id(imei)},
  "protocol_id": "rms-v1",
  "contractor_id": "",
  "supplier_id": "",
  "manufacturer_id": "",
  "device_id": ${lookup_device_id(imei)},
  "imei": ${imei},
  "ts": ${now_ms()},
  "msg_id": ${body.msgid || body.MSGID || uuid()},
  "body": ${body}
}
```
Publish to topic: `channels/${project_id}/messages/${imei}` with QoS 1.

## EMQX Console steps
1) Create a rule under Rule Engine.
2) Add an Action: "Republish" to topic `channels/${project_id}/messages/${imei}` with the transformed payload above.
3) If you need DB lookup, add a Resource (e.g., Postgres) and use a template to fetch project_id/device_id by imei.
4) Tighten ACLs to allow publishes only on `channels/+/messages/+` and commands on `channels/+/commands/+`.

## Notes
- If devices already include lower-case `imei` and envelope keys, the bridge can simply retarget the topic without payload transform.
- Keep QoS=1 to align with ingestion subscription QoS=1.
