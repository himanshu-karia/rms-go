package com.autogridmobility.rmsmqtt1.data

import kotlinx.serialization.Serializable

@Serializable
data class HeartbeatData(
    val VD: String,
    val TIMESTAMP: String,
    val DATE: String,
    val IMEI: String,
    val RTCDATE: String,
    val RTCTIME: String,
    val LAT: String,
    val LONG: String,
    val RSSI: String,
    val STINTERVAL: String,
    val POTP: String,
    val COTP: String,
    val GSM: String,
    val SIM: String,
    val NET: String,
    val GPRS: String,
    val SD: String,
    val ONLINE: String,
    val GPS: String,
    val GPSLOC: String,
    val RF: String,
    val TEMP: String,
    val SIMSLOT: String,
    val SIMCHNGCNT: String,
    val FLASH: String,
    val BATTST: String,
    val VBATT: Double,
    val PST: Int
)
