# JSON Parameter Names, Descriptions, and Units

## HeartbeatData
| Key           | Description                        | Unit      |
|--------------|------------------------------------|-----------|
| VD           | Virtual Device Index/Group         | N/A       |
| TIMESTAMP    | RTC Timestamp                      | N/A       |
| DATE         | Local Storage Date                 | N/A       |
| IMEI         | IMEI                              | N/A       |
| RTCDATE      | RTC Date                           | N/A       |
| RTCTIME      | RTC Time                           | N/A       |
| LAT          | Latitude                           | Degrees   |
| LONG         | Longitude                          | Degrees   |
| RSSI         | Signal Strength (RSSI)             | N/A       |
| STINTERVAL   | Periodic Interval                  | Minutes   |
| POTP         | Previous One Time Password         | N/A       |
| COTP         | Current One Time Password          | N/A       |
| GSM          | GSM Connected                      | N/A       |
| SIM          | SIM Detected                       | N/A       |
| NET          | Device in Network                  | N/A       |
| GPRS         | GPRS Connected                     | N/A       |
| SD           | SD Card Detected                   | N/A       |
| ONLINE       | Device Online                      | N/A       |
| GPS          | GPS Module Status                  | N/A       |
| GPSLOC       | GPS Location Locked                | N/A       |
| RF           | RF Module Status                   | N/A       |
| TEMP         | Device Temperature                 | Celsius   |
| SIMSLOT      | SIM Slot                           | N/A       |
| SIMCHNGCNT   | SIM Change Count                   | N/A       |
| FLASH        | Device Flash Status                | N/A       |
| BATTST       | Battery Input Status               | N/A       |
| VBATT        | Battery Voltage                    | V         |
| PST          | Power Supply Status                | N/A       |

## PumpData
| Key           | Description                        | Unit      |
|--------------|------------------------------------|-----------|
| VD           | Virtual Device Index/Group         | N/A       |
| TIMESTAMP    | RTC Timestamp                      | N/A       |
| DATE         | Local Storage Date                 | N/A       |
| IMEI         | IMEI                              | N/A       |
| PDKWH1       | Today Generated Energy             | KWH       |
| PTOTKWH1     | Cumulative Generated Energy        | KWH       |
| POPDWD1      | Daily Water Discharge              | Litres    |
| POPTOTWD1    | Total Water Discharge              | Litres    |
| PDHR1        | Pump Day Run Hours                 | Hrs       |
| PTOTHR1      | Pump Cumulative Run Hours          | Hrs       |
| POPKW1       | Output Active Power                | KW        |
| MAXINDEX     | Max Local Storage Index            | N/A       |
| INDEX        | Local Storage Index                | N/A       |
| LOAD         | Local Storage Load Status          | N/A       |
| STINTERVAL   | Periodic Interval                  | Minutes   |
| POTP         | Previous One Time Password         | N/A       |
| COTP         | Current One Time Password          | N/A       |
| PMAXFREQ1    | Maximum Frequency                  | Hz        |
| PFREQLSP1    | Lower Limit Frequency              | Hz        |
| PFREQHSP1    | Upper Limit Frequency              | Hz        |
| PCNTRMODE1   | Control Mode Status                | N/A       |
| PRUNST1      | Run Status                         | N/A       |
| POPFREQ1     | Output Frequency                   | Hz        |
| POPI1        | Output Current                     | A         |
| POPV1        | Output Voltage                     | V         |
| PDC1V1       | DC Input Voltage                   | DC V      |
| PDC1I1       | DC Current                         | DC I      |
| PDCVOC1      | DC Open Circuit Voltage            | DC V      |
| POPFLW1      | Flow Speed                         | LPM       |

## DaqData
| Key           | Description                        | Unit      |
|--------------|------------------------------------|-----------|
| VD           | Virtual Device Index/Group         | N/A       |
| TIMESTAMP    | RTC Timestamp                      | N/A       |
| MAXINDEX     | Max Local Storage Index            | N/A       |
| INDEX        | Local Storage Index                | N/A       |
| LOAD         | Local Storage Load Status          | N/A       |
| STINTERVAL   | Periodic Interval                  | Minutes   |
| MSGID        | Message Transaction Id             | N/A       |
| DATE         | Local Storage Date                 | N/A       |
| IMEI         | IMEI                              | N/A       |
| POTP         | Previous One Time Password         | N/A       |
| COTP         | Current One Time Password          | N/A       |
| AI11         | Analog Input -1                    | N/A       |
| AI21         | Analog Input - 2                   | N/A       |
| AI31         | Analog Input - 3                   | N/A       |
| AI41         | Analog Input - 4                   | N/A       |
| DI11         | Digital Input - 1                  | N/A       |
| DI21         | Digital Input - 2                  | N/A       |
| DI31         | Digital Input - 3                  | N/A       |
| DI41         | Digital Input - 4                  | N/A       |
| DO11         | Digital Output - 1                 | N/A       |
| DO21         | Digital Output - 2                 | N/A       |
| DO31         | Digital Output - 3                 | N/A       |
| DO41         | Digital Output - 4                 | N/A       |

## OnDemandCommand
| Key        | Description                        | Unit      |
|------------|------------------------------------|-----------|
| msgid      | Message Transaction Id             | N/A       |
| COTP       | Current One Time Password          | N/A       |
| POTP       | Previous One Time Password         | N/A       |
| timestamp  | Timestamp                          | N/A       |
| type       | Message Type                       | N/A       |
| cmd        | Command Type                       | N/A       |
| DO1        | Digital Output 1 (Pump Control)    | N/A       |

## OnDemandResponse
| Key        | Description                        | Unit      |
|------------|------------------------------------|-----------|
| timestamp  | Command Timestamp                  | N/A       |
| status     | Command Status                     | N/A       |
| DO1        | Digital Output 1 (Pump Control)    | Status    |
| PRUNST1    | Pump Run Status (PRUNST1)          | Status    |
