package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.os.IBinder
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.service.MqttService
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.delay

class HomeViewModel(application: Application) : AndroidViewModel(application) {
    
    private var mqttService: MqttService? = null
    private var bound = false
    
    private val _connectionStatus = MutableStateFlow("Inactive")
    val connectionStatus: StateFlow<String> = _connectionStatus
    
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(className: ComponentName, service: IBinder) {
            val binder = service as MqttService.MqttServiceBinder
            mqttService = binder.getService()
            bound = true
            
            // Observe connection status
            viewModelScope.launch {
                mqttService?.connectionStatus?.collect { status ->
                    _connectionStatus.value = status
                }
            }
            
            // Start periodic status refresh
            startPeriodicStatusRefresh()
        }
        
        override fun onServiceDisconnected(arg0: ComponentName) {
            bound = false
            mqttService = null
        }
    }
    
    init {
        // Start and bind to MQTT service
        val context = getApplication<Application>()
        val intent = Intent(context, MqttService::class.java)
        context.startForegroundService(intent)
        context.bindService(intent, connection, Context.BIND_AUTO_CREATE)
    }
    
    // Force refresh connection status using real-time check
    fun refreshConnectionStatus() {
        viewModelScope.launch {
            val isActuallyConnected = mqttService?.isCurrentlyConnected() ?: false
            _connectionStatus.value = if (isActuallyConnected) "Active" else "Inactive"
        }
    }
    
    private fun startPeriodicStatusRefresh() {
        viewModelScope.launch {
            while (true) {
                delay(3000) // Check every 3 seconds
                refreshConnectionStatus()
            }
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
