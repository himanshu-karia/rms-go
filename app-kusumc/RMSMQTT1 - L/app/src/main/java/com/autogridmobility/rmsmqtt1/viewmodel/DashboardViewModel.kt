package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.os.IBinder
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.data.DaqData
import com.autogridmobility.rmsmqtt1.data.HeartbeatData
import com.autogridmobility.rmsmqtt1.data.OnDemandCommand
import com.autogridmobility.rmsmqtt1.data.OnDemandResponse
import com.autogridmobility.rmsmqtt1.data.PumpData
import com.autogridmobility.rmsmqtt1.service.MqttService
import com.autogridmobility.rmsmqtt1.transport.command.CommandRequest
import com.autogridmobility.rmsmqtt1.transport.command.FallbackCommandTransport
import com.autogridmobility.rmsmqtt1.utils.MobileSessionManager
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter
import kotlin.random.Random

class DashboardViewModel(application: Application) : AndroidViewModel(application) {
    private val commandTransport = FallbackCommandTransport()
    private val sessionManager = MobileSessionManager(application)
    private val defaultProjectId = "pm-kusum-solar-pump-msedcl"
    private val defaultCommandId = "pump_toggle"
    
    private var mqttService: MqttService? = null
    private var bound = false
    
    private val _heartbeatData = MutableStateFlow<HeartbeatData?>(null)
    val heartbeatData: StateFlow<HeartbeatData?> = _heartbeatData
    
    private val _pumpData = MutableStateFlow<PumpData?>(null)
    val pumpData: StateFlow<PumpData?> = _pumpData
    
    private val _daqData = MutableStateFlow<DaqData?>(null)
    val daqData: StateFlow<DaqData?> = _daqData
    
    private val _lastOnDemandResponse = MutableStateFlow<OnDemandResponse?>(null)
    val lastOnDemandResponse: StateFlow<OnDemandResponse?> = _lastOnDemandResponse
    
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(className: ComponentName, service: IBinder) {
            val binder = service as MqttService.MqttServiceBinder
            mqttService = binder.getService()
            bound = true
            
            // Observe data from service
            viewModelScope.launch {
                mqttService?.heartbeatData?.collect { data ->
                    _heartbeatData.value = data
                }
            }
            
            viewModelScope.launch {
                mqttService?.pumpData?.collect { data ->
                    _pumpData.value = data
                }
            }
            
            viewModelScope.launch {
                mqttService?.daqData?.collect { data ->
                    _daqData.value = data
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
    
    fun sendPumpOnCommand() {
        sendPumpCommand(turnOn = true)
    }
    
    fun sendPumpOffCommand() {
        sendPumpCommand(turnOn = false)
    }

    private fun sendPumpCommand(turnOn: Boolean) {
        val command = OnDemandCommand(
            msgid = Random.nextInt(10000, 99999).toString(),
            COTP = "12356",
            POTP = "58986",
            timestamp = LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss")),
            type = "ondemand",
            cmd = "write",
            DO1 = if (turnOn) 1 else 0
        )

        viewModelScope.launch {
            val bearerToken = sessionManager.getAccessToken()
            if (bearerToken.isBlank()) {
                publishViaLegacyMqtt(command, turnOn)
                return@launch
            }

            val deviceRef = pumpData.value?.IMEI
                ?: heartbeatData.value?.IMEI
                ?: daqData.value?.IMEI
                ?: "869630050762180"

            val sendResult = commandTransport.sendCommand(
                request = CommandRequest(
                    deviceId = deviceRef,
                    projectId = defaultProjectId,
                    commandId = defaultCommandId,
                    payload = mapOf("mode" to if (turnOn) "on" else "off")
                ),
                bearerToken = bearerToken
            )

            if (sendResult.isFailure) {
                publishViaLegacyMqtt(command, turnOn)
                return@launch
            }

            val latest = commandTransport.getLatestStatus(deviceRef, defaultProjectId, bearerToken).getOrNull()
            val statusText = latest?.status ?: sendResult.getOrNull()?.status ?: "submitted"
            _lastOnDemandResponse.value = OnDemandResponse(
                timestamp = command.timestamp,
                status = "Pump ${if (turnOn) "ON" else "OFF"} ($statusText)",
                DO1 = if (turnOn) 1 else 0,
                PRUNST1 = if (turnOn) "1" else "0"
            )
        }
    }

    private fun publishViaLegacyMqtt(command: OnDemandCommand, turnOn: Boolean) {
        mqttService?.publishOnDemandCommand(command)
        _lastOnDemandResponse.value = OnDemandResponse(
            timestamp = command.timestamp,
            status = "Pump ${if (turnOn) "ON" else "OFF"}",
            DO1 = if (turnOn) 1 else 0,
            PRUNST1 = if (turnOn) "1" else "0"
        )
    }
    
    override fun onCleared() {
        super.onCleared()
        if (bound) {
            getApplication<Application>().unbindService(connection)
            bound = false
        }
    }
}
