package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.os.IBinder
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.data.HeartbeatData
import com.autogridmobility.rmsmqtt1.data.PumpData
import com.autogridmobility.rmsmqtt1.data.DaqData
import com.autogridmobility.rmsmqtt1.service.MqttService
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter
import kotlin.math.abs
import kotlinx.coroutines.flow.collect

/**
 * UX Dashboard ViewModel - Provides synchronized, real-time data for the UX Dashboard
 * with intelligent packet correlation and stale data detection
 */
class UxDashboardViewModel(application: Application) : AndroidViewModel(application) {
    
    companion object {
        private const val SYNC_TOLERANCE_SECONDS = 12L // ±12 seconds sync tolerance
        private const val STALE_DATA_THRESHOLD_SECONDS = 120L // 120 seconds before showing "No Data"
    }
    
    private val json = Json { ignoreUnknownKeys = true }
    private var mqttService: MqttService? = null
    private var bound = false
    
    // Synchronized dashboard data holder
    private val _dashboardData = MutableStateFlow(UxDashboardData())
    val dashboardData: StateFlow<UxDashboardData> = _dashboardData
    
    // Individual packet data with timestamps
    private val _latestHeartbeat = MutableStateFlow<TimestampedData<HeartbeatData>?>(null)
    private val _latestPumpData = MutableStateFlow<TimestampedData<PumpData>?>(null)
    private val _latestDaqData = MutableStateFlow<TimestampedData<DaqData>?>(null)
    
    // Time-series for pump frequency (used by time-series graph)
    private val _frequencySeries = MutableStateFlow<List<DataPoint>>(emptyList())
    val frequencySeries: StateFlow<List<DataPoint>> = _frequencySeries

    // Retention (minutes) for all time-series (configurable via Graphs screen)
    private val _retentionMinutes = MutableStateFlow(10) // default 10 minutes
    val retentionMinutes: StateFlow<Int> = _retentionMinutes
    fun setRetentionMinutes(mins: Long) {
        val clamped = mins.coerceIn(1, 120).toInt()
        _retentionMinutes.value = clamped
    }
    
    // Additional time-series buffers (time-based retention)
    private val _batterySeries = MutableStateFlow<List<DataPoint>>(emptyList())
    val batterySeries: StateFlow<List<DataPoint>> = _batterySeries

    private val _temperatureSeries = MutableStateFlow<List<DataPoint>>(emptyList())
    val temperatureSeries: StateFlow<List<DataPoint>> = _temperatureSeries

    private val _powerSeries = MutableStateFlow<List<DataPoint>>(emptyList())
    val powerSeries: StateFlow<List<DataPoint>> = _powerSeries

    private val _flowSeries = MutableStateFlow<List<DataPoint>>(emptyList())
    val flowSeries: StateFlow<List<DataPoint>> = _flowSeries

    private val _currentSeries = MutableStateFlow<List<DataPoint>>(emptyList())
    val currentSeries: StateFlow<List<DataPoint>> = _currentSeries

    private val _voltageSeries = MutableStateFlow<List<DataPoint>>(emptyList())
    val voltageSeries: StateFlow<List<DataPoint>> = _voltageSeries

