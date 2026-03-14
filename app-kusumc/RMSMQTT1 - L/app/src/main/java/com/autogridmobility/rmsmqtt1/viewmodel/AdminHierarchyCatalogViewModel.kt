package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApi
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApiClient
import com.autogridmobility.rmsmqtt1.data.mobile.AdminAuthorityItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminProtocolVersionItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminStateItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminVendorItem
import com.autogridmobility.rmsmqtt1.data.mobile.CreateAdminAuthorityRequest
import com.autogridmobility.rmsmqtt1.data.mobile.CreateAdminStateRequest
import com.autogridmobility.rmsmqtt1.data.mobile.CreateProtocolVersionRequest
import com.autogridmobility.rmsmqtt1.data.mobile.CreateVendorRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpdateAdminAuthorityRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpdateAdminStateRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpdateProtocolVersionRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpdateVendorRequest
import com.autogridmobility.rmsmqtt1.utils.MobileSessionManager
import com.autogridmobility.rmsmqtt1.utils.MobileSessionStore
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

class AdminHierarchyCatalogViewModel(
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

    private val _states = MutableStateFlow<List<AdminStateItem>>(emptyList())
    val states: StateFlow<List<AdminStateItem>> = _states

    private val _authorities = MutableStateFlow<List<AdminAuthorityItem>>(emptyList())
    val authorities: StateFlow<List<AdminAuthorityItem>> = _authorities

    private val _serverVendors = MutableStateFlow<List<AdminVendorItem>>(emptyList())
    val serverVendors: StateFlow<List<AdminVendorItem>> = _serverVendors

    private val _protocolVersions = MutableStateFlow<List<AdminProtocolVersionItem>>(emptyList())
    val protocolVersions: StateFlow<List<AdminProtocolVersionItem>> = _protocolVersions

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error

    private val _info = MutableStateFlow<String?>(null)
    val info: StateFlow<String?> = _info

    private fun tokenOrError(): String? {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return null
        }
        return token
    }

    fun clearMessages() {
        _error.value = null
        _info.value = null
    }

    fun loadHierarchy(stateId: String? = null) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listStates(token).onSuccess { _states.value = it }.onFailure { _error.value = it.message ?: "Failed to load states" }
            api.listAuthorities(token, stateId).onSuccess { _authorities.value = it }.onFailure { _error.value = it.message ?: "Failed to load authorities" }
            _isLoading.value = false
        }
    }

    fun createState(name: String, isoCode: String) {
        val token = tokenOrError() ?: return
        if (name.isBlank()) {
            _error.value = "State name is required"
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.createState(token, CreateAdminStateRequest(name, isoCode.ifBlank { null }))
                .onSuccess { _info.value = "State created"; loadHierarchy() }
                .onFailure { _error.value = it.message ?: "Failed to create state"; _isLoading.value = false }
        }
    }

    fun updateState(id: String, name: String, isoCode: String) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.updateState(token, id, UpdateAdminStateRequest(name.ifBlank { null }, isoCode.ifBlank { null }))
                .onSuccess { _info.value = "State updated"; loadHierarchy() }
                .onFailure { _error.value = it.message ?: "Failed to update state"; _isLoading.value = false }
        }
    }

    fun deleteState(id: String) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.deleteState(token, id)
                .onSuccess { _info.value = "State deleted"; loadHierarchy() }
                .onFailure { _error.value = it.message ?: "Failed to delete state"; _isLoading.value = false }
        }
    }

    fun createAuthority(stateId: String, name: String, type: String) {
        val token = tokenOrError() ?: return
        if (stateId.isBlank() || name.isBlank()) {
            _error.value = "State ID and authority name are required"
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.createAuthority(token, CreateAdminAuthorityRequest(stateId, name, type.ifBlank { "nodal" }))
                .onSuccess { _info.value = "Authority created"; loadHierarchy(stateId) }
                .onFailure { _error.value = it.message ?: "Failed to create authority"; _isLoading.value = false }
        }
    }

    fun updateAuthority(id: String, name: String, type: String, stateId: String?) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.updateAuthority(token, id, UpdateAdminAuthorityRequest(name.ifBlank { null }, type.ifBlank { null }))
                .onSuccess { _info.value = "Authority updated"; loadHierarchy(stateId) }
                .onFailure { _error.value = it.message ?: "Failed to update authority"; _isLoading.value = false }
        }
    }

    fun deleteAuthority(id: String, stateId: String?) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.deleteAuthority(token, id)
                .onSuccess { _info.value = "Authority deleted"; loadHierarchy(stateId) }
                .onFailure { _error.value = it.message ?: "Failed to delete authority"; _isLoading.value = false }
        }
    }

    fun loadCatalogs(projectId: String, vendorEndpoint: String = "server-vendors") {
        val token = tokenOrError() ?: return
        if (projectId.isBlank()) {
            _error.value = "Project ID is required"
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listVendors(token, vendorEndpoint)
                .onSuccess { _serverVendors.value = it.vendors }
                .onFailure { _error.value = it.message ?: "Failed to load vendors" }
            api.listProtocolVersions(token, projectId)
                .onSuccess { _protocolVersions.value = it.protocolVersions }
                .onFailure { _error.value = it.message ?: "Failed to load protocol versions" }
            _isLoading.value = false
        }
    }

    fun createVendor(name: String, endpoint: String, projectId: String) {
        val token = tokenOrError() ?: return
        if (name.isBlank()) {
            _error.value = "Vendor name is required"
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.createVendor(token, endpoint, CreateVendorRequest(name))
                .onSuccess { _info.value = "Vendor created"; loadCatalogs(projectId, endpoint) }
                .onFailure { _error.value = it.message ?: "Failed to create vendor"; _isLoading.value = false }
        }
    }

    fun updateVendor(id: String, name: String, endpoint: String, projectId: String) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.updateVendor(token, endpoint, id, UpdateVendorRequest(name))
                .onSuccess { _info.value = "Vendor updated"; loadCatalogs(projectId, endpoint) }
                .onFailure { _error.value = it.message ?: "Failed to update vendor"; _isLoading.value = false }
        }
    }

    fun deleteVendor(id: String, endpoint: String, projectId: String) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.deleteVendor(token, endpoint, id)
                .onSuccess { _info.value = "Vendor deleted"; loadCatalogs(projectId, endpoint) }
                .onFailure { _error.value = it.message ?: "Failed to delete vendor"; _isLoading.value = false }
        }
    }

    fun createProtocolVersion(stateId: String, authorityId: String, projectId: String, serverVendorId: String, version: String, name: String) {
        val token = tokenOrError() ?: return
        if (stateId.isBlank() || authorityId.isBlank() || projectId.isBlank() || serverVendorId.isBlank() || version.isBlank()) {
            _error.value = "State, authority, project, server vendor, and version are required"
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.createProtocolVersion(token, CreateProtocolVersionRequest(stateId, authorityId, projectId, serverVendorId, version, name.ifBlank { null }))
                .onSuccess { _info.value = "Protocol version created"; loadCatalogs(projectId) }
                .onFailure { _error.value = it.message ?: "Failed to create protocol version"; _isLoading.value = false }
        }
    }

    fun updateProtocolVersion(id: String, version: String, name: String, serverVendorId: String, projectId: String) {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.updateProtocolVersion(token, id, UpdateProtocolVersionRequest(version.ifBlank { null }, name.ifBlank { null }, serverVendorId.ifBlank { null }))
                .onSuccess { _info.value = "Protocol version updated"; loadCatalogs(projectId) }
                .onFailure { _error.value = it.message ?: "Failed to update protocol version"; _isLoading.value = false }
        }
    }
}
