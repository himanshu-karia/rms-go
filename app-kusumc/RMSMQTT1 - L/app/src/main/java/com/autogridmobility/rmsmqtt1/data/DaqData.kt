package com.autogridmobility.rmsmqtt1.data

import kotlinx.serialization.Serializable

@Serializable
data class DaqData(
    val VD: String,
    val TIMESTAMP: String,
    val MAXINDEX: String,
    val INDEX: String,
    val LOAD: String,
    val STINTERVAL: String,
    val MSGID: String,
    val DATE: String,
    val IMEI: String,
    val POTP: String,
    val COTP: String,
    val AI11: String,
    val AI21: String,
    val AI31: String,
    val AI41: String,
    val DI11: String,
    val DI21: String,
    val DI31: String,
    val DI41: String,
    val DO11: String,
    val DO21: String,
    val DO31: String,
    val DO41: String
)
