# MQTT Topics Used

## Topic Prefix
- All topics use the prefix: `869630050762180` (IMEI, configurable)
- Example: `869630050762180/heartbeat`

## Subscribed Topics
| Topic Suffix | Full Example Topic                | Data Type         | Direction   |
|--------------|-----------------------------------|-------------------|-------------|
| /heartbeat   | 869630050762180/heartbeat         | HeartbeatData     | Subscribe   |
| /pump        | 869630050762180/pump              | PumpData          | Subscribe   |
| /data        | 869630050762180/data              | PumpData          | Subscribe   |
| /daq         | 869630050762180/daq               | DaqData           | Subscribe   |
| /ondemand    | 869630050762180/ondemand          | OnDemandCommand/Response | Subscribe/Publish |

## Publish Topics
- The app publishes to the same topics as it subscribes, depending on the operation:
  - Heartbeat, pump, data, daq: publish device data
  - ondemand: publish commands and responses

## Notes
- The IMEI/prefix can be changed in code for different devices.
- All topics are dynamically constructed as `<IMEI>/<suffix>`.
- The app subscribes to all above topics after (re)connection.
