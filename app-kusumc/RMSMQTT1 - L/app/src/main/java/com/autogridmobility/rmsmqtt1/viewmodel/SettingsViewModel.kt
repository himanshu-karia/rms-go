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
import com.autogridmobility.rmsmqtt1.service.MqttConnectionState
import com.autogridmobility.rmsmqtt1.service.ButtonConfig
import com.autogridmobility.rmsmqtt1.utils.MqttPreferencesManager
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.delay

class SettingsViewModel(application: Application) : AndroidViewModel(application) {
    
    private val preferencesManager = MqttPreferencesManager(application)
    
    fun forceResetMqtt() {
        mqttService?.forceResetMqtt()
    }
    
    private var mqttService: MqttService? = null
    private var bound = false
    private var hasAttemptedAutoConnect = false
    
    // Settings state - initialized with saved preferences
    private val _url = MutableStateFlow(preferencesManager.getBrokerUrl())
    val url: StateFlow<String> = _url
    
    private val _port = MutableStateFlow(preferencesManager.getBrokerPort().toString())
    val port: StateFlow<String> = _port
    
    private val _username = MutableStateFlow(preferencesManager.getUsername())
    val username: StateFlow<String> = _username
    
    private val _password = MutableStateFlow(preferencesManager.getPassword())
    val password: StateFlow<String> = _password
    
    private val _clientId = MutableStateFlow(preferencesManager.getClientId())
    val clientId: StateFlow<String> = _clientId
    
    private val _topicPrefix = MutableStateFlow(preferencesManager.getTopicPrefix())
    val topicPrefix: StateFlow<String> = _topicPrefix
    
    // Field editability state
    private val _fieldsEditable = MutableStateFlow(true)
    val fieldsEditable: StateFlow<Boolean> = _fieldsEditable
    
    // Validation state
    private val _urlError = MutableStateFlow<String?>(null)
    val urlError: StateFlow<String?> = _urlError
    
    private val _portError = MutableStateFlow<String?>(null)
    val portError: StateFlow<String?> = _portError
    
    private val _clientIdError = MutableStateFlow<String?>(null)
    val clientIdError: StateFlow<String?> = _clientIdError
    
    private val _topicPrefixError = MutableStateFlow<String?>(null)
    val topicPrefixError: StateFlow<String?> = _topicPrefixError
    
    // Legacy connection status (for backward compatibility)
    private val _connectionStatus = MutableStateFlow("Inactive")
    val connectionStatus: StateFlow<String> = _connectionStatus
    
    private val _errorMessage = MutableStateFlow<String?>(null)
    val errorMessage: StateFlow<String?> = _errorMessage
    // New state-based properties
    private val _currentState = MutableStateFlow(MqttConnectionState.DISCONNECTED)
    val currentState: StateFlow<MqttConnectionState> = _currentState
    
    private val _buttonConfig = MutableStateFlow(ButtonConfig("Connect", true))
    val buttonConfig: StateFlow<ButtonConfig> = _buttonConfig
    
    private val _isConnecting = MutableStateFlow(false)
    val isConnecting: StateFlow<Boolean> = _isConnecting
    
    // Data simulation state
    private val _sendingInterval = MutableStateFlow("5")
    val sendingInterval: StateFlow<String> = _sendingInterval
    
    private val _isSimulating = MutableStateFlow(false)
    val isSimulating: StateFlow<Boolean> = _isSimulating
    
    private val _packetsPublished = MutableStateFlow(0)
    val packetsPublished: StateFlow<Int> = _packetsPublished
    
    // Topic prefix (IMEI) - removed as it's now in the main settings above
    
