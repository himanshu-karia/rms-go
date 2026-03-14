package com.autogridmobility.rmsmqtt1.service

import android.util.Log
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/**
 * MQTT Connection States
 */
enum class MqttConnectionState {
    DISCONNECTED,    // No connection, network may/may not be available
    CONNECTING,      // Connection attempt in progress
    CONNECTED,       // Successfully connected
    DISCONNECTING,   // Intentional disconnection in progress
    ERROR,           // Connection failed or unexpected disconnection
    NETWORK_LOST,    // Network unavailable while connected
    FORCE_RESETTING  // Brute-force cleanup in progress
}

/**
 * State Transition Triggers
 */
enum class StateTrigger {
    USER_CONNECT,
    USER_DISCONNECT,
    CONNECTION_SUCCESS,
    CONNECTION_FAILED,
    CONNECTION_ERROR,
    DISCONNECTION_COMPLETE,
    UNEXPECTED_DISCONNECTION,
    NETWORK_LOST,
    NETWORK_RESTORED,
    AUTO_RECONNECT,
    RETRY_ATTEMPT,
    TIMEOUT,
    SERVICE_RESTART
}

/**
 * Centralized MQTT connection state management
 * Ensures consistent state transitions and UI synchronization
 */
class MqttStateManager {
    
    companion object {
        private const val TAG = "MqttStateManager"
        private const val CONNECTING_TIMEOUT_MS = 30000L // 30 seconds
        private const val DISCONNECTING_TIMEOUT_MS = 10000L // 10 seconds
    }
    
    private val _currentState = MutableStateFlow(MqttConnectionState.DISCONNECTED)
    val currentState: StateFlow<MqttConnectionState> = _currentState
    
    private val _errorMessage = MutableStateFlow<String?>(null)
    val errorMessage: StateFlow<String?> = _errorMessage
    
    private val _lastSuccessfulConnection = MutableStateFlow<String?>(null)
    val lastSuccessfulConnection: StateFlow<String?> = _lastSuccessfulConnection
    
    private var onStateChanged: ((MqttConnectionState, String) -> Unit)? = null
    private var onNotificationUpdate: ((String, String) -> Unit)? = null
    private var uiUpdateCallback: ((MqttConnectionState, String?, String?) -> Unit)? = null
    
    /**
     * Set callbacks for state changes
     */
    fun setCallbacks(
        onStateChanged: (MqttConnectionState, String) -> Unit,
        onNotificationUpdate: (String, String) -> Unit
    ) {
        this.onStateChanged = onStateChanged
        this.onNotificationUpdate = onNotificationUpdate
    }
    
    /**
     * Set UI update callback for service integration
     */
    fun setUiUpdateCallback(callback: (MqttConnectionState, String?, String?) -> Unit) {
        this.uiUpdateCallback = callback
    }
    
    /**
     * Attempt state transition with validation
     */
    fun transitionTo(newState: MqttConnectionState, trigger: StateTrigger, details: String = "") {
        val currentState = _currentState.value
        
        if (isValidTransition(currentState, newState, trigger)) {
            logStateTransition(currentState, newState, trigger, details)
            _currentState.value = newState
            
            // Clear error message on successful transitions
            if (newState != MqttConnectionState.ERROR) {
                _errorMessage.value = null
            }
            
            // Update UI and notifications
            updateComponents(newState, trigger, details)
            
        } else {
            logInvalidTransition(currentState, newState, trigger, details)
        }
    }
    
    /**
     * Set error state with message
     */
    fun setError(message: String, trigger: StateTrigger = StateTrigger.CONNECTION_FAILED) {
        _errorMessage.value = message
        transitionTo(MqttConnectionState.ERROR, trigger, message)
    }
    
    /**
     * Record successful connection details
     */
    fun setConnectionSuccess(brokerUrl: String) {
        _lastSuccessfulConnection.value = brokerUrl
        // Only transition if not already CONNECTED
        if (_currentState.value != MqttConnectionState.CONNECTED) {
            transitionTo(MqttConnectionState.CONNECTED, StateTrigger.CONNECTION_SUCCESS, brokerUrl)
        }
    }
    
