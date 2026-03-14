package com.autogridmobility.rmsmqtt1.receivers

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import com.autogridmobility.rmsmqtt1.service.MqttService
import com.autogridmobility.rmsmqtt1.utils.MqttPreferencesManager

class BootReceiver : BroadcastReceiver() {
    
    companion object {
        private const val TAG = "BootReceiver"
    }
    
    override fun onReceive(context: Context, intent: Intent) {
        Log.d(TAG, "Received broadcast: ${intent.action}")
        
        when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED -> {
                Log.i(TAG, "Device boot completed - starting MQTT service")
                startMqttService(context)
            }
            Intent.ACTION_MY_PACKAGE_REPLACED,
            Intent.ACTION_PACKAGE_REPLACED -> {
                Log.i(TAG, "App package replaced - restarting MQTT service")
                startMqttService(context)
            }
        }
    }
    
    private fun startMqttService(context: Context) {
        try {
            // Check if auto-start is enabled using the new preferences manager
            val prefsManager = MqttPreferencesManager(context)
            val autoStart = prefsManager.getAutoStartOnBoot()
            
            if (autoStart) {
                val serviceIntent = Intent(context, MqttService::class.java).apply {
                    action = "START_SERVICE"
                    putExtra("auto_connect", true)
                }
                context.startForegroundService(serviceIntent)
                Log.i(TAG, "MQTT service started automatically")
            } else {
                Log.i(TAG, "Auto-start disabled by user")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start MQTT service on boot", e)
        }
    }
}
