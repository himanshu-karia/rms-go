package com.autogridmobility.rmsmqtt1.utils

import com.autogridmobility.rmsmqtt1.data.DaqData
import com.autogridmobility.rmsmqtt1.data.HeartbeatData
import com.autogridmobility.rmsmqtt1.data.PumpData
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter
import kotlin.random.Random

object DemoDataGenerator {
    
    fun generateHeartbeatData(): HeartbeatData {
        val now = LocalDateTime.now()
        return HeartbeatData(
            VD = "0",
            TIMESTAMP = now.format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss")),
            DATE = now.format(DateTimeFormatter.ofPattern("yyMMdd")),
            IMEI = "869630050762180",
            RTCDATE = now.format(DateTimeFormatter.ofPattern("yyMMdd")),
            RTCTIME = now.format(DateTimeFormatter.ofPattern("HHmmss")),
            LAT = (26.8 + Random.nextDouble(-0.01, 0.01)).toString(),
            LONG = (75.5 + Random.nextDouble(-0.01, 0.01)).toString(),
            RSSI = Random.nextInt(10, 30).toString(),
            STINTERVAL = "15",
            POTP = "0",
            COTP = "0",
            GSM = "1",
            SIM = "1",
            NET = "1",
            GPRS = "1",
            SD = "1",
            ONLINE = "1",
            GPS = "1",
            GPSLOC = "1",
            RF = "1",
            TEMP = Random.nextInt(25, 35).toString(),
            SIMSLOT = "1",
            SIMCHNGCNT = "0",
            FLASH = "1",
            BATTST = "1",
            VBATT = 3.2 + Random.nextDouble(-0.2, 0.3),
            PST = 1
        )
    }
    
    fun generatePumpData(): PumpData {
        val now = LocalDateTime.now()
        val isRunning = Random.nextBoolean()
        return PumpData(
            VD = "1",
            TIMESTAMP = now.format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss")),
            DATE = now.format(DateTimeFormatter.ofPattern("yyMMdd")),
            IMEI = "869630050762180",
            PDKWH1 = if (isRunning) String.format("%.2f", Random.nextDouble(0.1, 5.0)) else "0.00",
            PTOTKWH1 = String.format("%.2f", 450.0 + Random.nextDouble(0.0, 50.0)),
            POPDWD1 = if (isRunning) String.format("%.2f", Random.nextDouble(100.0, 1000.0)) else "0.00",
            POPTOTWD1 = String.format("%.2f", 875000.0 + Random.nextDouble(0.0, 1000.0)),
            PDHR1 = Random.nextInt(0, 8).toString(),
            PTOTHR1 = String.format("%.2f", 120.0 + Random.nextDouble(0.0, 10.0)),
            POPKW1 = if (isRunning) Random.nextInt(1, 5).toString() else "0",
            MAXINDEX = "0",
            INDEX = "0",
            LOAD = "0",
            STINTERVAL = "15",
            POTP = Random.nextInt(100000, 999999).toString(),
            COTP = Random.nextInt(100000, 999999).toString(),
            PMAXFREQ1 = "50",
            PFREQLSP1 = "45",
            PFREQHSP1 = "55",
            PCNTRMODE1 = "1",
            PRUNST1 = if (isRunning) "1" else "0",
            POPFREQ1 = if (isRunning) Random.nextInt(48, 52).toString() else "0",
            POPI1 = if (isRunning) String.format("%.1f", Random.nextDouble(5.0, 15.0)) else "0",
            POPV1 = if (isRunning) Random.nextInt(360, 380) else 0,
            PDC1V1 = if (isRunning) Random.nextInt(370, 390) else 0,
            PDC1I1 = if (isRunning) String.format("%.3f", Random.nextDouble(0.5, 2.0)) else "0.001",
            PDCVOC1 = if (isRunning) String.format("%.3f", Random.nextDouble(1.0, 3.0)) else "0.001",
            POPFLW1 = if (isRunning) Random.nextInt(50, 200).toString() else "0"
        )
    }
    
    fun generateDaqData(): DaqData {
        val now = LocalDateTime.now()
        return DaqData(
            VD = "12",
            TIMESTAMP = now.format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss")),
            MAXINDEX = Random.nextInt(90, 100).toString(),
            INDEX = Random.nextInt(1, 10).toString(),
            LOAD = "0",
            STINTERVAL = "2",
            MSGID = String.format("%03d", Random.nextInt(0, 999)),
            DATE = now.format(DateTimeFormatter.ofPattern("yyMM")),
            IMEI = "869630050762180",
            POTP = Random.nextInt(10000000, 99999999).toString(),
            COTP = Random.nextInt(10000000, 99999999).toString(),
            AI11 = Random.nextInt(0, 100).toString(),
            AI21 = Random.nextInt(0, 100).toString(),
            AI31 = String.format("%.2f", Random.nextDouble(0.0, 100.0)),
            AI41 = String.format("%.2f", Random.nextDouble(0.0, 100.0)),
            DI11 = Random.nextInt(0, 2).toString(),
            DI21 = Random.nextInt(0, 2).toString(),
            DI31 = Random.nextInt(0, 2).toString(),
            DI41 = Random.nextInt(0, 2).toString(),
            DO11 = Random.nextInt(0, 2).toString(),
            DO21 = Random.nextInt(0, 2).toString(),
            DO31 = Random.nextInt(0, 2).toString(),
            DO41 = Random.nextInt(0, 2).toString()
        )
    }
}
