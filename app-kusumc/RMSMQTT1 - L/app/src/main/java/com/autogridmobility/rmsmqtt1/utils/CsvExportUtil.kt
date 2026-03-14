package com.autogridmobility.rmsmqtt1.utils

import android.content.Context
import android.content.Intent
import androidx.core.content.FileProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.io.FileWriter
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter

object CsvExportUtil {
    
    suspend fun exportDataToCsv(
        context: Context,
        heartbeatHistory: List<Pair<String, String>>,
        pumpHistory: List<Pair<String, String>>,
        daqHistory: List<Pair<String, String>>,
        onDemandHistory: List<Pair<String, String>>,
        startDate: LocalDateTime? = null,
        endDate: LocalDateTime? = null
    ): Result<List<File>> = withContext(Dispatchers.IO) {
        try {
            val exportDir = File(context.getExternalFilesDir(null), "csv_exports")
            if (!exportDir.exists()) {
                exportDir.mkdirs()
            }
            
            val timestamp = LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyyMMdd_HHmmss"))
            val files = mutableListOf<File>()
            
            // Filter data by date range if provided
            val filteredHeartbeat = filterByDateRange(heartbeatHistory, startDate, endDate)
            val filteredPump = filterByDateRange(pumpHistory, startDate, endDate)
            val filteredDaq = filterByDateRange(daqHistory, startDate, endDate)
            val filteredOnDemand = filterByDateRange(onDemandHistory, startDate, endDate)
            
            // Export Heartbeat data
            if (filteredHeartbeat.isNotEmpty()) {
                val heartbeatFile = File(exportDir, "heartbeat_data_$timestamp.csv")
                exportToCsvFile(heartbeatFile, "Heartbeat Data", filteredHeartbeat)
                files.add(heartbeatFile)
            }
            
            // Export Pump data
            if (filteredPump.isNotEmpty()) {
                val pumpFile = File(exportDir, "pump_data_$timestamp.csv")
                exportToCsvFile(pumpFile, "Pump Data", filteredPump)
                files.add(pumpFile)
            }
            
            // Export DAQ data
            if (filteredDaq.isNotEmpty()) {
                val daqFile = File(exportDir, "daq_data_$timestamp.csv")
                exportToCsvFile(daqFile, "DAQ Data", filteredDaq)
                files.add(daqFile)
            }
            
            // Export OnDemand data
            if (filteredOnDemand.isNotEmpty()) {
                val onDemandFile = File(exportDir, "ondemand_data_$timestamp.csv")
                exportToCsvFile(onDemandFile, "OnDemand Data", filteredOnDemand)
                files.add(onDemandFile)
            }
            
            Result.success(files)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }
    
    private fun filterByDateRange(
        history: List<Pair<String, String>>,
        startDate: LocalDateTime?,
        endDate: LocalDateTime?
    ): List<Pair<String, String>> {
        if (startDate == null && endDate == null) {
            return history
        }
        
        return history.filter { (timestampStr, _) ->
            try {
                val timestamp = LocalDateTime.parse(timestampStr, DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss"))
                val afterStart = startDate?.let { timestamp.isAfter(it) || timestamp.isEqual(it) } ?: true
                val beforeEnd = endDate?.let { timestamp.isBefore(it) || timestamp.isEqual(it) } ?: true
                afterStart && beforeEnd
            } catch (e: Exception) {
                true // Include data with invalid timestamps
            }
        }
    }
    
    private fun exportToCsvFile(
        file: File,
        dataType: String,
        data: List<Pair<String, String>>
    ) {
        FileWriter(file).use { writer ->
            // Write header
            writer.append("Timestamp,JSON_Data\n")
            
            // Write data
            data.forEach { (timestamp, jsonData) ->
                writer.append("\"$timestamp\",\"${jsonData.replace("\"", "\"\"")}\"\n")
            }
        }
    }
    
    fun shareExportedFiles(context: Context, files: List<File>) {
        if (files.isEmpty()) return
        
        val uris = files.map { file ->
            FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                file
            )
        }
        
        val intent = Intent().apply {
            if (uris.size == 1) {
                action = Intent.ACTION_SEND
                putExtra(Intent.EXTRA_STREAM, uris.first())
            } else {
                action = Intent.ACTION_SEND_MULTIPLE
                putParcelableArrayListExtra(Intent.EXTRA_STREAM, ArrayList(uris))
            }
            type = "text/csv"
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        
        context.startActivity(Intent.createChooser(intent, "Share CSV Files"))
    }
}
