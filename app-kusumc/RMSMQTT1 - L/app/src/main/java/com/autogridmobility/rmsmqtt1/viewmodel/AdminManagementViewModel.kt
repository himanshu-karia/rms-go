package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApi
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApiClient
import com.autogridmobility.rmsmqtt1.data.mobile.AdminProject
import com.autogridmobility.rmsmqtt1.data.mobile.AdminUserSummary
import com.autogridmobility.rmsmqtt1.data.mobile.ApiKeyRecord
import com.autogridmobility.rmsmqtt1.data.mobile.CreateAdminUserRequest
import com.autogridmobility.rmsmqtt1.data.mobile.CreateApiKeyRequest
import com.autogridmobility.rmsmqtt1.data.mobile.ResetAdminPasswordRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpdateAdminUserRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpsertProjectRequest
import com.autogridmobility.rmsmqtt1.utils.MobileSessionManager
import com.autogridmobility.rmsmqtt1.utils.MobileSessionStore
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject

class AdminManagementViewModel(
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

    private val parser = Json { ignoreUnknownKeys = true }
    private val emailRegex = Regex("^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$")

    private val _projects = MutableStateFlow<List<AdminProject>>(emptyList())
    val projects: StateFlow<List<AdminProject>> = _projects

    private val _users = MutableStateFlow<List<AdminUserSummary>>(emptyList())
    val users: StateFlow<List<AdminUserSummary>> = _users

    private val _apiKeys = MutableStateFlow<List<ApiKeyRecord>>(emptyList())
    val apiKeys: StateFlow<List<ApiKeyRecord>> = _apiKeys

    private val _createdApiSecret = MutableStateFlow<String?>(null)
    val createdApiSecret: StateFlow<String?> = _createdApiSecret

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error

    private val _info = MutableStateFlow<String?>(null)
    val info: StateFlow<String?> = _info

    fun clearMessages() {
        _error.value = null
        _info.value = null
    }

    fun clearApiSecret() {
        _createdApiSecret.value = null
    }

    fun loadProjects() {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listProjects(token)
                .onSuccess { _projects.value = it }
                .onFailure { _error.value = it.message ?: "Failed to load projects" }
            _isLoading.value = false
        }
    }

    fun createOrUpdateProject(
        id: String,
        name: String,
        type: String,
        location: String,
        ownerOrgId: String,
        configText: String,
        isUpdate: Boolean
    ) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (id.trim().isBlank() || name.trim().isBlank()) {
            _error.value = "Project ID and Name are required"
            return
        }
        val config = parseConfig(configText)
        if (config == null) {
            _error.value = "Config must be valid JSON object"
            return
        }

        val req = UpsertProjectRequest(
            id = id.trim(),
            name = name.trim(),
            type = type.trim().ifBlank { "rms" },
            location = location.trim(),
            ownerOrgId = ownerOrgId.trim().ifBlank { null },
            config = config
        )

        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            val result = if (isUpdate) api.updateProject(token, req.id, req) else api.createProject(token, req)
            result
                .onSuccess {
                    _info.value = if (isUpdate) "Project updated" else "Project created"
                    loadProjects()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to save project"
                    _isLoading.value = false
                }
        }
    }

    fun deleteProject(projectId: String) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.deleteProject(token, projectId)
                .onSuccess {
                    _info.value = "Project deleted"
                    loadProjects()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to delete project"
                    _isLoading.value = false
                }
        }
    }

    fun loadUsers() {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listUsers(token)
                .onSuccess { _users.value = it.users }
                .onFailure { _error.value = it.message ?: "Failed to load users" }
            _isLoading.value = false
        }
    }

    fun createUser(
        username: String,
        displayName: String,
        email: String,
        phone: String,
        password: String,
        status: String,
        mustRotate: Boolean
    ) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (username.trim().isBlank() || password.isBlank()) {
            _error.value = "Username and password are required"
            return
        }
        val normalizedEmail = email.trim()
        val normalizedPhone = phone.trim()
        if (normalizedEmail.isNotBlank() && !emailRegex.matches(normalizedEmail)) {
            _error.value = "Email is invalid"
            return
        }
        if (normalizedPhone.isNotBlank() && !Regex("^\\d{10}$").matches(normalizedPhone)) {
            _error.value = "Phone must be exactly 10 digits"
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.createUser(
                token,
                CreateAdminUserRequest(
                    username = username.trim(),
                    displayName = displayName.trim().ifBlank { null },
                    email = normalizedEmail.ifBlank { null },
                    phone = normalizedPhone.ifBlank { null },
                    password = password,
                    status = status,
                    mustRotatePassword = mustRotate
                )
            )
                .onSuccess {
                    _info.value = "User created"
                    loadUsers()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to create user"
                    _isLoading.value = false
                }
        }
    }

    fun updateUser(userId: String, displayName: String, status: String, mustRotate: Boolean) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.updateUser(token, userId, UpdateAdminUserRequest(displayName = displayName, status = status, mustRotatePassword = mustRotate))
                .onSuccess {
                    _info.value = "User updated"
                    loadUsers()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to update user"
                    _isLoading.value = false
                }
        }
    }

    fun resetUserPassword(userId: String, password: String, requireChange: Boolean) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (password.isBlank()) {
            _error.value = "Password is required"
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.resetUserPassword(token, userId, ResetAdminPasswordRequest(password = password, requirePasswordChange = requireChange))
                .onSuccess {
                    _info.value = "Password reset"
                    loadUsers()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to reset password"
                    _isLoading.value = false
                }
        }
    }

    fun deleteUser(userId: String) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.deleteUser(token, userId)
                .onSuccess {
                    _info.value = "User deleted"
                    loadUsers()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to delete user"
                    _isLoading.value = false
                }
        }
    }

    fun loadApiKeys() {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listApiKeys(token)
                .onSuccess { _apiKeys.value = it }
                .onFailure { _error.value = it.message ?: "Failed to load API keys" }
            _isLoading.value = false
        }
    }

    fun createApiKey(name: String, scopesCsv: String) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        if (name.trim().isBlank()) {
            _error.value = "Key name is required"
            return
        }
        val scopes = scopesCsv.split(',').map { it.trim() }.filter { it.isNotBlank() }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.createApiKey(token, CreateApiKeyRequest(name = name.trim(), scopes = scopes))
                .onSuccess {
                    _createdApiSecret.value = it.secret
                    _info.value = "API key created"
                    loadApiKeys()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to create API key"
                    _isLoading.value = false
                }
        }
    }

    fun revokeApiKey(id: String) {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.revokeApiKey(token, id)
                .onSuccess {
                    _info.value = "API key revoked"
                    loadApiKeys()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to revoke API key"
                    _isLoading.value = false
                }
        }
    }

    private fun parseConfig(raw: String): JsonObject? {
        val text = raw.trim().ifBlank { "{}" }
        return try {
            parser.parseToJsonElement(text).jsonObject
        } catch (_: Exception) {
            null
        }
    }
}