    // Subscribed topics list - now dynamically generated based on topic prefix
    private val _subscribedTopics = MutableStateFlow<List<String>>(emptyList())
    val subscribedTopics: StateFlow<List<String>> = _subscribedTopics
    
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(className: ComponentName, service: IBinder) {
            val binder = service as MqttService.MqttServiceBinder
            mqttService = binder.getService()
            bound = true
            
            // Observe legacy connection status (for backward compatibility)
            viewModelScope.launch {
                mqttService?.connectionStatus?.collect { status ->
                    _connectionStatus.value = status
                }
            }
            
            // Observe state manager state
            viewModelScope.launch {
                mqttService?.getStateManager()?.currentState?.collect { state ->
                    _currentState.value = state
                    _isConnecting.value = state == MqttConnectionState.CONNECTING || 
                                         state == MqttConnectionState.DISCONNECTING
                    updateFieldEditability(state)
                }
            }
            
            // Update button configuration based on state
            viewModelScope.launch {
                mqttService?.getStateManager()?.currentState?.collect { state ->
                    val config = mqttService?.getStateManager()?.getButtonConfig() 
                                ?: ButtonConfig("Connect", true)
                    _buttonConfig.value = config
                }
            }
            
            // Observe simulation status
            viewModelScope.launch {
                mqttService?.dataSimulationService?.isSimulating?.collect { simulating ->
                    _isSimulating.value = simulating
                }
            }
            
            // Observe packets published
            viewModelScope.launch {
                mqttService?.dataSimulationService?.packetsPublished?.collect { packets ->
                    _packetsPublished.value = packets
                }
            }
            // Observe error messages from state manager
            viewModelScope.launch {
                mqttService?.getStateManager()?.errorMessage?.collect { msg ->
                    _errorMessage.value = msg
                }
            }
            
            // Start periodic status refresh to catch disconnections
            startPeriodicStatusRefresh()

            // Auto-connect once on bind if settings are valid and currently disconnected
            maybeAutoConnect()
        }
        
