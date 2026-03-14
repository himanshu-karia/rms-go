package com.autogridmobility.rmsmqtt1.data

data class ParameterInfo(
    val name: String,
    val unit: String
)

object ParameterMappings {
    
    val heartbeatMapping = mapOf(
        "VD" to ParameterInfo("Virtual Device Index/Group", "N/A"),
        "TIMESTAMP" to ParameterInfo("RTC Timestamp", "N/A"),
        "DATE" to ParameterInfo("Local Storage Date", "N/A"),
        "IMEI" to ParameterInfo("IMEI", "N/A"),
        "RTCDATE" to ParameterInfo("RTC Date", "N/A"),
        "RTCTIME" to ParameterInfo("RTC Time", "N/A"),
        "LAT" to ParameterInfo("Latitude", "Degrees"),
        "LONG" to ParameterInfo("Longitude", "Degrees"),
        "RSSI" to ParameterInfo("Signal Strength (RSSI)", "N/A"),
        "STINTERVAL" to ParameterInfo("Periodic Interval", "Minutes"),
        "POTP" to ParameterInfo("Previous One Time Password", "N/A"),
        "COTP" to ParameterInfo("Current One Time Password", "N/A"),
        "GSM" to ParameterInfo("GSM Connected", "N/A"),
        "SIM" to ParameterInfo("SIM Detected", "N/A"),
        "NET" to ParameterInfo("Device in Network", "N/A"),
        "GPRS" to ParameterInfo("GPRS Connected", "N/A"),
        "SD" to ParameterInfo("SD Card Detected", "N/A"),
        "ONLINE" to ParameterInfo("Device Online", "N/A"),
        "GPS" to ParameterInfo("GPS Module Status", "N/A"),
        "GPSLOC" to ParameterInfo("GPS Location Locked", "N/A"),
        "RF" to ParameterInfo("RF Module Status", "N/A"),
        "TEMP" to ParameterInfo("Device Temperature", "Celsius"),
        "SIMSLOT" to ParameterInfo("SIM Slot", "N/A"),
        "SIMCHNGCNT" to ParameterInfo("SIM Change Count", "N/A"),
        "FLASH" to ParameterInfo("Device Flash Status", "N/A"),
        "BATTST" to ParameterInfo("Battery Input Status", "N/A"),
        "VBATT" to ParameterInfo("Battery Voltage", "V"),
        "PST" to ParameterInfo("Power Supply Status", "N/A")
    )
    
    val pumpDataMapping = mapOf(
        "VD" to ParameterInfo("Virtual Device Index/Group", "N/A"),
        "TIMESTAMP" to ParameterInfo("RTC Timestamp", "N/A"),
        "DATE" to ParameterInfo("Local Storage Date", "N/A"),
        "IMEI" to ParameterInfo("IMEI", "N/A"),
        "PDKWH1" to ParameterInfo("Today Generated Energy", "KWH"),
        "PTOTKWH1" to ParameterInfo("Cumulative Generated Energy", "KWH"),
        "POPDWD1" to ParameterInfo("Daily Water Discharge", "Litres"),
        "POPTOTWD1" to ParameterInfo("Total Water Discharge", "Litres"),
        "PDHR1" to ParameterInfo("Pump Day Run Hours", "Hrs"),
        "PTOTHR1" to ParameterInfo("Pump Cumulative Run Hours", "Hrs"),
        "POPKW1" to ParameterInfo("Output Active Power", "KW"),
        "MAXINDEX" to ParameterInfo("Max Local Storage Index", "N/A"),
        "INDEX" to ParameterInfo("Local Storage Index", "N/A"),
        "LOAD" to ParameterInfo("Local Storage Load Status", "N/A"),
        "STINTERVAL" to ParameterInfo("Periodic Interval", "Minutes"),
        "POTP" to ParameterInfo("Previous One Time Password", "N/A"),
        "COTP" to ParameterInfo("Current One Time Password", "N/A"),
        "PMAXFREQ1" to ParameterInfo("Maximum Frequency", "Hz"),
        "PFREQLSP1" to ParameterInfo("Lower Limit Frequency", "Hz"),
        "PFREQHSP1" to ParameterInfo("Upper Limit Frequency", "Hz"),
        "PCNTRMODE1" to ParameterInfo("Control Mode Status", "N/A"),
        "PRUNST1" to ParameterInfo("Run Status", "N/A"),
        "POPFREQ1" to ParameterInfo("Output Frequency", "Hz"),
        "POPI1" to ParameterInfo("Output Current", "A"),
        "POPV1" to ParameterInfo("Output Voltage", "V"),
        "PDC1V1" to ParameterInfo("DC Input Voltage", "DC V"),
        "PDC1I1" to ParameterInfo("DC Current", "DC I"),
        "PDCVOC1" to ParameterInfo("DC Open Circuit Voltage", "DC V"),
        "POPFLW1" to ParameterInfo("Flow Speed", "LPM")
    )
    
