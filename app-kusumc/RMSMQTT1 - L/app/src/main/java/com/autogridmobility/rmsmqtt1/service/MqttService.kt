package com.autogridmobility.rmsmqtt1.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Binder
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import com.autogridmobility.rmsmqtt1.MainActivity
import com.autogridmobility.rmsmqtt1.R
import com.autogridmobility.rmsmqtt1.data.DaqData
import com.autogridmobility.rmsmqtt1.data.HeartbeatData
import com.autogridmobility.rmsmqtt1.data.OnDemandCommand
import com.autogridmobility.rmsmqtt1.data.PumpData
import com.autogridmobility.rmsmqtt1.service.MqttConnectionState
import com.autogridmobility.rmsmqtt1.service.MqttStateManager
import com.autogridmobility.rmsmqtt1.service.StateTrigger
import com.autogridmobility.rmsmqtt1.utils.NetworkMonitor
import com.autogridmobility.rmsmqtt1.utils.MqttPreferencesManager
import com.hivemq.client.mqtt.MqttClient
import com.hivemq.client.mqtt.mqtt3.Mqtt3AsyncClient
import com.hivemq.client.mqtt.mqtt3.message.connect.connack.Mqtt3ConnAck
import com.hivemq.client.mqtt.mqtt3.message.publish.Mqtt3Publish
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.cancelChildren
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import java.net.InetAddress
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter
import java.util.concurrent.CompletableFuture

class MqttService : Service() {
    /**
     * Brute-force cleanup: cancel all jobs, disconnect and null MQTT client, reset state
     */
    fun forceResetMqtt() {
        Log.w(TAG, "Force resetting all MQTT resources!")
        // Transition to FORCE_RESETTING state
        stateManager.transitionTo(MqttConnectionState.FORCE_RESETTING, StateTrigger.SERVICE_RESTART, "User requested force reset")
        // Cancel all jobs in the service scope
        try {
            serviceScope.coroutineContext.cancelChildren()
        } catch (e: Exception) {
            Log.w(TAG, "Exception while cancelling jobs", e)
        }
        // Disconnect and null the MQTT client
        try {
            mqttClient?.disconnect()?.whenComplete { _, _ -> }
        } catch (e: Exception) {
            Log.w(TAG, "Exception during force disconnect", e)
        }
        mqttClient = null
        isConnecting = false
        isReconnecting = false
        reconnectAttempts = 0
        lastConnectionParams = null
        // Clear all data flows and histories
        _heartbeatData.value = null
        _pumpData.value = null
        _daqData.value = null
        _heartbeatHistory.value = emptyList()
        _pumpHistory.value = emptyList()
        _daqHistory.value = emptyList()
        _onDemandHistory.value = emptyList()
        // Notify UI and set state to DISCONNECTED
        stateManager.transitionTo(MqttConnectionState.DISCONNECTED, StateTrigger.SERVICE_RESTART, "Force reset complete")
        updateNotification("MQTT Service", "Force reset complete. Ready to reconnect.")
    }
    
    companion object {
        private const val TAG = "MqttService"
        private const val NOTIFICATION_ID = 1
        private const val CHANNEL_ID = "mqtt_service_channel"
        private const val DEMO_IMEI = "869630050762180"
    }
    
    // Configurable topic prefix
    private var topicPrefix: String = DEMO_IMEI
    private lateinit var preferencesManager: MqttPreferencesManager
    
    // Method to update topic prefix
    fun updateTopicPrefix(newPrefix: String) {
        topicPrefix = newPrefix
        Log.d(TAG, "Updated topic prefix to: $newPrefix")
    }

    private val binder = MqttServiceBinder()
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private lateinit var networkMonitor: NetworkMonitor
    private val stateManager = MqttStateManager()

    private var mqttClient: Mqtt3AsyncClient? = null

    // Notification suppression
    private var lastNotificationTitle: String? = null
    private var lastNotificationContent: String? = null
    private val json = Json { ignoreUnknownKeys = true }
    
    // Data simulation service
    val dataSimulationService = DataSimulationService()
    
    // Legacy connection status for backward compatibility
    private val _connectionStatus = MutableStateFlow("Inactive")
    val connectionStatus: StateFlow<String> = _connectionStatus
    