    /**
     * Validate if state transition is allowed
     */
    private fun isValidTransition(
        from: MqttConnectionState,
        to: MqttConnectionState,
        trigger: StateTrigger
    ): Boolean {
        return when (from) {
            MqttConnectionState.DISCONNECTED -> to in listOf(
                MqttConnectionState.CONNECTING,
                MqttConnectionState.FORCE_RESETTING
            )
            MqttConnectionState.CONNECTING -> to in listOf(
                MqttConnectionState.CONNECTED,
                MqttConnectionState.ERROR,
                MqttConnectionState.DISCONNECTED, // For user cancel
                MqttConnectionState.FORCE_RESETTING
            )
            MqttConnectionState.CONNECTED -> to in listOf(
                MqttConnectionState.DISCONNECTING,
                MqttConnectionState.ERROR,
                MqttConnectionState.NETWORK_LOST,
                MqttConnectionState.FORCE_RESETTING
            )
            MqttConnectionState.DISCONNECTING -> to in listOf(
                MqttConnectionState.DISCONNECTED,
                MqttConnectionState.ERROR, // If disconnection fails
                MqttConnectionState.FORCE_RESETTING
            )
            MqttConnectionState.ERROR -> to in listOf(
                MqttConnectionState.CONNECTING, // Retry
                MqttConnectionState.DISCONNECTED, // Give up
                MqttConnectionState.FORCE_RESETTING
            )
            MqttConnectionState.NETWORK_LOST -> to in listOf(
                MqttConnectionState.CONNECTING, // Network restored
                MqttConnectionState.DISCONNECTED, // Give up
                MqttConnectionState.ERROR, // Network issues
                MqttConnectionState.FORCE_RESETTING
            )
            MqttConnectionState.FORCE_RESETTING -> to in listOf(
                MqttConnectionState.DISCONNECTED
            )
        }
    }
    
    /**
     * Update UI components and notifications based on state
     */
    private fun updateComponents(state: MqttConnectionState, trigger: StateTrigger, details: String) {
        val statusText = getStatusText(state)
        val notificationTitle = "MQTT Service"
        val notificationContent = getNotificationContent(state, details)
        
        // Notify callbacks
        onStateChanged?.invoke(state, statusText)
        onNotificationUpdate?.invoke(notificationTitle, notificationContent)
        uiUpdateCallback?.invoke(state, details.takeIf { it.isNotEmpty() }, _errorMessage.value)
    }
    
    /**
     * Get user-friendly status text for UI
     */
    fun getStatusText(state: MqttConnectionState): String {
        return when (state) {
            MqttConnectionState.DISCONNECTED -> "Inactive"
            MqttConnectionState.CONNECTING -> "Connecting..."
            MqttConnectionState.CONNECTED -> "Active"
            MqttConnectionState.DISCONNECTING -> "Disconnecting..."
            MqttConnectionState.ERROR -> "Error"
            MqttConnectionState.NETWORK_LOST -> "No Network"
            MqttConnectionState.FORCE_RESETTING -> "Force Resetting..."
        }
    }
    
    /**
     * Get notification content based on state
     */
    private fun getNotificationContent(state: MqttConnectionState, details: String): String {
        return when (state) {
            MqttConnectionState.DISCONNECTED -> "Disconnected"
            MqttConnectionState.CONNECTING -> "Connecting to ${details.ifEmpty { "broker" }}"
            MqttConnectionState.CONNECTED -> "Connected to ${details.ifEmpty { "broker" }}"
            MqttConnectionState.DISCONNECTING -> "Disconnecting..."
            MqttConnectionState.ERROR -> "Connection Error: ${details.ifEmpty { "Unknown" }}"
            MqttConnectionState.NETWORK_LOST -> "Waiting for network"
            MqttConnectionState.FORCE_RESETTING -> "Force resetting MQTT..."
        }
    }
    
