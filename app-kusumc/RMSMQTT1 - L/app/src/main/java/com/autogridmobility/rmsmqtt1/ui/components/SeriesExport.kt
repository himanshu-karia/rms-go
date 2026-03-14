package com.autogridmobility.rmsmqtt1.ui.components

import android.util.Log
import com.autogridmobility.rmsmqtt1.viewmodel.DataPoint
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/** Simple CSV export helper (currently logs CSV). Replace with file provider / share intent as needed. */
fun exportSeriesCsv(label: String, points: List<DataPoint>) {
    if (points.isEmpty()) {
        Log.i("SeriesExport", "No points to export for $label")
        return
    }
    val formatter = DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss").withZone(ZoneId.systemDefault())
    val sb = StringBuilder("timestamp,value\n")
    points.forEach { p ->
        sb.append(formatter.format(Instant.ofEpochMilli(p.t))).append(',').append(p.v).append('\n')
    }
    Log.i("SeriesExport", "CSV Export ($label):\n$sb")
}