    // Reconnection parameters
    private var isConnecting = false
    private var isReconnecting = false
    private var reconnectAttempts = 0
    private val maxReconnectAttempts = 10
    private val reconnectDelayMs = 5000L // 5 seconds
    
    // Connection parameters for reconnection
    private var lastConnectionParams: ConnectionParams? = null
    
    data class ConnectionParams(
        val url: String,
        val port: Int,
        val username: String?,
        val password: String?,
        val clientId: String
    )
    
    // Data flows
    private val _heartbeatData = MutableStateFlow<HeartbeatData?>(null)
    val heartbeatData: StateFlow<HeartbeatData?> = _heartbeatData
    
    private val _pumpData = MutableStateFlow<PumpData?>(null)
    val pumpData: StateFlow<PumpData?> = _pumpData
    
    private val _daqData = MutableStateFlow<DaqData?>(null)
    val daqData: StateFlow<DaqData?> = _daqData
    
    // Raw data storage for CSV export
    private val _heartbeatHistory = MutableStateFlow<List<Pair<String, String>>>(emptyList())
    val heartbeatHistory: StateFlow<List<Pair<String, String>>> = _heartbeatHistory
    
    private val _pumpHistory = MutableStateFlow<List<Pair<String, String>>>(emptyList())
    val pumpHistory: StateFlow<List<Pair<String, String>>> = _pumpHistory
    
    private val _daqHistory = MutableStateFlow<List<Pair<String, String>>>(emptyList())
    val daqHistory: StateFlow<List<Pair<String, String>>> = _daqHistory
    
    private val _onDemandHistory = MutableStateFlow<List<Pair<String, String>>>(emptyList())
    val onDemandHistory: StateFlow<List<Pair<String, String>>> = _onDemandHistory
    
    inner class MqttServiceBinder : Binder() {
        fun getService(): MqttService = this@MqttService
    }
    
    override fun onBind(intent: Intent): IBinder = binder
    
    override fun onCreate() {
        super.onCreate()
        
        // Initialize preferences manager and load saved topic prefix
        preferencesManager = MqttPreferencesManager(this)
        topicPrefix = preferencesManager.getTopicPrefix()
        Log.d(TAG, "Loaded topic prefix from preferences: $topicPrefix")
        
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, createNotification("MQTT Service", "Initializing..."))
        
        // Initialize data simulation service
        dataSimulationService.setMqttService(this)
        
        // Initialize state manager with callbacks
        initializeStateManager()
        
        // Initialize network monitoring
        initializeNetworkMonitoring()
        
