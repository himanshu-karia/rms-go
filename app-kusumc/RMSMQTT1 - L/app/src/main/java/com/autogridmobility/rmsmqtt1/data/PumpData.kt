package com.autogridmobility.rmsmqtt1.data

import kotlinx.serialization.Serializable

@Serializable
data class PumpData(
    val VD: String,
    val TIMESTAMP: String,
    val DATE: String,
    val IMEI: String,
    val PDKWH1: String,
    val PTOTKWH1: String,
    val POPDWD1: String,
    val POPTOTWD1: String,
    val PDHR1: String,
    val PTOTHR1: String,
    val POPKW1: String,
    val MAXINDEX: String,
    val INDEX: String,
    val LOAD: String,
    val STINTERVAL: String,
    val POTP: String,
    val COTP: String,
    val PMAXFREQ1: String,
    val PFREQLSP1: String,
    val PFREQHSP1: String,
    val PCNTRMODE1: String,
    val PRUNST1: String,
    val POPFREQ1: String,
    val POPI1: String,
    val POPV1: Int,
    val PDC1V1: Int,
    val PDC1I1: String,
    val PDCVOC1: String,
    val POPFLW1: String
)
