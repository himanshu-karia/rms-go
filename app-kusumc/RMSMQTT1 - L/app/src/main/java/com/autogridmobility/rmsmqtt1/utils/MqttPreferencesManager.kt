package com.autogridmobility.rmsmqtt1.utils

import android.content.Context
import android.content.SharedPreferences

class MqttPreferencesManager(private val context: Context) {
    
    companion object {
        private const val PREFS_NAME = "mqtt_settings"
        private const val KEY_BROKER_URL = "broker_url"
        private const val KEY_BROKER_PORT = "broker_port"
        private const val KEY_USERNAME = "username"
        private const val KEY_PASSWORD = "password"
        private const val KEY_CLIENT_ID = "client_id"
        private const val KEY_TOPIC_PREFIX = "topic_prefix"
        private const val KEY_AUTO_START = "auto_start_on_boot"
        
        // Default values aligned with internal EMQX deployment
        private const val DEFAULT_BROKER_URL = "rms-iot.local"
        private const val DEFAULT_BROKER_PORT = 18883
        private const val DEFAULT_USERNAME = ""
        private const val DEFAULT_PASSWORD = ""
        private const val DEFAULT_TOPIC_PREFIX = "869630050762180"
        private const val DEFAULT_AUTO_START = true
    }
    
    private val prefs: SharedPreferences = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
    
    // Broker URL
    fun getBrokerUrl(): String = prefs.getString(KEY_BROKER_URL, DEFAULT_BROKER_URL) ?: DEFAULT_BROKER_URL
    
    fun saveBrokerUrl(url: String) {
        prefs.edit().putString(KEY_BROKER_URL, url).apply()
    }
    
    // Broker Port
    fun getBrokerPort(): Int = prefs.getInt(KEY_BROKER_PORT, DEFAULT_BROKER_PORT)
    
    fun saveBrokerPort(port: Int) {
        prefs.edit().putInt(KEY_BROKER_PORT, port).apply()
    }
    
    // Username
    fun getUsername(): String = prefs.getString(KEY_USERNAME, DEFAULT_USERNAME) ?: DEFAULT_USERNAME
    
    fun saveUsername(username: String) {
        prefs.edit().putString(KEY_USERNAME, username).apply()
    }
    
    // Password
    fun getPassword(): String = prefs.getString(KEY_PASSWORD, DEFAULT_PASSWORD) ?: DEFAULT_PASSWORD
    
    fun savePassword(password: String) {
        prefs.edit().putString(KEY_PASSWORD, password).apply()
    }
    
    // Client ID
    fun getClientId(): String {
        val savedClientId = prefs.getString(KEY_CLIENT_ID, null)
        return if (savedClientId.isNullOrBlank()) {
            generateNewClientId()
        } else {
            savedClientId
        }
    }
    
    fun saveClientId(clientId: String) {
        prefs.edit().putString(KEY_CLIENT_ID, clientId).apply()
    }
    
    fun generateNewClientId(): String {
        val newClientId = "NEReceiver554_${System.currentTimeMillis()}"
        saveClientId(newClientId)
        return newClientId
    }
    
    // Topic Prefix (IMEI)
    fun getTopicPrefix(): String = prefs.getString(KEY_TOPIC_PREFIX, DEFAULT_TOPIC_PREFIX) ?: DEFAULT_TOPIC_PREFIX
    
    fun saveTopicPrefix(prefix: String) {
        prefs.edit().putString(KEY_TOPIC_PREFIX, prefix).apply()
    }
    
    // Auto start on boot
    fun getAutoStartOnBoot(): Boolean = prefs.getBoolean(KEY_AUTO_START, DEFAULT_AUTO_START)
    
    fun saveAutoStartOnBoot(autoStart: Boolean) {
        prefs.edit().putBoolean(KEY_AUTO_START, autoStart).apply()
    }
    
    // Validation helpers
    fun isValidPort(portString: String): Boolean {
        val port = portString.toIntOrNull() ?: return false
        return port in 1..65535
    }
    
    fun isValidUrl(url: String): Boolean {
        return url.isNotBlank() && url.trim().isNotEmpty()
    }
    
    fun isValidClientId(clientId: String): Boolean {
        return clientId.isNotBlank() && clientId.trim().isNotEmpty()
    }
    
    fun isValidTopicPrefix(prefix: String): Boolean {
        return prefix.isNotBlank() && prefix.trim().isNotEmpty()
    }
    
    // Get all settings as a bundle for connection
    data class MqttSettings(
        val brokerUrl: String,
        val brokerPort: Int,
        val username: String,
        val password: String,
        val clientId: String,
        val topicPrefix: String
    )
    
    fun getAllSettings(): MqttSettings {
        return MqttSettings(
            brokerUrl = getBrokerUrl(),
            brokerPort = getBrokerPort(),
            username = getUsername(),
            password = getPassword(),
            clientId = getClientId(),
            topicPrefix = getTopicPrefix()
        )
    }
}
