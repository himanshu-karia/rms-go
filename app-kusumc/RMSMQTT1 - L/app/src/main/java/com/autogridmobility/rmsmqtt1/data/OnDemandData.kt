package com.autogridmobility.rmsmqtt1.data

import kotlinx.serialization.Serializable

@Serializable
data class OnDemandCommand(
    val msgid: String,
    val COTP: String,
    val POTP: String,
    val timestamp: String,
    val type: String,
    val cmd: String,
    val DO1: Int // 0 for OFF, 1 for ON
)

@Serializable
data class OnDemandResponse(
    val timestamp: String,
    val status: String,
    val DO1: Int,
    val PRUNST1: String
)