    // DAQ analog inputs (AI11..AI41)
    private val _ai11Series = MutableStateFlow<List<DataPoint>>(emptyList())
    val ai11Series: StateFlow<List<DataPoint>> = _ai11Series
    private val _ai21Series = MutableStateFlow<List<DataPoint>>(emptyList())
    val ai21Series: StateFlow<List<DataPoint>> = _ai21Series
    private val _ai31Series = MutableStateFlow<List<DataPoint>>(emptyList())
    val ai31Series: StateFlow<List<DataPoint>> = _ai31Series
    private val _ai41Series = MutableStateFlow<List<DataPoint>>(emptyList())
    val ai41Series: StateFlow<List<DataPoint>> = _ai41Series
    
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(className: ComponentName, service: IBinder) {
            val binder = service as MqttService.MqttServiceBinder
            mqttService = binder.getService()
            bound = true
            
            // Subscribe to heartbeat data
            viewModelScope.launch {
                mqttService?.heartbeatHistory?.collect { history ->
                    if (history.isNotEmpty()) {
                        val latest = history.last()
                        try {
                            val heartbeatData = json.decodeFromString<HeartbeatData>(latest.second)
                            val timestamp = parseTimestamp(heartbeatData.TIMESTAMP)
                            _latestHeartbeat.value = TimestampedData(heartbeatData, timestamp)
                            synchronizeData()
                        } catch (e: Exception) {
                            // Handle parsing error
                        }
                    }
                }
            }
            
            // Subscribe to pump data
            viewModelScope.launch {
                mqttService?.pumpHistory?.collect { history ->
                    if (history.isNotEmpty()) {
                        val latest = history.last()
                        try {
                            val pumpData = json.decodeFromString<PumpData>(latest.second)
                            val timestamp = parseTimestamp(pumpData.TIMESTAMP)
                            _latestPumpData.value = TimestampedData(pumpData, timestamp)
                            synchronizeData()
                        } catch (e: Exception) {
                            // Handle parsing error
                        }
                    }
                }
            }

            // Build a time-series buffer for frequency values from pumpHistory
            viewModelScope.launch {
                mqttService?.pumpHistory?.collect { history ->
                    try {
            val retentionMillis = _retentionMinutes.value * 60_000
            val nowMs = System.currentTimeMillis()
            val points = history.mapNotNull { pair ->
                            val tsString = pair.first
                            val payload = pair.second
                            try {
                                val pd = json.decodeFromString<PumpData>(payload)
                                val freq = pd.POPFREQ1.toFloatOrNull()
                                if (freq == null) return@mapNotNull null
                                val t = try {
                                    val lt = parseTimestamp(tsString)
                                    lt.atZone(java.time.ZoneId.systemDefault()).toInstant().toEpochMilli()
                                } catch (_: Exception) {
                                    System.currentTimeMillis()
                                }
                if (nowMs - t > retentionMillis) return@mapNotNull null
                                DataPoint(t, freq)
                            } catch (_: Exception) {
                                null
                            }
                        }
            _frequencySeries.value = points
                    } catch (_: Exception) {
                        // ignore
                    }
                }
            }

            // Build time-series buffers for other pump metrics (power, flow, current, voltage)
            viewModelScope.launch {
                mqttService?.pumpHistory?.collect { history ->
                    try {
                        val retentionMillis = _retentionMinutes.value * 60_000
                        val nowMs = System.currentTimeMillis()
                        val powerPts = mutableListOf<DataPoint>()
                        val flowPts = mutableListOf<DataPoint>()
                        val currentPts = mutableListOf<DataPoint>()
                        val voltPts = mutableListOf<DataPoint>()
                        history.forEach { pair ->
                            val tsString = pair.first
                            val payload = pair.second
                            try {
                                val pd = json.decodeFromString<PumpData>(payload)
                                val t = try { parseTimestamp(tsString).atZone(java.time.ZoneId.systemDefault()).toInstant().toEpochMilli() } catch (_: Exception) { System.currentTimeMillis() }
                                if (nowMs - t <= retentionMillis) {
                                    pd.POPKW1.toFloatOrNull()?.let { powerPts.add(DataPoint(t, it)) }
                                    pd.POPFLW1.toFloatOrNull()?.let { flowPts.add(DataPoint(t, it)) }
                                    pd.POPI1.toFloatOrNull()?.let { currentPts.add(DataPoint(t, it)) }
                                    voltPts.add(DataPoint(t, pd.POPV1.toFloat()))
                                }
                            } catch (_: Exception) { }
                        }
                        _powerSeries.value = powerPts
                        _flowSeries.value = flowPts
                        _currentSeries.value = currentPts
                        _voltageSeries.value = voltPts
                    } catch (_: Exception) { }
                }
            }

            // Build battery and temperature series from heartbeatHistory
            viewModelScope.launch {
                mqttService?.heartbeatHistory?.collect { history ->
                    try {
                        val retentionMillis = _retentionMinutes.value * 60_000
                        val nowMs = System.currentTimeMillis()
                        val batteryPts = mutableListOf<DataPoint>()
                        val tempPts = mutableListOf<DataPoint>()
                        history.forEach { pair ->
                            val tsString = pair.first
                            val payload = pair.second
                            try {
                                val hd = json.decodeFromString<HeartbeatData>(payload)
                                val t = try { parseTimestamp(tsString).atZone(java.time.ZoneId.systemDefault()).toInstant().toEpochMilli() } catch (_: Exception) { System.currentTimeMillis() }
                                if (nowMs - t <= retentionMillis) {
                                    batteryPts.add(DataPoint(t, hd.VBATT.toFloat()))
                                    hd.TEMP.toFloatOrNull()?.let { tempPts.add(DataPoint(t, it)) }
                                }
                            } catch (_: Exception) { }
                        }
                        _batterySeries.value = batteryPts
                        _temperatureSeries.value = tempPts
                    } catch (_: Exception) { }
                }
            }

            // Build DAQ analog input series
            viewModelScope.launch {
                mqttService?.daqHistory?.collect { history ->
                    try {
                        val retentionMillis = _retentionMinutes.value * 60_000
                        val nowMs = System.currentTimeMillis()
                        val ai11 = mutableListOf<DataPoint>()
                        val ai21 = mutableListOf<DataPoint>()
                        val ai31 = mutableListOf<DataPoint>()
                        val ai41 = mutableListOf<DataPoint>()
                        history.forEach { pair ->
                            val tsString = pair.first
                            val payload = pair.second
                            try {
                                val dd = json.decodeFromString<DaqData>(payload)
                                val t = try { parseTimestamp(tsString).atZone(java.time.ZoneId.systemDefault()).toInstant().toEpochMilli() } catch (_: Exception) { System.currentTimeMillis() }
                                if (nowMs - t <= retentionMillis) {
                                    dd.AI11.toFloatOrNull()?.let { ai11.add(DataPoint(t, it)) }
                                    dd.AI21.toFloatOrNull()?.let { ai21.add(DataPoint(t, it)) }
                                    dd.AI31.toFloatOrNull()?.let { ai31.add(DataPoint(t, it)) }
                                    dd.AI41.toFloatOrNull()?.let { ai41.add(DataPoint(t, it)) }
                                }
                            } catch (_: Exception) { }
                        }
                        _ai11Series.value = ai11
                        _ai21Series.value = ai21
                        _ai31Series.value = ai31
                        _ai41Series.value = ai41
                    } catch (_: Exception) { }
                }
            }
            
            // Subscribe to DAQ data
            viewModelScope.launch {
                mqttService?.daqHistory?.collect { history ->
                    if (history.isNotEmpty()) {
                        val latest = history.last()
                        try {
                            val daqData = json.decodeFromString<DaqData>(latest.second)
                            val timestamp = parseTimestamp(daqData.TIMESTAMP)
                            _latestDaqData.value = TimestampedData(daqData, timestamp)
                            synchronizeData()
                        } catch (e: Exception) {
                            // Handle parsing error
                        }
                    }
                }
            }
        }
        
