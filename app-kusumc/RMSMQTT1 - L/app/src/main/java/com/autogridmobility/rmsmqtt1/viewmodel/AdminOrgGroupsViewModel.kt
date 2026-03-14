package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import android.util.Log
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.autogridmobility.rmsmqtt1.data.mobile.AddUserGroupMemberRequest
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApi
import com.autogridmobility.rmsmqtt1.data.mobile.AdminApiClient
import com.autogridmobility.rmsmqtt1.data.mobile.AdminOrg
import com.autogridmobility.rmsmqtt1.data.mobile.AdminUserGroup
import com.autogridmobility.rmsmqtt1.data.mobile.UpsertOrgRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpsertUserGroupRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UpdateUserGroupRequest
import com.autogridmobility.rmsmqtt1.data.mobile.UserGroupMember
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

class AdminOrgGroupsViewModel(
    application: Application,
    private val api: AdminApi = AdminApiClient(),
    private val sessionStore: MobileSessionStore = MobileSessionManager(application),
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO
) : AndroidViewModel(application) {
    private val tag = "AdminOrgGroupsVM"

    constructor(application: Application) : this(
        application = application,
        api = AdminApiClient(),
        sessionStore = MobileSessionManager(application),
        ioDispatcher = Dispatchers.IO
    )

    private val parser = Json { ignoreUnknownKeys = true }

    private val _orgs = MutableStateFlow<List<AdminOrg>>(emptyList())
    val orgs: StateFlow<List<AdminOrg>> = _orgs

    private val _groups = MutableStateFlow<List<AdminUserGroup>>(emptyList())
    val groups: StateFlow<List<AdminUserGroup>> = _groups

    private val _members = MutableStateFlow<List<UserGroupMember>>(emptyList())
    val members: StateFlow<List<UserGroupMember>> = _members

    private val _selectedGroupId = MutableStateFlow<String?>(null)
    val selectedGroupId: StateFlow<String?> = _selectedGroupId

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

    private fun tokenOrError(): String? {
        val token = sessionStore.getAccessToken()
        if (token.isBlank()) {
            _error.value = "Session missing. Login again."
            return null
        }
        return token
    }

    fun loadOrgs() {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listOrgs(token)
                .onSuccess {
                    _orgs.value = it
                    Log.i(tag, "listOrgs success: count=${it.size}")
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to load orgs"
                    Log.e(tag, "listOrgs failed", it)
                }
            _isLoading.value = false
        }
    }

    fun createOrUpdateOrg(
        id: String,
        name: String,
        type: String,
        path: String,
        parentId: String,
        metadataText: String,
        isUpdate: Boolean
    ) {
        val token = tokenOrError() ?: return
        if (name.trim().isBlank() || type.trim().isBlank()) {
            _error.value = "Org name and type are required"
            Log.w(tag, "createOrUpdateOrg validation failed: name/type required")
            return
        }
        if (isUpdate && id.trim().isBlank()) {
            _error.value = "Org ID is required for update"
            Log.w(tag, "createOrUpdateOrg validation failed: org id required for update")
            return
        }
        val metadata = parseJsonObject(metadataText)
        if (metadata == null) {
            _error.value = "Metadata must be a valid JSON object"
            Log.w(tag, "createOrUpdateOrg validation failed: invalid metadata json")
            return
        }

        val req = UpsertOrgRequest(
            name = name.trim(),
            type = type.trim(),
            path = path.trim(),
            parentId = parentId.trim().ifBlank { null },
            metadata = metadata
        )

        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            val result = if (isUpdate) {
                api.updateOrg(token, id.trim(), req)
            } else {
                api.createOrg(token, req)
            }
            result.onSuccess {
                _info.value = if (isUpdate) "Org updated" else "Org created"
                Log.i(tag, "${if (isUpdate) "updateOrg" else "createOrg"} success: id=${it.id}, name=${it.name}")
                loadOrgs()
            }.onFailure {
                _error.value = it.message ?: "Failed to save org"
                Log.e(tag, "${if (isUpdate) "updateOrg" else "createOrg"} failed", it)
                _isLoading.value = false
            }
        }
    }

    fun loadUserGroups() {
        val token = tokenOrError() ?: return
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listUserGroups(token)
                .onSuccess {
                    _groups.value = it.groups
                    Log.i(tag, "listUserGroups success: count=${it.groups.size}")
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to load user groups"
                    Log.e(tag, "listUserGroups failed", it)
                }
            _isLoading.value = false
        }
    }

    fun createUserGroup(
        name: String,
        description: String,
        stateId: String,
        authorityId: String,
        projectId: String,
        defaultRoleIdsCsv: String
    ) {
        val token = tokenOrError() ?: return
        if (name.trim().isBlank()) {
            _error.value = "Group name is required"
            Log.w(tag, "createUserGroup validation failed: name required")
            return
        }
        val scope = mutableMapOf<String, String>()
        stateId.trim().takeIf { it.isNotBlank() }?.let { scope["state_id"] = it }
        authorityId.trim().takeIf { it.isNotBlank() }?.let { scope["authority_id"] = it }
        projectId.trim().takeIf { it.isNotBlank() }?.let { scope["project_id"] = it }
        if (scope.isEmpty()) {
            _error.value = "At least one scope field is required"
            Log.w(tag, "createUserGroup validation failed: scope required")
            return
        }
        val roleIds = parseCsv(defaultRoleIdsCsv)
        if (roleIds.isEmpty()) {
            _error.value = "At least one default role ID is required"
            Log.w(tag, "createUserGroup validation failed: default role ids required")
            return
        }

        val req = UpsertUserGroupRequest(
            name = name.trim(),
            description = description.trim().ifBlank { null },
            scope = scope,
            defaultRoleIds = roleIds
        )

        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.createUserGroup(token, req)
                .onSuccess {
                    _info.value = "User group created"
                    Log.i(tag, "createUserGroup success")
                    loadUserGroups()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to create user group"
                    Log.e(tag, "createUserGroup failed", it)
                    _isLoading.value = false
                }
        }
    }

    fun updateUserGroup(groupId: String, name: String, description: String, defaultRoleIdsCsv: String) {
        val token = tokenOrError() ?: return
        if (groupId.trim().isBlank()) {
            _error.value = "Group ID is required"
            Log.w(tag, "updateUserGroup validation failed: group id required")
            return
        }

        val parsedRoles = parseCsv(defaultRoleIdsCsv)
        val req = UpdateUserGroupRequest(
            name = name.trim().ifBlank { null },
            description = description.trim().ifBlank { null },
            defaultRoleIds = if (parsedRoles.isEmpty()) null else parsedRoles
        )

        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.updateUserGroup(token, groupId.trim(), req)
                .onSuccess {
                    _info.value = "User group updated"
                    Log.i(tag, "updateUserGroup success: groupId=${groupId.trim()}")
                    loadUserGroups()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to update user group"
                    Log.e(tag, "updateUserGroup failed: groupId=${groupId.trim()}", it)
                    _isLoading.value = false
                }
        }
    }

    fun deleteUserGroup(groupId: String) {
        val token = tokenOrError() ?: return
        if (groupId.trim().isBlank()) {
            _error.value = "Group ID is required"
            Log.w(tag, "deleteUserGroup validation failed: group id required")
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.deleteUserGroup(token, groupId.trim())
                .onSuccess {
                    _info.value = "User group deleted"
                    Log.i(tag, "deleteUserGroup success: groupId=${groupId.trim()}")
                    if (_selectedGroupId.value == groupId.trim()) {
                        _selectedGroupId.value = null
                        _members.value = emptyList()
                    }
                    loadUserGroups()
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to delete user group"
                    Log.e(tag, "deleteUserGroup failed: groupId=${groupId.trim()}", it)
                    _isLoading.value = false
                }
        }
    }

    fun loadMembers(groupId: String) {
        val token = tokenOrError() ?: return
        if (groupId.trim().isBlank()) {
            _error.value = "Group ID is required"
            Log.w(tag, "loadMembers validation failed: group id required")
            return
        }
        val selected = groupId.trim()
        _selectedGroupId.value = selected
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.listUserGroupMembers(token, selected)
                .onSuccess {
                    _members.value = it.members
                    Log.i(tag, "listMembers success: groupId=$selected, count=${it.members.size}")
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to load members"
                    Log.e(tag, "listMembers failed: groupId=$selected", it)
                }
            _isLoading.value = false
        }
    }

    fun addMember(groupId: String, userId: String) {
        val token = tokenOrError() ?: return
        if (groupId.trim().isBlank() || userId.trim().isBlank()) {
            _error.value = "Group ID and user ID are required"
            Log.w(tag, "addMember validation failed: groupId/userId required")
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.addUserGroupMember(token, groupId.trim(), AddUserGroupMemberRequest(userId.trim()))
                .onSuccess {
                    _info.value = "Member added"
                    Log.i(tag, "addMember success: groupId=${groupId.trim()}, userId=${userId.trim()}")
                    loadMembers(groupId)
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to add member"
                    Log.e(tag, "addMember failed: groupId=${groupId.trim()}, userId=${userId.trim()}", it)
                    _isLoading.value = false
                }
        }
    }

    fun removeMember(groupId: String, userId: String) {
        val token = tokenOrError() ?: return
        if (groupId.trim().isBlank() || userId.trim().isBlank()) {
            _error.value = "Group ID and user ID are required"
            Log.w(tag, "removeMember validation failed: groupId/userId required")
            return
        }
        viewModelScope.launch(ioDispatcher) {
            _isLoading.value = true
            clearMessages()
            api.removeUserGroupMember(token, groupId.trim(), userId.trim())
                .onSuccess {
                    _info.value = "Member removed"
                    Log.i(tag, "removeMember success: groupId=${groupId.trim()}, userId=${userId.trim()}")
                    loadMembers(groupId)
                }
                .onFailure {
                    _error.value = it.message ?: "Failed to remove member"
                    Log.e(tag, "removeMember failed: groupId=${groupId.trim()}, userId=${userId.trim()}", it)
                    _isLoading.value = false
                }
        }
    }

    private fun parseCsv(raw: String): List<String> {
        return raw.split(',').map { it.trim() }.filter { it.isNotBlank() }
    }

    private fun parseJsonObject(raw: String): JsonObject? {
        val text = raw.trim().ifBlank { "{}" }
        return try {
            parser.parseToJsonElement(text).jsonObject
        } catch (_: Exception) {
            null
        }
    }
}