        // Start periodic state validation
        startPeriodicStateValidation()
    }
    
    private fun startPeriodicStateValidation() {
        serviceScope.launch {
            while (true) {
                delay(10000) // Validate every 10 seconds
                validateCurrentState()
            }
        }
    }
    
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        return START_STICKY
    }
    
    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "MQTT Service",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "PMKUSUM IoT MQTT Connection Service"
            }
            val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            notificationManager.createNotificationChannel(channel)
        }
    }
    
    private fun createNotification(title: String, content: String): Notification {
        val intent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK
        }
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(content)
            .setSmallIcon(R.drawable.ic_menu_camera) // Using existing icon
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }
    
    private fun updateNotification(title: String, content: String) {
        // Only update if content or title changed
        if (title == lastNotificationTitle && content == lastNotificationContent) return
        lastNotificationTitle = title
        lastNotificationContent = content
        val notification = createNotification(title, content)
        val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(NOTIFICATION_ID, notification)
    }
    
    private fun showErrorNotification(message: String) {
        val notification = createNotification("MQTT Error", message)
        val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(NOTIFICATION_ID + 1, notification)
    }
    
    private fun showInfoNotification(message: String) {
        val notification = createNotification("MQTT Info", message)
        val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(NOTIFICATION_ID + 2, notification)
    }
    
    private fun initializeStateManager() {
        stateManager.setUiUpdateCallback { state, context, error ->
            // Update legacy connection status for backward compatibility
            _connectionStatus.value = when (state) {
                MqttConnectionState.CONNECTED -> "Connected"
                MqttConnectionState.CONNECTING -> "Connecting"
                MqttConnectionState.DISCONNECTING -> "Disconnecting"
                MqttConnectionState.ERROR -> "Error"
                MqttConnectionState.NETWORK_LOST -> "Network Lost"
                MqttConnectionState.DISCONNECTED -> "Inactive"
                MqttConnectionState.FORCE_RESETTING -> "Resetting..."
            }
            
            // Update notification with connection status
            val notificationTitle = "MQTT Service"
            val notificationText = if (error != null) {
                "Error: $error"
            } else {
                context ?: when (state) {
                    MqttConnectionState.DISCONNECTED -> "Disconnected"
                    MqttConnectionState.CONNECTING -> "Connecting..."
                    MqttConnectionState.CONNECTED -> "Connected to $context"
                    MqttConnectionState.DISCONNECTING -> "Disconnecting..."
                    MqttConnectionState.ERROR -> "Connection Error"
                    MqttConnectionState.NETWORK_LOST -> "Waiting for Network"
                    MqttConnectionState.FORCE_RESETTING -> "Resetting MQTT resources..."
                }
            }
            
            updateNotification(notificationTitle, notificationText)
            updateNotification("MQTT Service", notificationText)
        }
    }
    
    private fun initializeNetworkMonitoring() {
        networkMonitor = NetworkMonitor(this)

        networkMonitor.setNetworkCallback { isAvailable ->
            val isMqttConnected = mqttClient?.config?.state?.isConnected == true
            stateManager.validateState(isAvailable, isMqttConnected)
            val currentState = stateManager.currentState.value
            if (isAvailable) {
                Log.d(TAG, "Network became available")
                reconnectAttempts = 0
                // If we were in NETWORK_LOST, trigger reconnection
                if (currentState == MqttConnectionState.CONNECTING || currentState == MqttConnectionState.NETWORK_LOST) {
                    attemptReconnectionViaStateManager()
                }
            } else {
                Log.d(TAG, "Network lost")
                if (currentState == MqttConnectionState.CONNECTED) {
                    stateManager.transitionTo(MqttConnectionState.NETWORK_LOST, StateTrigger.NETWORK_LOST)
                }
                updateNotification("MQTT Service", "Network unavailable")
            }
        }
    }
    
    private fun attemptReconnectionViaStateManager() {
        val currentState = stateManager.currentState.value
        // Prevent multiple parallel reconnections
        if (isReconnecting) {
            Log.d(TAG, "Reconnection already in progress, skipping new attempt.")
            return
        }
        // Allow reconnection if DISCONNECTED, or CONNECTING but not actually connected
        val canReconnect =
            (currentState == MqttConnectionState.DISCONNECTED ||
             (currentState == MqttConnectionState.CONNECTING && (mqttClient == null || mqttClient?.config?.state?.isConnected != true)) ||
             currentState == MqttConnectionState.NETWORK_LOST)
        // Only attempt reconnection if we have stored connection parameters
        if (canReconnect &&
            networkMonitor.isNetworkCurrentlyAvailable() &&
            lastConnectionParams != null &&
            reconnectAttempts < maxReconnectAttempts) {
            Log.d(TAG, "Attempting automatic reconnection (attempt ${reconnectAttempts + 1}/$maxReconnectAttempts)")
            isReconnecting = true
            serviceScope.launch {
                try {
                    // Extra delay to ensure network is truly available
                    Log.d(TAG, "Waiting 3s after network available before reconnecting...")
                    delay(3000)
                    // Always destroy any previous client before reconnecting
                    try {
                        mqttClient?.disconnect()
                    } catch (ex: Exception) {
                        Log.w(TAG, "Exception during client disconnect before reconnect", ex)
                    }
                    mqttClient = null
                    // Wait before reconnection attempt (existing delay)
                    delay(reconnectDelayMs)
                    // Attempt reconnection with stored parameters
                    lastConnectionParams?.let { params ->
                        reconnectAttempts++
                        Log.d(TAG, "Reconnecting to ${params.url}:${params.port}")
                        performConnection(params)
                    }
                } catch (e: Exception) {
                    Log.e(TAG, "Reconnection attempt failed", e)
                    reconnectAttempts++
                    if (reconnectAttempts >= maxReconnectAttempts) {
                        Log.w(TAG, "Max reconnection attempts reached")
                        stateManager.setError("Max reconnection attempts reached")
                    }
                } finally {
                    isReconnecting = false
                }
            }
        } else {
            Log.d(TAG, "Reconnection not applicable: state=$currentState, network=${networkMonitor.isNetworkCurrentlyAvailable()}, params=${lastConnectionParams != null}, attempts=$reconnectAttempts")
        }
        // Validate current state against reality
        validateCurrentState()
    }
    
    private fun performConnection(params: ConnectionParams) {
        serviceScope.launch {
            try {
                // Always destroy any previous client before reconnecting
                mqttClient = null
                // Set connecting state
                stateManager.transitionTo(MqttConnectionState.CONNECTING, StateTrigger.AUTO_RECONNECT, params.url)
                // Wait 3 seconds before DNS resolution
                Log.d(TAG, "Waiting 3s before DNS resolution for ${params.url}")
                delay(3000)
                // Resolve IPv4 address
                val ipAddress = try {
                    resolveIPv4Address(params.url).get()
                } catch (e: Exception) {
                    Log.e(TAG, "DNS resolution failed for ${params.url}", e)
                    stateManager.setError("DNS resolution failed: ${e.message}")
                    showErrorNotification("DNS resolution failed: ${e.message}")
                    // Do not attempt connection if DNS fails
                    if (reconnectAttempts < maxReconnectAttempts) {
                        attemptReconnectionViaStateManager()
                    }
                    return@launch
                }
                Log.d(TAG, "Resolved ${params.url} to $ipAddress")
                // Create new MQTT client
                val clientBuilder = MqttClient.builder()
                    .identifier(params.clientId)
                    .serverHost(ipAddress)
                    .serverPort(params.port)
                    .useMqttVersion3()
                mqttClient = clientBuilder.buildAsync()
                // Build connection
                val connectBuilder = mqttClient!!.connectWith()
                    .cleanSession(true)
                if (!params.username.isNullOrBlank()) {
                    connectBuilder.simpleAuth()
                        .username(params.username)
                        .password(params.password?.toByteArray() ?: byteArrayOf())
                        .applySimpleAuth()
                }
                // Connect with timeout handling
                connectBuilder.send().whenComplete { connAck: Mqtt3ConnAck?, throwable: Throwable? ->
                    if (throwable != null) {
                        Log.e(TAG, "Reconnection failed", throwable)
                        stateManager.setError("Reconnection failed: ${throwable.message}")
                        showErrorNotification("Reconnection failed: ${throwable.message}")
                        // Schedule another reconnection attempt if we haven't reached max attempts
                        if (reconnectAttempts < maxReconnectAttempts) {
                            attemptReconnectionViaStateManager()
                        }
                    } else {
                        Log.d(TAG, "Reconnected successfully")
                        reconnectAttempts = 0 // Reset on successful reconnection
                        stateManager.setConnectionSuccess("${params.url}:${params.port}")
                        // Set up disconnection listener
                        mqttClient?.let { client ->
                            setupDisconnectionListener(client)
                        }
                        subscribeToTopics()
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "Reconnection error", e)
                stateManager.setError("Reconnection error: ${e.message}")
                showErrorNotification("Reconnection error: ${e.message}")
                // Schedule another reconnection attempt if we haven't reached max attempts
                if (reconnectAttempts < maxReconnectAttempts) {
                    attemptReconnectionViaStateManager()
                }
            }
        }
    }
    
    private fun validateCurrentState() {
        val isNetworkAvailable = networkMonitor.isNetworkCurrentlyAvailable()
        val isMqttConnected = mqttClient?.config?.state?.isConnected == true
        stateManager.validateState(isNetworkAvailable, isMqttConnected)
    }
    
    fun getStateManager(): MqttStateManager = stateManager
    
    fun isCurrentlyConnected(): Boolean {
        return stateManager.currentState.value == MqttConnectionState.CONNECTED
    }
    
    fun isMqttConnected(): Boolean {
        return mqttClient?.config?.state?.isConnected == true && 
               networkMonitor.isNetworkCurrentlyAvailable()
    }
    
    private fun resolveIPv4Address(hostname: String): CompletableFuture<String> {
        return CompletableFuture.supplyAsync {
            try {
                val addresses = InetAddress.getAllByName(hostname)
                // Filter for IPv4 addresses only
                val ipv4Address = addresses.firstOrNull { it.address.size == 4 }
                ipv4Address?.hostAddress ?: throw Exception("No IPv4 address found for $hostname")
            } catch (e: Exception) {
                Log.e(TAG, "DNS resolution failed for $hostname", e)
                throw e
            }
        }
    }
    
    fun connect(url: String, port: Int, username: String?, password: String?, clientId: String) {
        // Store connection parameters for potential reconnection
        lastConnectionParams = ConnectionParams(url, port, username, password, clientId)

        // Check if connection is allowed via state manager
        if (!stateManager.canUserConnect(networkMonitor.isNetworkCurrentlyAvailable())) {
            Log.w(TAG, "Connection not allowed in current state: ${stateManager.currentState.value}")
            return
        }

        // Reset reconnection attempts for new user-initiated connection
        reconnectAttempts = 0

        // Transition to connecting state
        stateManager.transitionTo(MqttConnectionState.CONNECTING, StateTrigger.USER_CONNECT, url)

        serviceScope.launch {
            try {
                // Always destroy any previous client before connecting
                try {
                    mqttClient?.disconnect()
                } catch (_: Exception) {}
                mqttClient = null
                // Set connecting state
                stateManager.transitionTo(MqttConnectionState.CONNECTING, StateTrigger.AUTO_RECONNECT, url)
                // Resolve IPv4 address
                val ipAddress = resolveIPv4Address(url).get()
                Log.d(TAG, "Resolved $url to $ipAddress")
                // Create new MQTT client
                val clientBuilder = MqttClient.builder()
                    .identifier(clientId)
                    .serverHost(ipAddress)
                    .serverPort(port)
                    .useMqttVersion3()
                mqttClient = clientBuilder.buildAsync()
                // Build connection
                val connectBuilder = mqttClient!!.connectWith()
                    .cleanSession(true)
                if (!username.isNullOrBlank()) {
                    connectBuilder.simpleAuth()
                        .username(username)
                        .password(password?.toByteArray() ?: byteArrayOf())
                        .applySimpleAuth()
                }
                // Connect with timeout handling
                connectBuilder.send().whenComplete { connAck: Mqtt3ConnAck?, throwable: Throwable? ->
                    if (throwable != null) {
                        Log.e(TAG, "Connection failed", throwable)
                        stateManager.setError("Connection failed: ${throwable.message}")
                        showErrorNotification("Connection failed: ${throwable.message}")
                        // Schedule another reconnection attempt if we haven't reached max attempts
                        if (reconnectAttempts < maxReconnectAttempts) {
                            attemptReconnectionViaStateManager()
                        }
                    } else {
                        Log.d(TAG, "Connected successfully")
                        reconnectAttempts = 0 // Reset on successful connection
                        stateManager.setConnectionSuccess("$url:$port")
                        // Set up disconnection listener
                        mqttClient?.let { client ->
                            setupDisconnectionListener(client)
                        }
                        subscribeToTopics()
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "Connection error", e)
                stateManager.setError("Connection error: ${e.message}")
                showErrorNotification("Connection error: ${e.message}")
                // Schedule another reconnection attempt if we haven't reached max attempts
                if (reconnectAttempts < maxReconnectAttempts) {
                    attemptReconnectionViaStateManager()
                }
            }
        }
    }
    // Subscribe to topics after (re)connection
    private fun subscribeToTopics() {
        val topics = listOf(
            "$topicPrefix/heartbeat",
            "$topicPrefix/pump",
            "$topicPrefix/data", // Added subscription to /data
            "$topicPrefix/daq",
            "$topicPrefix/ondemand"
        )
        topics.forEach { topic ->
            mqttClient?.subscribeWith()
                ?.topicFilter(topic)
                ?.callback { publish ->
                    try {
                        val payload = publish.payload.orElse(null)?.let {
                            val bytes = ByteArray(it.remaining())
                            val pos = it.position()
                            it.get(bytes)
                            it.position(pos) // reset position for safety
                            String(bytes)
                        } ?: "(no payload)"
                        Log.d(TAG, "Received on $topic: $payload")

                        val timestamp = LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss"))
                        when {
                            topic.endsWith("/heartbeat") -> {
                                _heartbeatHistory.value = _heartbeatHistory.value + (timestamp to payload)
                                try {
                                    val data = json.decodeFromString(HeartbeatData.serializer(), payload)
                                    _heartbeatData.value = data
                                } catch (_: Exception) {}
                            }
                            topic.endsWith("/pump") || topic.endsWith("/data") -> {
                                _pumpHistory.value = _pumpHistory.value + (timestamp to payload)
                                try {
                                    val data = json.decodeFromString(PumpData.serializer(), payload)
                                    _pumpData.value = data
                                } catch (_: Exception) {}
                            }
                            topic.endsWith("/daq") -> {
                                _daqHistory.value = _daqHistory.value + (timestamp to payload)
                                try {
                                    val data = json.decodeFromString(DaqData.serializer(), payload)
                                    _daqData.value = data
                                } catch (_: Exception) {}
                            }
                            topic.endsWith("/ondemand") -> {
                                _onDemandHistory.value = _onDemandHistory.value + (timestamp to payload)
                            }
                        }
                        // Optionally, show notification for received message
                        // showInfoNotification("Received on $topic: $payload")
                    } catch (e: Exception) {
                        Log.e(TAG, "Error handling message on $topic", e)
                    }
                }
                ?.send()
        }
        Log.d(TAG, "Subscribed to topics: $topics")
    }
    
    fun publishMessage(topic: String, payload: String) {
        serviceScope.launch {
            try {
                val fullTopic = "$topicPrefix/$topic"
                
                mqttClient?.publishWith()
                    ?.topic(fullTopic)
                    ?.payload(payload.toByteArray())
                    ?.send()
                    ?.whenComplete { _, throwable ->
                        if (throwable != null) {
                            Log.e(TAG, "Failed to publish to $fullTopic", throwable)
                            showErrorNotification("Failed to publish to $fullTopic")
                        } else {
                            Log.d(TAG, "Published to $fullTopic")
                            showInfoNotification("Published to $fullTopic")
                        }
                    }
            } catch (e: Exception) {
                Log.e(TAG, "Error publishing message", e)
                showErrorNotification("Error publishing: ${e.message}")
            }
        }
    }
    
    fun publishOnDemandCommand(command: OnDemandCommand) {
        serviceScope.launch {
            try {
                val topic = "$topicPrefix/ondemand"
                val payload = json.encodeToString(command)
                
                mqttClient?.publishWith()
                    ?.topic(topic)
                    ?.payload(payload.toByteArray())
                    ?.send()
                    ?.whenComplete { _, throwable ->
                        if (throwable != null) {
                            Log.e(TAG, "Failed to publish on-demand command", throwable)
                            showErrorNotification("Failed to publish command")
                        } else {
                            Log.d(TAG, "Published on-demand command")
                        }
                    }
                
                // Add to history
                val timestamp = LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss"))
                _onDemandHistory.value = _onDemandHistory.value + (timestamp to payload)
                
            } catch (e: Exception) {
                Log.e(TAG, "Error publishing on-demand command", e)
                showErrorNotification("Error publishing command: ${e.message}")
            }
        }
    }
    
    fun disconnect() {
        if (stateManager.currentState.value == MqttConnectionState.CONNECTED) {
            Log.d(TAG, "Disconnecting by user request, current state: ${stateManager.currentState.value}")
        }
        
        // Reset reconnection attempts on manual disconnect to prevent auto-reconnection
        reconnectAttempts = maxReconnectAttempts
        
        stateManager.transitionTo(MqttConnectionState.DISCONNECTING, StateTrigger.USER_DISCONNECT)
        
        serviceScope.launch {
            try {
                mqttClient?.disconnect()?.whenComplete { _, throwable ->
                    if (throwable != null) {
                        Log.e(TAG, "Disconnect error", throwable)
                        stateManager.transitionTo(MqttConnectionState.ERROR, StateTrigger.CONNECTION_ERROR, throwable.message ?: "Unknown error")
                        showErrorNotification("Disconnect error: ${throwable.message}")
                    } else {
                        Log.d(TAG, "Disconnected successfully")
                        stateManager.transitionTo(MqttConnectionState.DISCONNECTED, StateTrigger.DISCONNECTION_COMPLETE)
                    }
                    // Clean up client reference after disconnect completes
                    mqttClient = null
                    isConnecting = false
                }
            } catch (e: Exception) {
                Log.e(TAG, "Error during disconnect", e)
                stateManager.transitionTo(MqttConnectionState.ERROR, StateTrigger.CONNECTION_ERROR, e.message ?: "Unknown error")
                showErrorNotification("Disconnect error: ${e.message}")
                mqttClient = null
                isConnecting = false
            }
        }
    }
    
    private fun setupDisconnectionListener(client: Mqtt3AsyncClient) {
        Log.d(TAG, "Setting up disconnection monitoring for MQTT client")
        
        // Monitor connection state periodically
        serviceScope.launch {
            while (client.config.state.isConnected) {
                delay(5000) // Check every 5 seconds
                
                // Check if client is still connected
                if (!client.config.state.isConnected) {
                    Log.w(TAG, "MQTT client disconnected unexpectedly")
                    
                    // Handle unexpected disconnection
                    val currentState = stateManager.currentState.value
                    if (currentState == MqttConnectionState.CONNECTED) {
                        Log.w(TAG, "Unexpected disconnection detected")
                        
                        if (networkMonitor.isNetworkCurrentlyAvailable()) {
                            // Network is available, likely server-side disconnection
                            stateManager.transitionTo(MqttConnectionState.ERROR, StateTrigger.UNEXPECTED_DISCONNECTION, "Server disconnected")
                            showErrorNotification("Connection lost: Server disconnected")
                            
                        // Destroy client before attempting reconnection
                        mqttClient = null
                        // Attempt automatic reconnection
                        attemptReconnectionViaStateManager()
                        } else {
                            // Network lost
                            stateManager.transitionTo(MqttConnectionState.NETWORK_LOST, StateTrigger.NETWORK_LOST)
                        showErrorNotification("Connection lost: Network unavailable")
                        mqttClient = null
                        }
                    }
                    break
                }
            }
        }
    }
    
    private fun attemptReconnection() {
        val currentState = stateManager.currentState.value
        if (currentState != MqttConnectionState.CONNECTED && 
            currentState != MqttConnectionState.CONNECTING) {
            Log.d(TAG, "State check failed for reconnection, current state: $currentState")
            _connectionStatus.value = "Inactive"
            updateNotification("MQTT Service", "Connection lost")
        } else if (currentState == MqttConnectionState.DISCONNECTED) {
            _connectionStatus.value = "Disconnected"
            updateNotification("MQTT Service", "Disconnected")
        }
        
        Log.d(TAG, "State validation check completed, status: ${_connectionStatus.value}")
    }
    
    fun reconnect() {
        Log.d(TAG, "Manual reconnection requested")
        
        if (lastConnectionParams == null) {
            Log.w(TAG, "No previous connection parameters available for reconnection")
            showErrorNotification("No previous connection to reconnect to")
            return
        }
        
        val currentState = stateManager.currentState.value
        if (currentState != MqttConnectionState.DISCONNECTED && currentState != MqttConnectionState.ERROR) {
            Log.w(TAG, "Cannot reconnect in current state: $currentState")
            return
        }
        
        // Reset reconnection attempts for manual reconnection
        reconnectAttempts = 0
        
        // Use the stored connection parameters
        lastConnectionParams?.let { params ->
            Log.d(TAG, "Reconnecting to ${params.url}:${params.port}")
            connect(params.url, params.port, params.username, params.password, params.clientId)
        }
    }
    
    fun clearAllHistory() {
        _heartbeatHistory.value = emptyList()
        _pumpHistory.value = emptyList()
        _daqHistory.value = emptyList()
        _onDemandHistory.value = emptyList()
    }
    
    // Data simulation methods
    fun startDataSimulation() {
        dataSimulationService.start()
    }
    
    fun startDataSimulation(intervalSeconds: Int) {
        dataSimulationService.start(intervalSeconds)
    }
    
    fun stopDataSimulation() {
        dataSimulationService.stop()
    }
    
    fun isSimulating(): Boolean {
        return dataSimulationService.isRunning()
    }
    
    fun getSimulationPacketsPublished(): Int {
        return dataSimulationService.getPacketsPublished()
    }
    
    fun resetSimulationPacketCount() {
        dataSimulationService.resetPacketCount()
    }
    
    override fun onDestroy() {
        super.onDestroy()
        dataSimulationService.stop()
        
        // Clean up network monitoring
        if (::networkMonitor.isInitialized) {
            networkMonitor.cleanup()
        }
        
        // Cancel all coroutines
        serviceScope.launch {
            mqttClient?.disconnect()
        }
    }
}