        override fun onServiceDisconnected(arg0: ComponentName) {
            bound = false
            mqttService = null
        }
    }
    
    init {
        // Bind to MQTT service
        val context = getApplication<Application>()
        val intent = Intent(context, MqttService::class.java)
        context.bindService(intent, connection, Context.BIND_AUTO_CREATE)
    }
    
    /**
     * Synchronize data from all three packet types with intelligent stale detection
     */
    private fun synchronizeData() {
        val now = LocalDateTime.now()
        val heartbeat = _latestHeartbeat.value
        val pumpData = _latestPumpData.value
        val daqData = _latestDaqData.value
        
        // Find the most recent timestamp among all packets
        val timestamps = listOfNotNull(
            heartbeat?.timestamp,
            pumpData?.timestamp,
            daqData?.timestamp
        )
        
        if (timestamps.isEmpty()) return
        
        val mostRecentTime = timestamps.maxOrNull() ?: return
        
        // Check data freshness and synchronization
        val syncedData = UxDashboardData(
            communicationStatus = calculateCommunicationStatus(heartbeat, mostRecentTime),
            rssi = getValueWithFreshness(heartbeat?.data?.RSSI, heartbeat?.timestamp, mostRecentTime, now),
            batteryVoltage = getValueWithFreshness(heartbeat?.data?.VBATT?.toString(), heartbeat?.timestamp, mostRecentTime, now),
            batteryStatus = getValueWithFreshness(heartbeat?.data?.BATTST, heartbeat?.timestamp, mostRecentTime, now),
            powerState = getValueWithFreshness(heartbeat?.data?.PST?.toString(), heartbeat?.timestamp, mostRecentTime, now),
            pumpRunning = getValueWithFreshness(pumpData?.data?.PRUNST1, pumpData?.timestamp, mostRecentTime, now),
            frequency = getValueWithFreshness(pumpData?.data?.POPFREQ1, pumpData?.timestamp, mostRecentTime, now),
            power = getValueWithFreshness(pumpData?.data?.POPKW1, pumpData?.timestamp, mostRecentTime, now),
            flowRate = getValueWithFreshness(pumpData?.data?.POPFLW1, pumpData?.timestamp, mostRecentTime, now),
            current = getValueWithFreshness(pumpData?.data?.POPI1, pumpData?.timestamp, mostRecentTime, now),
            voltage = getValueWithFreshness(pumpData?.data?.POPV1?.toString(), pumpData?.timestamp, mostRecentTime, now),
            dailyEnergy = getValueWithFreshness(pumpData?.data?.PDKWH1, pumpData?.timestamp, mostRecentTime, now),
            totalEnergy = getValueWithFreshness(pumpData?.data?.PTOTKWH1, pumpData?.timestamp, mostRecentTime, now),
            dailyWater = getValueWithFreshness(pumpData?.data?.POPDWD1, pumpData?.timestamp, mostRecentTime, now),
            totalWater = getValueWithFreshness(pumpData?.data?.POPTOTWD1, pumpData?.timestamp, mostRecentTime, now),
            dailyHours = getValueWithFreshness(pumpData?.data?.PDHR1, pumpData?.timestamp, mostRecentTime, now),
            totalHours = getValueWithFreshness(pumpData?.data?.PTOTHR1, pumpData?.timestamp, mostRecentTime, now),
            temperature = getValueWithFreshness(heartbeat?.data?.TEMP, heartbeat?.timestamp, mostRecentTime, now),
            gpsStatus = getValueWithFreshness(heartbeat?.data?.GPS, heartbeat?.timestamp, mostRecentTime, now),
            gpsLocation = getValueWithFreshness(heartbeat?.data?.GPSLOC, heartbeat?.timestamp, mostRecentTime, now),
            rfModule = getValueWithFreshness(heartbeat?.data?.RF, heartbeat?.timestamp, mostRecentTime, now),
            sdCard = getValueWithFreshness(heartbeat?.data?.SD, heartbeat?.timestamp, mostRecentTime, now),
            flashMemory = getValueWithFreshness(heartbeat?.data?.FLASH, heartbeat?.timestamp, mostRecentTime, now),
            analogInputs = listOf(
                getValueWithFreshness(daqData?.data?.AI11, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.AI21, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.AI31, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.AI41, daqData?.timestamp, mostRecentTime, now)
            ),
            digitalInputs = listOf(
                getValueWithFreshness(daqData?.data?.DI11, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.DI21, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.DI31, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.DI41, daqData?.timestamp, mostRecentTime, now)
            ),
            digitalOutputs = listOf(
                getValueWithFreshness(daqData?.data?.DO11, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.DO21, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.DO31, daqData?.timestamp, mostRecentTime, now),
                getValueWithFreshness(daqData?.data?.DO41, daqData?.timestamp, mostRecentTime, now)
            ),
            lastUpdateTime = mostRecentTime,
            deviceImei = heartbeat?.data?.IMEI ?: pumpData?.data?.IMEI ?: daqData?.data?.IMEI ?: "Unknown"
        )
        
        _dashboardData.value = syncedData
    }
    
    /**
     * Calculate smart communication status based on hierarchical dependencies
     */
    private fun calculateCommunicationStatus(
        heartbeat: TimestampedData<HeartbeatData>?,
        mostRecentTime: LocalDateTime
    ): UxValue<String> {
        if (heartbeat == null) return UxValue("No Data", false, true)
        
        val data = heartbeat.data
        val isFresh = isDataFresh(heartbeat.timestamp, LocalDateTime.now())
        val isStale = isDataStale(heartbeat.timestamp, LocalDateTime.now())
        
        if (isStale) return UxValue("No Data", false, true)
        
        val status = when {
            data.GSM != "1" -> "Offline"
            data.SIM != "1" -> "Offline"
            data.NET != "1" -> "Offline"
            data.GPRS != "1" -> "Connected"
            data.ONLINE != "1" -> "Connected"
            else -> "Online"
        }
        
        return UxValue(status, isFresh, false)
    }
    
    /**
     * Get value with freshness and stale detection
     */
    private fun getValueWithFreshness(
        value: String?,
        dataTime: LocalDateTime?,
        mostRecentTime: LocalDateTime,
        currentTime: LocalDateTime
    ): UxValue<String> {
        if (value == null || dataTime == null) {
            return UxValue("No Data", false, true)
        }
        
        val isFresh = isSynchronized(dataTime, mostRecentTime)
        val isStale = isDataStale(dataTime, currentTime)
        
        return when {
            isStale -> UxValue("No Data", false, true)
            else -> UxValue(value, isFresh, false)
        }
    }
    
    /**
     * Check if two timestamps are synchronized within tolerance
     */
    private fun isSynchronized(timestamp1: LocalDateTime, timestamp2: LocalDateTime): Boolean {
        val diff = abs(java.time.Duration.between(timestamp1, timestamp2).seconds)
        return diff <= SYNC_TOLERANCE_SECONDS
    }
    
    /**
     * Check if data is fresh (within sync tolerance of most recent)
     */
    private fun isDataFresh(dataTime: LocalDateTime, currentTime: LocalDateTime): Boolean {
        val age = java.time.Duration.between(dataTime, currentTime).seconds
        return age <= SYNC_TOLERANCE_SECONDS
    }
    
    /**
     * Check if data is stale (older than threshold)
     */
    private fun isDataStale(dataTime: LocalDateTime, currentTime: LocalDateTime): Boolean {
        val age = java.time.Duration.between(dataTime, currentTime).seconds
        return age > STALE_DATA_THRESHOLD_SECONDS
    }
    
    /**
     * Parse timestamp from string to LocalDateTime
     */
    private fun parseTimestamp(timestampString: String): LocalDateTime {
        return try {
            LocalDateTime.parse(timestampString, DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss"))
        } catch (e: Exception) {
            LocalDateTime.now() // Fallback to current time
        }
    }
    
    override fun onCleared() {
        super.onCleared()
        if (bound) {
            getApplication<Application>().unbindService(connection)
            bound = false
        }
    }
}

