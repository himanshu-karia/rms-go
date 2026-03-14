package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import android.os.Build
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.data.mobile.MobileApiClient
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAuthApi
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAssignmentItem
import com.autogridmobility.rmsmqtt1.data.mobile.MobileRequestOtpRequest
import com.autogridmobility.rmsmqtt1.data.mobile.MobileVerifyOtpRequest
import com.autogridmobility.rmsmqtt1.sync.MobileSyncOrchestrator
import com.autogridmobility.rmsmqtt1.sync.WorkManagerMobileSyncOrchestrator
import com.autogridmobility.rmsmqtt1.utils.MobileSessionManager
import com.autogridmobility.rmsmqtt1.utils.MobileSessionStore
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

class MobileAuthViewModel(
    application: Application,
    private val apiClient: MobileAuthApi = MobileApiClient(),
    private val sessionManager: MobileSessionStore = MobileSessionManager(application),
    private val syncOrchestrator: MobileSyncOrchestrator = WorkManagerMobileSyncOrchestrator(application),
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO
) : AndroidViewModel(application) {

    companion object {
        private const val DEV_BYPASS_USERNAME = "Him"
        private const val DEV_BYPASS_PASSWORD = "0554"
        private const val DEV_BYPASS_PHONE = "9999999999"
    }

    constructor(application: Application) : this(
        application = application,
        apiClient = MobileApiClient(),
        sessionManager = MobileSessionManager(application),
        syncOrchestrator = WorkManagerMobileSyncOrchestrator(application),
        ioDispatcher = Dispatchers.IO
    )

    private val _phone = MutableStateFlow(sessionManager.getPhone())
    val phone: StateFlow<String> = _phone

    private val _otpRef = MutableStateFlow<String?>(null)
    val otpRef: StateFlow<String?> = _otpRef

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error

    private val _isAuthenticated = MutableStateFlow(sessionManager.hasSession())
    val isAuthenticated: StateFlow<Boolean> = _isAuthenticated

    private val _assignments = MutableStateFlow<List<MobileAssignmentItem>>(emptyList())
    val assignments: StateFlow<List<MobileAssignmentItem>> = _assignments

    init {
        if (_isAuthenticated.value) {
            bootstrapSession()
        }
    }

    fun updatePhone(value: String) {
        _phone.value = value
    }

    fun requestOtp(onRequested: () -> Unit) {
        val normalizedPhone = _phone.value.trim()
        if (normalizedPhone.isBlank()) {
            _error.value = "Phone is required"
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            _error.value = null

            val request = MobileRequestOtpRequest(
                phone = normalizedPhone,
                deviceFingerprint = buildDeviceFingerprint(),
                deviceName = Build.MODEL ?: "Android",
                appVersion = "1.0.0"
            )

            apiClient.requestOtp(request)
                .onSuccess {
                    _otpRef.value = it.otpRef
                    onRequested()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to request OTP"
                }

            _isLoading.value = false
        }
    }

    fun verifyOtp(otp: String, onSuccess: () -> Unit) {
        val normalizedPhone = _phone.value.trim()
        val reference = _otpRef.value

        if (normalizedPhone.isBlank() || reference.isNullOrBlank() || otp.isBlank()) {
            _error.value = "Phone, otp_ref and OTP are required"
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            _error.value = null

            apiClient.verifyOtp(MobileVerifyOtpRequest(normalizedPhone, otp.trim(), reference))
                .onSuccess { response ->
                    sessionManager.saveSession(response.accessToken, response.refreshToken, normalizedPhone)
                    _isAuthenticated.value = true
                    loadAssignments()
                    syncOrchestrator.schedulePeriodic()
                    syncOrchestrator.triggerNow()
                    onSuccess()
                }
                .onFailure {
                    _error.value = it.message ?: "OTP verification failed"
                }

            _isLoading.value = false
        }
    }

    fun loginWithBypass(username: String, password: String, onSuccess: () -> Unit) {
        if (username.trim() != DEV_BYPASS_USERNAME || password.trim() != DEV_BYPASS_PASSWORD) {
            _error.value = "Invalid credentials"
            return
        }

        val bypassPhone = _phone.value.trim().ifBlank { DEV_BYPASS_PHONE }
        _phone.value = bypassPhone

        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            _error.value = null

            val otpRequest = MobileRequestOtpRequest(
                phone = bypassPhone,
                deviceFingerprint = buildDeviceFingerprint(),
                deviceName = Build.MODEL ?: "Android",
                appVersion = "1.0.0"
            )

            val requestedOtpRef = apiClient.requestOtp(otpRequest).getOrNull()?.otpRef
            val devOtpResult = apiClient.getLatestDevOtp(bypassPhone)

            if (devOtpResult.isFailure) {
                _error.value = devOtpResult.exceptionOrNull()?.message ?: "Bypass login failed"
                _isLoading.value = false
                return@launch
            }

            val devOtp = devOtpResult.getOrThrow()
            val otpRef = devOtp.otpRef ?: requestedOtpRef
            if (otpRef.isNullOrBlank()) {
                _error.value = "Bypass login failed: missing otp_ref"
                _isLoading.value = false
                return@launch
            }

            apiClient.verifyOtp(MobileVerifyOtpRequest(bypassPhone, devOtp.otp.trim(), otpRef))
                .onSuccess { response ->
                    sessionManager.saveSession(response.accessToken, response.refreshToken, bypassPhone)
                    _otpRef.value = otpRef
                    _isAuthenticated.value = true
                    loadAssignments()
                    syncOrchestrator.schedulePeriodic()
                    syncOrchestrator.triggerNow()
                    onSuccess()
                }
                .onFailure {
                    _error.value = it.message ?: "Bypass login failed"
                }

            _isLoading.value = false
        }
    }

    fun logout(onLoggedOut: () -> Unit = {}) {
        viewModelScope.launch(ioDispatcher) {
            val token = sessionManager.getAccessToken()
            if (token.isNotBlank()) {
                apiClient.logout(token)
            }
            sessionManager.clearSession()
            _otpRef.value = null
            _assignments.value = emptyList()
            _isAuthenticated.value = false
            onLoggedOut()
        }
    }

    fun clearError() {
        _error.value = null
    }

    private fun bootstrapSession() {
        viewModelScope.launch(ioDispatcher) {
            val refreshToken = sessionManager.getRefreshToken()
            if (refreshToken.isBlank()) {
                _isAuthenticated.value = false
                return@launch
            }

            apiClient.refreshToken(refreshToken)
                .onSuccess { refreshed ->
                    sessionManager.saveSession(
                        accessToken = refreshed.accessToken,
                        refreshToken = refreshed.refreshToken,
                        phone = sessionManager.getPhone()
                    )
                    _isAuthenticated.value = true
                    loadAssignments()
                    syncOrchestrator.schedulePeriodic()
                }
                .onFailure {
                    sessionManager.clearSession()
                    _isAuthenticated.value = false
                }
        }
    }

    private fun loadAssignments() {
        viewModelScope.launch(ioDispatcher) {
            val token = sessionManager.getAccessToken()
            if (token.isBlank()) {
                _assignments.value = emptyList()
                return@launch
            }
            apiClient.getAssignments(token)
                .onSuccess { _assignments.value = it.items }
                .onFailure {
                    val message = it.message ?: "Failed to load assignments"
                    if (isAuthFailure(message)) {
                        sessionManager.clearSession()
                        _otpRef.value = null
                        _assignments.value = emptyList()
                        _isAuthenticated.value = false
                        _error.value = "Session expired. Please login again."
                    } else {
                        _error.value = message
                    }
                }
        }
    }

    private fun isAuthFailure(message: String): Boolean {
        val normalized = message.lowercase()
        return normalized.startsWith("auth_401") ||
            normalized.contains("unauthorized") ||
            normalized.contains("invalid or expired token") ||
            normalized.contains("session revoked") ||
            normalized.contains("session inactive") ||
            normalized.contains("user disabled")
    }

    private fun buildDeviceFingerprint(): String {
        val brand = Build.BRAND ?: "brand"
        val model = Build.MODEL ?: "model"
        val device = Build.DEVICE ?: "device"
        val sdk = Build.VERSION.SDK_INT
        return "$brand-$model-$device-sdk$sdk"
    }
}