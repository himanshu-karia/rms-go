package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApi
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApiClient
import com.autogridmobility.rmsmqtt1.data.mobile.AdminAuthorityItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminProject
import com.autogridmobility.rmsmqtt1.data.mobile.AdminProtocolVersionItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminStateItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminVendorItem
import com.autogridmobility.rmsmqtt1.data.mobile.CommandCatalogItem
import com.autogridmobility.rmsmqtt1.data.mobile.CreateSimulatorSessionRequest
import com.autogridmobility.rmsmqtt1.data.mobile.SimulatorSession
import com.autogridmobility.rmsmqtt1.data.mobile.UpsertCommandCatalogRequest
import com.autogridmobility.rmsmqtt1.utils.MobileSessionManager
import com.autogridmobility.rmsmqtt1.utils.MobileSessionStore
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement

class AdminOpsViewModel(
    application: Application,
    private val api: AdminApi = AdminApiClient(),
    private val sessionStore: MobileSessionStore = MobileSessionManager(application),
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO
) : AndroidViewModel(application) {
    constructor(application: Application) : this(
        application = application,
        api = AdminApiClient(),
        sessionStore = MobileSessionManager(application),
        ioDispatcher = Dispatchers.IO
    )

    private val serializer = Json { ignoreUnknownKeys = true }

    private val _projectId = MutableStateFlow("")
    val projectId: StateFlow<String> = _projectId

    private val _deviceId = MutableStateFlow("")
    val deviceId: StateFlow<String> = _deviceId

    private val _commandItems = MutableStateFlow<List<CommandCatalogItem>>(emptyList())
    val commandItems: StateFlow<List<CommandCatalogItem>> = _commandItems

    private val _sessionDeviceUuid = MutableStateFlow("")
    val sessionDeviceUuid: StateFlow<String> = _sessionDeviceUuid

    private val _sessionDurationMins = MutableStateFlow("60")
    val sessionDurationMins: StateFlow<String> = _sessionDurationMins

    private val _simulatorSessions = MutableStateFlow<List<SimulatorSession>>(emptyList())
    val simulatorSessions: StateFlow<List<SimulatorSession>> = _simulatorSessions

    private val _simImei = MutableStateFlow("")
    val simImei: StateFlow<String> = _simImei

    private val _simTopic = MutableStateFlow("heartbeat")
    val simTopic: StateFlow<String> = _simTopic

    private val _simClientId = MutableStateFlow("")
    val simClientId: StateFlow<String> = _simClientId

    private val _simUsername = MutableStateFlow("")
    val simUsername: StateFlow<String> = _simUsername

    private val _simPassword = MutableStateFlow("")
    val simPassword: StateFlow<String> = _simPassword

    private val _simPayload = MutableStateFlow("{\n  \"packet_type\": \"heartbeat\",\n  \"ts\": 0\n}")
    val simPayload: StateFlow<String> = _simPayload

    private val _simHttpResult = MutableStateFlow<String?>(null)
    val simHttpResult: StateFlow<String?> = _simHttpResult

    private val _bootstrapProjects = MutableStateFlow<List<AdminProject>>(emptyList())
    val bootstrapProjects: StateFlow<List<AdminProject>> = _bootstrapProjects

    private val _bootstrapStates = MutableStateFlow<List<AdminStateItem>>(emptyList())
    val bootstrapStates: StateFlow<List<AdminStateItem>> = _bootstrapStates

    private val _bootstrapAuthorities = MutableStateFlow<List<AdminAuthorityItem>>(emptyList())
    val bootstrapAuthorities: StateFlow<List<AdminAuthorityItem>> = _bootstrapAuthorities

    private val _bootstrapVendors = MutableStateFlow<List<AdminVendorItem>>(emptyList())
    val bootstrapVendors: StateFlow<List<AdminVendorItem>> = _bootstrapVendors

    private val _bootstrapProtocols = MutableStateFlow<List<AdminProtocolVersionItem>>(emptyList())
    val bootstrapProtocols: StateFlow<List<AdminProtocolVersionItem>> = _bootstrapProtocols

    private val _isBootstrapping = MutableStateFlow(false)
    val isBootstrapping: StateFlow<Boolean> = _isBootstrapping

    private val _isLoadingCatalog = MutableStateFlow(false)
    val isLoadingCatalog: StateFlow<Boolean> = _isLoadingCatalog

    private val _isLoadingSessions = MutableStateFlow(false)
    val isLoadingSessions: StateFlow<Boolean> = _isLoadingSessions

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error

    private val _info = MutableStateFlow<String?>(null)
    val info: StateFlow<String?> = _info

    fun updateProjectId(value: String) {
        _projectId.value = value
    }

    fun updateDeviceId(value: String) {
        _deviceId.value = value
    }

    fun updateSessionDeviceUuid(value: String) {
        _sessionDeviceUuid.value = value
    }

    fun updateSessionDurationMins(value: String) {
        if (value.isEmpty() || value.all { it.isDigit() }) {
            _sessionDurationMins.value = value
        }
    }

    fun updateSimImei(value: String) {
        _simImei.value = value
    }

    fun updateSimTopic(value: String) {
        _simTopic.value = value
    }

    fun updateSimClientId(value: String) {
        _simClientId.value = value
    }

    fun updateSimUsername(value: String) {
        _simUsername.value = value
    }

    fun updateSimPassword(value: String) {
        _simPassword.value = value
    }

    fun updateSimPayload(value: String) {
        _simPayload.value = value
    }

    fun clearError() {
        _error.value = null
    }

    fun clearInfo() {
        _info.value = null
    }

    fun fetchDeviceOpenCredentials() {
        val imei = _simImei.value.trim()
        if (imei.isBlank()) {
            _error.value = "IMEI is required"
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingSessions.value = true
            _error.value = null
            _info.value = null

            api.loadDeviceOpenCredentials(imei)
                .onSuccess { response ->
                    val credential = response.credential
                    _simUsername.value = credential?.username ?: _simUsername.value
                    _simPassword.value = credential?.password ?: _simPassword.value
                    _simClientId.value = credential?.clientId ?: _simClientId.value
                    val firstTopic = credential?.publishTopics?.firstOrNull()?.substringAfterLast('/')
                    if (!firstTopic.isNullOrBlank()) {
                        _simTopic.value = firstTopic
                    }
                    _info.value = "Device-open credentials loaded"
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to fetch device-open credentials"
                }

            _isLoadingSessions.value = false
        }
    }

    fun sendHttpsIngest() {
        val topic = _simTopic.value.trim()
        val imei = _simImei.value.trim()
        val clientId = _simClientId.value.trim()
        val username = _simUsername.value.trim()
        val password = _simPassword.value
        val payload = _simPayload.value.trim()

        if (topic.isBlank() || imei.isBlank() || clientId.isBlank() || username.isBlank() || password.isBlank() || payload.isBlank()) {
            _error.value = "Topic, IMEI, clientId, username, password and payload are required"
            return
        }

        if (parsePayload(payload) == null) {
            _error.value = "Payload must be valid JSON"
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingSessions.value = true
            _error.value = null
            _info.value = null

            api.sendHttpsIngest(topic, imei, clientId, username, password, payload)
                .onSuccess { result ->
                    _simHttpResult.value = "${result.statusCode} ${result.body}".trim()
                    _info.value = "HTTPS ingest request sent"
                }
                .onFailure {
                    _error.value = it.message ?: "HTTPS ingest failed"
                }

            _isLoadingSessions.value = false
        }
    }

    fun loadCommandCatalog() {
        val token = sessionStore.getAccessToken()
        val project = _projectId.value.trim()
        val device = _deviceId.value.trim()

        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (project.isBlank() || device.isBlank()) {
            _error.value = "Project ID and Device ID/IMEI are required"
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingCatalog.value = true
            _error.value = null
            _info.value = null
            api.listCommandCatalog(token, project, device)
                .onSuccess { _commandItems.value = it }
                .onFailure { _error.value = it.message ?: "Failed to load command catalog" }
            _isLoadingCatalog.value = false
        }
    }

    fun upsertCommandCatalog(
        id: String?,
        name: String,
        scope: String,
        transport: String,
        protocolId: String,
        modelId: String,
        payloadSchemaText: String,
        deviceIdsRaw: String
    ) {
        val token = sessionStore.getAccessToken()
        val project = _projectId.value.trim()
        val device = _deviceId.value.trim()

        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (project.isBlank() || device.isBlank()) {
            _error.value = "Project ID and Device ID/IMEI are required"
            return
        }
        if (name.trim().isBlank()) {
            _error.value = "Command name is required"
            return
        }

        val payloadSchema = parsePayload(payloadSchemaText)
        if (payloadSchemaText.trim().isNotBlank() && payloadSchema == null) {
            _error.value = "Payload schema must be valid JSON"
            return
        }

        val deviceIds = deviceIdsRaw
            .split(',')
            .map { it.trim() }
            .filter { it.isNotBlank() }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingCatalog.value = true
            _error.value = null
            _info.value = null

            val request = UpsertCommandCatalogRequest(
                id = id?.takeIf { it.isNotBlank() },
                name = name.trim(),
                scope = scope,
                transport = transport.ifBlank { "mqtt" },
                protocolId = protocolId.trim().ifBlank { null },
                modelId = modelId.trim().ifBlank { null },
                projectId = project,
                payloadSchema = payloadSchema,
                deviceIds = deviceIds
            )

            api.upsertCommandCatalog(token, request)
                .onSuccess {
                    _info.value = if (id.isNullOrBlank()) "Command created" else "Command updated"
                    loadCommandCatalog()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to save command"
                    _isLoadingCatalog.value = false
                }
        }
    }

    fun deleteCommandCatalog(commandId: String) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (commandId.isBlank()) {
            _error.value = "Command ID is required"
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingCatalog.value = true
            _error.value = null
            _info.value = null
            api.deleteCommandCatalog(token, commandId)
                .onSuccess {
                    _info.value = "Command deleted"
                    loadCommandCatalog()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to delete command"
                    _isLoadingCatalog.value = false
                }
        }
    }

    fun loadSimulatorSessions() {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingSessions.value = true
            _error.value = null
            _info.value = null
            api.listSimulatorSessions(token)
                .onSuccess { _simulatorSessions.value = it.sessions }
                .onFailure { _error.value = it.message ?: "Failed to load simulator sessions" }
            _isLoadingSessions.value = false
        }
    }

    fun bootstrapSimulatorContext() {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isBootstrapping.value = true
            _error.value = null
            _info.value = null

            val projects = api.listProjects(token).getOrElse {
                _error.value = it.message ?: "Failed to load projects"
                _isBootstrapping.value = false
                return@launch
            }
            _bootstrapProjects.value = projects
            val selectedProjectId = projects.firstOrNull()?.id.orEmpty()
            if (selectedProjectId.isNotBlank()) {
                _projectId.value = selectedProjectId
            }

            val states = api.listStates(token).getOrElse {
                _error.value = it.message ?: "Failed to load states"
                _isBootstrapping.value = false
                return@launch
            }
            _bootstrapStates.value = states

            val firstState = states.firstOrNull()?.id
            val authorities = api.listAuthorities(token, firstState).getOrElse {
                _error.value = it.message ?: "Failed to load authorities"
                _isBootstrapping.value = false
                return@launch
            }
            _bootstrapAuthorities.value = authorities

            val vendors = api.listVendors(token, "server-vendors").getOrElse {
                _error.value = it.message ?: "Failed to load vendors"
                _isBootstrapping.value = false
                return@launch
            }
            _bootstrapVendors.value = vendors.vendors

            if (selectedProjectId.isNotBlank()) {
                val protocols = api.listProtocolVersions(token, selectedProjectId).getOrElse {
                    _error.value = it.message ?: "Failed to load protocol versions"
                    _isBootstrapping.value = false
                    return@launch
                }
                _bootstrapProtocols.value = protocols.protocolVersions
            } else {
                _bootstrapProtocols.value = emptyList()
            }

            _info.value = "Simulator context bootstrapped"
            _isBootstrapping.value = false
        }
    }

    fun createSimulatorSession() {
        val token = sessionStore.getAccessToken()
        val deviceUuid = _sessionDeviceUuid.value.trim()
        val duration = _sessionDurationMins.value.toIntOrNull()

        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (deviceUuid.isBlank()) {
            _error.value = "Device UUID is required"
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingSessions.value = true
            _error.value = null
            _info.value = null
            api.createSimulatorSession(token, CreateSimulatorSessionRequest(deviceUuid, duration))
                .onSuccess {
                    _info.value = "Simulator session created"
                    loadSimulatorSessions()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to create simulator session"
                    _isLoadingSessions.value = false
                }
        }
    }

    fun revokeSimulatorSession(sessionId: String) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }

        viewModelScope.launch(ioDispatcher) {
            _isLoadingSessions.value = true
            _error.value = null
            _info.value = null
            api.revokeSimulatorSession(token, sessionId)
                .onSuccess {
                    _info.value = "Simulator session revoked"
                    loadSimulatorSessions()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to revoke simulator session"
                    _isLoadingSessions.value = false
                }
        }
    }

    private fun parsePayload(text: String): JsonElement? {
        val raw = text.trim()
        if (raw.isBlank()) return null
        return try {
            serializer.parseToJsonElement(raw)
        } catch (_: Exception) {
            null
        }
    }
}