/**
 * Data class for timestamped packet data
 */
data class TimestampedData<T>(
    val data: T,
    val timestamp: LocalDateTime
)

/**
 * UX Dashboard value with freshness indicators
 */
data class UxValue<T>(
    val value: T,
    val isFresh: Boolean,       // Within sync tolerance of most recent packet
    val isStale: Boolean        // Older than stale threshold (show "No Data")
)

/**
 * Synchronized dashboard data structure
 */
data class UxDashboardData(
    // Communication & Connectivity Hub
    val communicationStatus: UxValue<String> = UxValue("No Data", false, true),
    val rssi: UxValue<String> = UxValue("No Data", false, true),
    
    // Power & Battery System
    val batteryVoltage: UxValue<String> = UxValue("No Data", false, true),
    val batteryStatus: UxValue<String> = UxValue("No Data", false, true),
    val powerState: UxValue<String> = UxValue("No Data", false, true),
    
    // Pump Operations Hub
    val pumpRunning: UxValue<String> = UxValue("No Data", false, true),
    val frequency: UxValue<String> = UxValue("No Data", false, true),
    val power: UxValue<String> = UxValue("No Data", false, true),
    val flowRate: UxValue<String> = UxValue("No Data", false, true),
    val current: UxValue<String> = UxValue("No Data", false, true),
    val voltage: UxValue<String> = UxValue("No Data", false, true),
    
    // Energy Monitoring
    val dailyEnergy: UxValue<String> = UxValue("No Data", false, true),
    val totalEnergy: UxValue<String> = UxValue("No Data", false, true),
    val dailyWater: UxValue<String> = UxValue("No Data", false, true),
    val totalWater: UxValue<String> = UxValue("No Data", false, true),
    val dailyHours: UxValue<String> = UxValue("No Data", false, true),
    val totalHours: UxValue<String> = UxValue("No Data", false, true),
    
    // System Health
    val temperature: UxValue<String> = UxValue("No Data", false, true),
    val gpsStatus: UxValue<String> = UxValue("No Data", false, true),
    val gpsLocation: UxValue<String> = UxValue("No Data", false, true),
    val rfModule: UxValue<String> = UxValue("No Data", false, true),
    val sdCard: UxValue<String> = UxValue("No Data", false, true),
    val flashMemory: UxValue<String> = UxValue("No Data", false, true),
    
    // Digital I/O Matrix
    val analogInputs: List<UxValue<String>> = List(4) { UxValue("No Data", false, true) },
    val digitalInputs: List<UxValue<String>> = List(4) { UxValue("No Data", false, true) },
    val digitalOutputs: List<UxValue<String>> = List(4) { UxValue("No Data", false, true) },
    
    // Metadata
    val lastUpdateTime: LocalDateTime? = null,
    val deviceImei: String = "Unknown"
)

/**
 * Simple time-series data point used by graphs
 */
data class DataPoint(
    val t: Long, // epoch millis
    val v: Float
)