    /**
     * Get button configuration for current state
     */
    fun getButtonConfig(): ButtonConfig {
        return when (_currentState.value) {
            MqttConnectionState.DISCONNECTED -> ButtonConfig("Connect", true)
            MqttConnectionState.CONNECTING -> ButtonConfig("Connecting...", false)
            MqttConnectionState.CONNECTED -> ButtonConfig("Disconnect", true)
            MqttConnectionState.DISCONNECTING -> ButtonConfig("Disconnecting...", false)
            MqttConnectionState.ERROR -> ButtonConfig("Retry", true)
            MqttConnectionState.NETWORK_LOST -> ButtonConfig("Retry", true) // Allow retry when network lost
            MqttConnectionState.FORCE_RESETTING -> ButtonConfig("Force Resetting...", false)
        }
    }
    
    /**
     * Check if auto-reconnection should be attempted
     */
    fun shouldAutoReconnect(networkAvailable: Boolean, networkValidated: Boolean): Boolean {
        return networkAvailable && networkValidated && 
               _currentState.value in listOf(
                   MqttConnectionState.NETWORK_LOST,
                   MqttConnectionState.ERROR
               )
    }
    
    /**
     * Check if manual connection is allowed
     */
    fun canUserConnect(networkAvailable: Boolean): Boolean {
        return networkAvailable && 
               _currentState.value in listOf(
                   MqttConnectionState.DISCONNECTED,
                   MqttConnectionState.ERROR,
                   MqttConnectionState.NETWORK_LOST
               )
    }
    
    /**
     * Check if manual disconnection is allowed
     */
    fun canUserDisconnect(): Boolean {
        return _currentState.value == MqttConnectionState.CONNECTED
    }
    
    /**
     * Force state validation and correction
     */
    fun validateState(isNetworkAvailable: Boolean, isMqttConnected: Boolean) {
        val currentState = _currentState.value
        // Only trigger reconnection if state is exactly NETWORK_LOST
        if (isNetworkAvailable && currentState == MqttConnectionState.NETWORK_LOST) {
            Log.d(TAG, "validateState: Network restored, triggering reconnection from NETWORK_LOST")
            transitionTo(MqttConnectionState.CONNECTING, StateTrigger.NETWORK_RESTORED, "Network restored, auto-reconnect")
        }
        // If network is lost while connected, move to NETWORK_LOST
        if (!isNetworkAvailable && currentState == MqttConnectionState.CONNECTED) {
            Log.d(TAG, "validateState: Network lost during connection, moving to NETWORK_LOST")
            transitionTo(MqttConnectionState.NETWORK_LOST, StateTrigger.NETWORK_LOST, "Network lost during connection")
        }
        // Only move to ERROR from CONNECTED if network is available but MQTT is not connected
        if (isNetworkAvailable && !isMqttConnected && currentState == MqttConnectionState.CONNECTED) {
            Log.d(TAG, "validateState: MQTT disconnected unexpectedly (network available), moving to ERROR")
            transitionTo(MqttConnectionState.ERROR, StateTrigger.CONNECTION_ERROR, "MQTT disconnected unexpectedly")
        }
        // If MQTT is connected but state is not CONNECTED, correct to CONNECTED
        if (isMqttConnected && currentState != MqttConnectionState.CONNECTED && currentState != MqttConnectionState.CONNECTING) {
            Log.d(TAG, "validateState: MQTT connected, correcting state to CONNECTED")
            transitionTo(MqttConnectionState.CONNECTED, StateTrigger.CONNECTION_SUCCESS, "MQTT connected, correcting state")
        }
        // Always update UI/notification
        updateComponents(_currentState.value, StateTrigger.SERVICE_RESTART, "State validated")
    }
    
    private fun logStateTransition(
        from: MqttConnectionState,
        to: MqttConnectionState,
        trigger: StateTrigger,
        details: String
    ) {
        Log.d(TAG, "STATE_CHANGE: $from → $to (TRIGGER: $trigger) ${if (details.isNotEmpty()) "[$details]" else ""}")
    }
    
    private fun logInvalidTransition(
        from: MqttConnectionState,
        to: MqttConnectionState,
        trigger: StateTrigger,
        details: String
    ) {
        Log.w(TAG, "INVALID_TRANSITION: $from → $to (TRIGGER: $trigger) ${if (details.isNotEmpty()) "[$details]" else ""}")
    }
}

/**
 * Button configuration data class
 */
data class ButtonConfig(
    val text: String,
    val enabled: Boolean
)