        override fun onServiceDisconnected(arg0: ComponentName) {
            bound = false
            mqttService = null
        }
    }
    
    init {
        // Initialize subscribed topics list with saved topic prefix
        updateSubscribedTopicsList()
        
        // Validate initial values
        validateUrl(_url.value)
        validatePort(_port.value)
        validateClientId(_clientId.value)
        validateTopicPrefix(_topicPrefix.value)
        
        // Bind to MQTT service
        val context = getApplication<Application>()
        val intent = Intent(context, MqttService::class.java)
        context.bindService(intent, connection, Context.BIND_AUTO_CREATE)
    }
    
    fun updateUrl(newUrl: String) {
        _url.value = newUrl
        preferencesManager.saveBrokerUrl(newUrl)
        validateUrl(newUrl)
        updateSubscribedTopicsList()
    }
    
    fun updatePort(newPort: String) {
        _port.value = newPort
        val portInt = newPort.toIntOrNull()
        if (portInt != null && preferencesManager.isValidPort(newPort)) {
            preferencesManager.saveBrokerPort(portInt)
        }
        validatePort(newPort)
    }

    fun clearErrorMessage() {
        _errorMessage.value = null
    }
    
    fun updateUsername(newUsername: String) {
        _username.value = newUsername
        preferencesManager.saveUsername(newUsername)
    }
    
    fun updatePassword(newPassword: String) {
        _password.value = newPassword
        preferencesManager.savePassword(newPassword)
    }
    
    fun updateClientId(newClientId: String) {
        _clientId.value = newClientId
        preferencesManager.saveClientId(newClientId)
        validateClientId(newClientId)
    }
    
    fun updateTopicPrefix(newPrefix: String) {
        _topicPrefix.value = newPrefix
        preferencesManager.saveTopicPrefix(newPrefix)
        validateTopicPrefix(newPrefix)
        updateSubscribedTopicsList()
    }
    
    fun generateNewClientId() {
        val newClientId = preferencesManager.generateNewClientId()
        _clientId.value = newClientId
        validateClientId(newClientId)
    }
    
    // Validation methods
    private fun validateUrl(url: String) {
        _urlError.value = if (preferencesManager.isValidUrl(url)) {
            null
        } else {
            "URL cannot be empty"
        }
    }
    
    private fun validatePort(port: String) {
        _portError.value = if (preferencesManager.isValidPort(port)) {
            null
        } else {
            "Port must be between 1 and 65535"
        }
    }
    
    private fun validateClientId(clientId: String) {
        _clientIdError.value = if (preferencesManager.isValidClientId(clientId)) {
            null
        } else {
            "Client ID cannot be empty"
        }
    }
    
    private fun validateTopicPrefix(prefix: String) {
        _topicPrefixError.value = if (preferencesManager.isValidTopicPrefix(prefix)) {
            null
        } else {
            "Topic Prefix cannot be empty"
        }
    }
    
    // Update field editability based on connection state
    private fun updateFieldEditability(state: MqttConnectionState) {
        _fieldsEditable.value = when (state) {
            MqttConnectionState.DISCONNECTED,
            MqttConnectionState.ERROR,
            MqttConnectionState.NETWORK_LOST -> true
            else -> false
        }
    }
    
    fun updateSendingInterval(interval: String) {
        // Only allow numbers
        if (interval.isEmpty() || interval.all { it.isDigit() }) {
            _sendingInterval.value = interval
        }
    }
    
    private fun updateSubscribedTopicsList() {
        val prefix = _topicPrefix.value
        _subscribedTopics.value = listOf(
            "$prefix/heartbeat",
            "$prefix/data", 
            "$prefix/daq",
            "$prefix/ondemand"
        )
    }
    
    // Validate all fields before connecting
    private fun validateAllFields(): Boolean {
        validateUrl(_url.value)
        validatePort(_port.value)
        validateClientId(_clientId.value)
        validateTopicPrefix(_topicPrefix.value)
        
        return _urlError.value == null && 
               _portError.value == null && 
               _clientIdError.value == null && 
               _topicPrefixError.value == null
    }
    
    fun startDataSimulation() {
        val interval = _sendingInterval.value.toIntOrNull() ?: 5
        mqttService?.startDataSimulation(interval)
        mqttService?.resetSimulationPacketCount()
    }
    
    fun stopDataSimulation() {
        mqttService?.stopDataSimulation()
    }
    
    fun startSimulation() {
        startDataSimulation()
        _isSimulating.value = true
    }
    
    fun stopSimulation() {
        stopDataSimulation()
        _isSimulating.value = false
    }
    
    fun connect() {
        // Validate all fields before connecting
        if (!validateAllFields()) {
            _errorMessage.value = "Please fix validation errors before connecting"
            return
        }
        
        val portInt = _port.value.toIntOrNull() ?: 1883
        mqttService?.connect(
            url = _url.value,
            port = portInt,
            username = _username.value.takeIf { it.isNotBlank() },
            password = _password.value.takeIf { it.isNotBlank() },
            clientId = _clientId.value
        )
        
        // Update the service with new topic prefix
        mqttService?.updateTopicPrefix(_topicPrefix.value)
    }
    
    fun disconnect() {
        mqttService?.disconnect()
    }
    
    fun clearAllData() {
        mqttService?.clearAllHistory()
    }
    
    fun reconnect() {
        mqttService?.reconnect()
    }
    
    // Force refresh connection status using real-time check
    fun refreshConnectionStatus() {
        viewModelScope.launch {
            val isActuallyConnected = mqttService?.isCurrentlyConnected() ?: false
            _connectionStatus.value = if (isActuallyConnected) "Active" else "Inactive"
        }
    }
    
    // Get real-time connection status for button states
    fun isRealTimeConnected(): Boolean {
        return mqttService?.isCurrentlyConnected() ?: false
    }
    
    private fun startPeriodicStatusRefresh() {
        viewModelScope.launch {
            while (true) {
                delay(3000) // Check every 3 seconds
                refreshConnectionStatus()
            }
        }
    }

    private fun maybeAutoConnect() {
        if (hasAttemptedAutoConnect) return
        if (!validateAllFields()) return

        val state = _currentState.value
        if (state != MqttConnectionState.DISCONNECTED && state != MqttConnectionState.ERROR && state != MqttConnectionState.NETWORK_LOST) {
            return
        }

        hasAttemptedAutoConnect = true
        connect()
    }
    
    override fun onCleared() {
        super.onCleared()
        if (bound) {
            getApplication<Application>().unbindService(connection)
            bound = false
        }
    }
}
