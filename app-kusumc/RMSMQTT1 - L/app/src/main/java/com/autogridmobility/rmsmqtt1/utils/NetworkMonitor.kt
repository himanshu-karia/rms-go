package com.autogridmobility.rmsmqtt1.utils

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.os.Build
import android.util.Log
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

class NetworkMonitor(private val context: Context) {
    
    companion object {
        private const val TAG = "NetworkMonitor"
    }
    
    private val connectivityManager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    
    private val _isNetworkAvailable = MutableStateFlow(false)
    val isNetworkAvailable: StateFlow<Boolean> = _isNetworkAvailable.asStateFlow()
    
    private val _isValidatedConnection = MutableStateFlow(false)
    val isValidatedConnection: StateFlow<Boolean> = _isValidatedConnection.asStateFlow()
    
    private var networkCallback: ConnectivityManager.NetworkCallback? = null
    private var onNetworkStateChanged: ((Boolean, Boolean) -> Unit)? = null
    private var simpleNetworkCallback: ((Boolean) -> Unit)? = null
    
    fun startMonitoring(onStateChanged: (isAvailable: Boolean, isValidated: Boolean) -> Unit) {
        onNetworkStateChanged = onStateChanged
        
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            startModernNetworkMonitoring()
        } else {
            startLegacyNetworkMonitoring()
        }
        
        // Initial check
        checkCurrentNetworkState()
    }
    
    fun stopMonitoring() {
        networkCallback?.let { callback ->
            try {
                connectivityManager.unregisterNetworkCallback(callback)
                Log.d(TAG, "Network monitoring stopped")
            } catch (e: Exception) {
                Log.e(TAG, "Error stopping network monitoring", e)
            }
        }
        networkCallback = null
        onNetworkStateChanged = null
    }
    
    private fun startModernNetworkMonitoring() {
        val networkRequest = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)
            .build()
        
        networkCallback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                Log.d(TAG, "Network available: $network")
                updateNetworkState(network)
            }
            
            override fun onCapabilitiesChanged(network: Network, networkCapabilities: NetworkCapabilities) {
                Log.d(TAG, "Network capabilities changed: $network")
                updateNetworkState(network, networkCapabilities)
            }
            
            override fun onLost(network: Network) {
                Log.d(TAG, "Network lost: $network")
                _isNetworkAvailable.value = false
                _isValidatedConnection.value = false
                onNetworkStateChanged?.invoke(false, false)
            }
            
            override fun onUnavailable() {
                Log.d(TAG, "Network unavailable")
                _isNetworkAvailable.value = false
                _isValidatedConnection.value = false
                onNetworkStateChanged?.invoke(false, false)
            }
        }
        
        try {
            connectivityManager.registerNetworkCallback(networkRequest, networkCallback!!)
            Log.i(TAG, "Modern network monitoring started")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to register network callback", e)
            startLegacyNetworkMonitoring()
        }
    }
    
    private fun startLegacyNetworkMonitoring() {
        // For older Android versions, we'll rely on the BroadcastReceiver
        Log.i(TAG, "Using legacy network monitoring via BroadcastReceiver")
        checkCurrentNetworkState()
    }
    
    private fun updateNetworkState(network: Network, capabilities: NetworkCapabilities? = null) {
        try {
            val networkCapabilities = capabilities ?: connectivityManager.getNetworkCapabilities(network)
            
            if (networkCapabilities != null) {
                val hasInternet = networkCapabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                val isValidated = networkCapabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)
                val hasTransport = networkCapabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) ||
                                networkCapabilities.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) ||
                                networkCapabilities.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET)
                
                Log.d(TAG, "Network state - Internet: $hasInternet, Validated: $isValidated, Transport: $hasTransport")
                
                val isAvailable = hasInternet && hasTransport
                val isValidConnection = isAvailable && isValidated
                
                _isNetworkAvailable.value = isAvailable
                _isValidatedConnection.value = isValidConnection
                
                onNetworkStateChanged?.invoke(isAvailable, isValidConnection)
            } else {
                Log.w(TAG, "Network capabilities are null")
                _isNetworkAvailable.value = false
                _isValidatedConnection.value = false
                onNetworkStateChanged?.invoke(false, false)
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error updating network state", e)
            _isNetworkAvailable.value = false
            _isValidatedConnection.value = false
            onNetworkStateChanged?.invoke(false, false)
        }
    }
    
    fun checkCurrentNetworkState() {
        try {
            val activeNetwork = connectivityManager.activeNetwork
            if (activeNetwork != null) {
                updateNetworkState(activeNetwork)
            } else {
                Log.d(TAG, "No active network")
                _isNetworkAvailable.value = false
                _isValidatedConnection.value = false
                onNetworkStateChanged?.invoke(false, false)
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error checking current network state", e)
            _isNetworkAvailable.value = false
            _isValidatedConnection.value = false
            onNetworkStateChanged?.invoke(false, false)
        }
    }
    
    fun isNetworkCurrentlyAvailable(): Boolean {
        return try {
            val activeNetwork = connectivityManager.activeNetwork
            val networkCapabilities = connectivityManager.getNetworkCapabilities(activeNetwork)
            
            networkCapabilities?.let {
                it.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) &&
                it.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) &&
                (it.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) ||
                 it.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) ||
                 it.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET))
            } ?: false
        } catch (e: Exception) {
            Log.e(TAG, "Error checking network availability", e)
            false
        }
    }
    
    /**
     * Set simple network availability callback
     */
    fun setNetworkCallback(callback: (Boolean) -> Unit) {
        simpleNetworkCallback = callback
        // Start monitoring if not already started
        if (networkCallback == null) {
            startMonitoring { isAvailable, _ ->
                callback(isAvailable)
            }
        }
    }
    
    /**
     * Cleanup network monitoring
     */
    fun cleanup() {
        stopMonitoring()
        simpleNetworkCallback = null
        onNetworkStateChanged = null
    }
}
