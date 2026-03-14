package com.autogridmobility.rmsmqtt1.service

import android.util.Log
import com.autogridmobility.rmsmqtt1.utils.DemoDataGenerator
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class DataSimulationService {
    
    companion object {
        private const val TAG = "DataSimulationService"
        private const val DEFAULT_PACKETS_COUNT = 10
    }
    
    private val json = Json { ignoreUnknownKeys = true }
    private var simulationJob: Job? = null
    private val serviceScope = CoroutineScope(Dispatchers.IO)
    
    private val _isSimulating = MutableStateFlow(false)
    val isSimulating: StateFlow<Boolean> = _isSimulating
    
    private val _currentInterval = MutableStateFlow(5)
    val currentInterval: StateFlow<Int> = _currentInterval
    
    private val _packetsPublished = MutableStateFlow(0)
    val packetsPublished: StateFlow<Int> = _packetsPublished
    
    // Reference to MQTT service for publishing
    private var mqttService: MqttService? = null
    private var connectionStatusJob: Job? = null

    fun setMqttService(service: MqttService) {
        this.mqttService = service
        // Listen to connection status changes
        connectionStatusJob?.cancel()
        connectionStatusJob = serviceScope.launch {
            service.connectionStatus.collect { status ->
                if (status == "Connected") {
                    if (_isSimulating.value && simulationJob == null) {
                        // If simulation was running but stopped due to disconnect, restart
                        startSimulation(_currentInterval.value)
                    }
                } else {
                    if (_isSimulating.value) {
                        // Stop simulation if connection lost
                        stopSimulation()
                    }
                }
            }
        }
    }
    
    fun startSimulation(intervalSeconds: Int) {
        if (_isSimulating.value) {
            Log.w(TAG, "Simulation already running")
            return
        }

        _currentInterval.value = intervalSeconds
        _isSimulating.value = true
        _packetsPublished.value = 0

        Log.d(TAG, "Starting data simulation with ${intervalSeconds}s interval")

        simulationJob = serviceScope.launch {
            while (_isSimulating.value) {
                // Wait for MQTT connection before publishing
                while (mqttService?.isCurrentlyConnected() != true) {
                    Log.d(TAG, "Waiting for MQTT connection before publishing...")
                    delay(1000)
                    if (!_isSimulating.value) return@launch
                }
                try {
                    publishSimulationBatch()
                    delay(intervalSeconds * 1000L)
                } catch (e: Exception) {
                    Log.e(TAG, "Error during simulation", e)
                }
            }
        }
    }
    
    fun stopSimulation() {
        Log.d(TAG, "Stopping data simulation")
        _isSimulating.value = false
        simulationJob?.cancel()
        simulationJob = null
    }
    
    private suspend fun publishSimulationBatch() {
        val mqttService = this.mqttService
        if (mqttService == null) {
            Log.w(TAG, "MQTT service not available")
            return
        }
        
        Log.d(TAG, "Publishing simulation batch of ${DEFAULT_PACKETS_COUNT} packets per topic")
        
        try {
            // Publish 10 heartbeat packets
            repeat(DEFAULT_PACKETS_COUNT) { index ->
                val heartbeatData = DemoDataGenerator.generateHeartbeatData()
                val heartbeatJson = json.encodeToString(heartbeatData)
                publishToTopic(mqttService, "heartbeat", heartbeatJson)
                _packetsPublished.value += 1
                
                // Small delay between packets to avoid overwhelming
                delay(100)
            }
            
            // Publish 10 pump data packets
            repeat(DEFAULT_PACKETS_COUNT) { index ->
                val pumpData = DemoDataGenerator.generatePumpData()
                val pumpJson = json.encodeToString(pumpData)
                publishToTopic(mqttService, "data", pumpJson)
                _packetsPublished.value += 1
                
                delay(100)
            }
            
            // Publish 10 DAQ data packets
            repeat(DEFAULT_PACKETS_COUNT) { index ->
                val daqData = DemoDataGenerator.generateDaqData()
                val daqJson = json.encodeToString(daqData)
                publishToTopic(mqttService, "daq", daqJson)
                _packetsPublished.value += 1
                
                delay(100)
            }
            
            Log.d(TAG, "Simulation batch completed. Total packets: ${_packetsPublished.value}")
            
        } catch (e: Exception) {
            Log.e(TAG, "Error publishing simulation batch", e)
        }
    }
    
    private fun publishToTopic(mqttService: MqttService, topic: String, payload: String) {
        try {
            mqttService.publishMessage(topic, payload)
            Log.d(TAG, "Published to $topic: ${payload.take(100)}...")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to publish to $topic", e)
        }
    }
    
    fun resetPacketCount() {
        _packetsPublished.value = 0
    }
    
    // Alias methods for MqttService compatibility
    fun start() {
        startSimulation(5) // Default 5 second interval
    }
    
    fun start(intervalSeconds: Int) {
        startSimulation(intervalSeconds)
    }
    
    fun stop() {
        stopSimulation()
    }
    
    fun isRunning(): Boolean {
        return _isSimulating.value
    }
    
    fun getPacketsPublished(): Int {
        return _packetsPublished.value
    }
}