    val daqDataMapping = mapOf(
        "VD" to ParameterInfo("Virtual Device Index/Group", "N/A"),
        "TIMESTAMP" to ParameterInfo("RTC Timestamp", "N/A"),
        "MAXINDEX" to ParameterInfo("Max Local Storage Index", "N/A"),
        "INDEX" to ParameterInfo("Local Storage Index", "N/A"),
        "LOAD" to ParameterInfo("Local Storage Load Status", "N/A"),
        "STINTERVAL" to ParameterInfo("Periodic Interval", "Minutes"),
        "MSGID" to ParameterInfo("Message Transaction Id", "N/A"),
        "DATE" to ParameterInfo("Local Storage Date", "N/A"),
        "IMEI" to ParameterInfo("IMEI", "N/A"),
        "POTP" to ParameterInfo("Previous One Time Password", "N/A"),
        "COTP" to ParameterInfo("Current One Time Password", "N/A"),
        "AI11" to ParameterInfo("Analog Input -1", "N/A"),
        "AI21" to ParameterInfo("Analog Input - 2", "N/A"),
        "AI31" to ParameterInfo("Analog Input - 3", "N/A"),
        "AI41" to ParameterInfo("Analog Input - 4", "N/A"),
        "DI11" to ParameterInfo("Digital Input - 1", "N/A"),
        "DI21" to ParameterInfo("Digital Input - 2", "N/A"),
        "DI31" to ParameterInfo("Digital Input - 3", "N/A"),
        "DI41" to ParameterInfo("Digital Input - 4", "N/A"),
        "DO11" to ParameterInfo("Digital Output - 1", "N/A"),
        "DO21" to ParameterInfo("Digital Output - 2", "N/A"),
        "DO31" to ParameterInfo("Digital Output - 3", "N/A"),
        "DO41" to ParameterInfo("Digital Output - 4", "N/A")
    )
    
    val onDemandCommandMapping = mapOf(
        "msgid" to ParameterInfo("Message Transaction Id", "N/A"),
        "COTP" to ParameterInfo("Current One Time Password", "N/A"),
        "POTP" to ParameterInfo("Previous One Time Password", "N/A"),
        "timestamp" to ParameterInfo("Timestamp", "N/A"),
        "type" to ParameterInfo("Message Type", "N/A"),
        "cmd" to ParameterInfo("Command Type", "N/A"),
        "DO1" to ParameterInfo("Digital Output 1 (Pump Control)", "N/A")
    )
    
    val onDemandResponseMapping = mapOf(
        "timestamp" to ParameterInfo("Command Timestamp", "N/A"),
        "DO1" to ParameterInfo("Digital Output 1 (Pump Control)", "Status"),
        "PRUNST1" to ParameterInfo("Pump Run Status (PRUNST1)", "Status")
    )
}
