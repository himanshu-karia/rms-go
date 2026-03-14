package com.autogridmobility.rmsmqtt1.receivers

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.util.Log
import com.autogridmobility.rmsmqtt1.service.MqttService

class NetworkReceiver : BroadcastReceiver() {
    
    companion object {
        private const val TAG = "NetworkReceiver"
    }
    
    override fun onReceive(context: Context, intent: Intent) {
        Log.d(TAG, "Network state changed: ${intent.action}")
        
        // Note: CONNECTIVITY_ACTION is deprecated but still functional for fallback scenarios
        @Suppress("DEPRECATION")
        if (intent.action == ConnectivityManager.CONNECTIVITY_ACTION) {
            checkNetworkAndReconnect(context)
        }
    }
    
    private fun checkNetworkAndReconnect(context: Context) {
        val connectivityManager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        
        try {
            val activeNetwork = connectivityManager.activeNetwork
            val networkCapabilities = connectivityManager.getNetworkCapabilities(activeNetwork)
            
            if (activeNetwork != null && networkCapabilities != null) {
                val hasInternet = networkCapabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                val isValidated = networkCapabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)
                
                Log.d(TAG, "Network status - Internet: $hasInternet, Validated: $isValidated")
                
                if (hasInternet && isValidated) {
                    // Network is available and validated, check MQTT service
                    val serviceIntent = Intent(context, MqttService::class.java).apply {
                        action = "CHECK_AND_RECONNECT"
                    }
                    context.startForegroundService(serviceIntent)
                    Log.i(TAG, "Network available - requesting MQTT reconnection check")
                } else {
                    Log.w(TAG, "Network not validated or no internet capability")
                }
            } else {
                Log.w(TAG, "No active network available")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error checking network status", e)
        }
    }
}
