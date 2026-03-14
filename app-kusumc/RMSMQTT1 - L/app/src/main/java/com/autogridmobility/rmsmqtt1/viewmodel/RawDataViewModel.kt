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

class RawDataViewModel(application: Application) : AndroidViewModel(application) {
    
    private var mqttService: MqttService? = null
    private var bound = false
    
    private val _heartbeatHistory = MutableStateFlow<List<Pair<String, String>>>(emptyList())
    val heartbeatHistory: StateFlow<List<Pair<String, String>>> = _heartbeatHistory
    
    private val _pumpHistory = MutableStateFlow<List<Pair<String, String>>>(emptyList())
    val pumpHistory: StateFlow<List<Pair<String, String>>> = _pumpHistory
    
    private val _daqHistory = MutableStateFlow<List<Pair<String, String>>>(emptyList())
    val daqHistory: StateFlow<List<Pair<String, String>>> = _daqHistory
    
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(className: ComponentName, service: IBinder) {
            val binder = service as MqttService.MqttServiceBinder
            mqttService = binder.getService()
            bound = true
            
            // Observe raw data history from service
            viewModelScope.launch {
                mqttService?.heartbeatHistory?.collect { history ->
                    _heartbeatHistory.value = history
                }
            }
            
            viewModelScope.launch {
                mqttService?.pumpHistory?.collect { history ->
                    _pumpHistory.value = history
                }
            }
            
            viewModelScope.launch {
                mqttService?.daqHistory?.collect { history ->
                    _daqHistory.value = history
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
    
    override fun onCleared() {
        super.onCleared()
        if (bound) {
            getApplication<Application>().unbindService(connection)
            bound = false
        }
    }
}
