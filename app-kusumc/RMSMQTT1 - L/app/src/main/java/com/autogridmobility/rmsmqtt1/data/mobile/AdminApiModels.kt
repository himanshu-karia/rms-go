package com.autogridmobility.rmsmqtt1.data.mobile

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

@Serializable
data class CommandCatalogItem(
    val id: String,
    val name: String,
    val scope: String,
    @SerialName("protocol_id") val protocolId: String? = null,
    @SerialName("model_id") val modelId: String? = null,
    @SerialName("project_id") val projectId: String? = null,
    @SerialName("payload_schema") val payloadSchema: JsonElement? = null,
    val transport: String? = null,
    @SerialName("created_at") val createdAt: String? = null
)

@Serializable
data class UpsertCommandCatalogRequest(
    val id: String? = null,
    val name: String,
    val scope: String,
    val transport: String = "mqtt",
    val protocolId: String? = null,
    val modelId: String? = null,
    val projectId: String? = null,
    val payloadSchema: JsonElement? = null,
    val deviceIds: List<String> = emptyList()
)

@Serializable
data class UpsertCommandCatalogResponse(
    val id: String
)

@Serializable
data class SimulatorSession(
    @SerialName("session_id") val sessionId: String,
    @SerialName("device_uuid") val deviceUuid: String,
    val status: String,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("expires_at") val expiresAt: String? = null,
    @SerialName("revoked_at") val revokedAt: String? = null
)

@Serializable
data class SimulatorSessionsResponse(
    val count: Int = 0,
    val sessions: List<SimulatorSession> = emptyList(),
    @SerialName("next_cursor") val nextCursor: String? = null
)

@Serializable
data class CreateSimulatorSessionRequest(
    val deviceUuid: String,
    val expiresInMinutes: Int? = null
)

@Serializable
data class DeviceOpenCredential(
    val username: String? = null,
    val password: String? = null,
    @SerialName("client_id") val clientId: String? = null,
    val endpoints: List<String> = emptyList(),
    @SerialName("publish_topics") val publishTopics: List<String> = emptyList()
)

@Serializable
data class DeviceOpenResponse(
    val credential: DeviceOpenCredential? = null
)

data class HttpsIngestResult(
    val statusCode: Int,
    val body: String
)

@Serializable
data class AdminProject(
    val id: String,
    val name: String? = null,
    val type: String? = null,
    val location: String? = null,
    @SerialName("owner_org_id") val ownerOrgId: String? = null,
    val config: JsonObject? = null
)

@Serializable
data class UpsertProjectRequest(
    val id: String,
    val name: String,
    val type: String,
    val location: String,
    @SerialName("owner_org_id") val ownerOrgId: String? = null,
    val config: JsonObject? = null
)

@Serializable
data class AdminUserRoleBinding(
    val id: String,
    @SerialName("role_key") val roleKey: String? = null,
    @SerialName("role_type") val roleType: String? = null
)

@Serializable
data class AdminUserSummary(
    val id: String,
    val username: String,
    @SerialName("display_name") val displayName: String? = null,
    val status: String? = null,
    @SerialName("must_rotate_password") val mustRotatePassword: Boolean = false,
    @SerialName("role_bindings") val roleBindings: List<AdminUserRoleBinding> = emptyList()
)

@Serializable
data class AdminUsersResponse(
    val users: List<AdminUserSummary> = emptyList()
)

@Serializable
data class CreateAdminUserRequest(
    val username: String,
    @SerialName("displayName") val displayName: String? = null,
    val email: String? = null,
    val phone: String? = null,
    val password: String,
    val status: String = "active",
    @SerialName("mustRotatePassword") val mustRotatePassword: Boolean = true
)

@Serializable
data class UpdateAdminUserRequest(
    @SerialName("displayName") val displayName: String? = null,
    val status: String? = null,
    @SerialName("mustRotatePassword") val mustRotatePassword: Boolean? = null
)

@Serializable
data class ResetAdminPasswordRequest(
    val password: String,
    @SerialName("requirePasswordChange") val requirePasswordChange: Boolean = true
)

@Serializable
data class ApiKeyRecord(
    val id: String,
    val name: String,
    val prefix: String? = null,
    val scopes: List<String> = emptyList(),
    @SerialName("is_active") val isActive: Boolean = true,
    @SerialName("last_used_at") val lastUsedAt: String? = null
)

@Serializable
data class CreateApiKeyRequest(
    val name: String,
    val scopes: List<String> = emptyList()
)

@Serializable
data class CreateApiKeyResponse(
    val secret: String
)

@Serializable
data class AdminStateItem(
    val id: String,
    val name: String,
    @SerialName("iso_code") val isoCode: String? = null
)

@Serializable
data class CreateAdminStateRequest(
    val name: String,
    @SerialName("iso_code") val isoCode: String? = null
)

@Serializable
data class UpdateAdminStateRequest(
    val name: String? = null,
    @SerialName("iso_code") val isoCode: String? = null
)

@Serializable
data class AdminAuthorityItem(
    val id: String,
    @SerialName("state_id") val stateId: String,
    val name: String,
    val type: String? = null
)

@Serializable
data class CreateAdminAuthorityRequest(
    @SerialName("state_id") val stateId: String,
    val name: String,
    val type: String = "nodal"
)

@Serializable
data class UpdateAdminAuthorityRequest(
    val name: String? = null,
    val type: String? = null
)

@Serializable
data class AdminVendorItem(
    val id: String,
    val name: String,
    val category: String? = null
)

@Serializable
data class AdminVendorsResponse(
    val vendors: List<AdminVendorItem> = emptyList()
)

@Serializable
data class CreateVendorRequest(
    val name: String
)

@Serializable
data class UpdateVendorRequest(
    val name: String
)

@Serializable
data class AdminProtocolVersionItem(
    val id: String,
    @SerialName("project_id") val projectId: String,
    @SerialName("state_id") val stateId: String,
    @SerialName("authority_id") val authorityId: String,
    @SerialName("server_vendor_id") val serverVendorId: String,
    val version: String,
    val name: String? = null
)

@Serializable
data class AdminProtocolVersionsResponse(
    @SerialName("protocol_versions") val protocolVersions: List<AdminProtocolVersionItem> = emptyList(),
    val count: Int = 0
)

@Serializable
data class CreateProtocolVersionRequest(
    @SerialName("stateId") val stateId: String,
    @SerialName("stateAuthorityId") val stateAuthorityId: String,
    @SerialName("projectId") val projectId: String,
    @SerialName("serverVendorId") val serverVendorId: String,
    val version: String,
    val name: String? = null
)

@Serializable
data class UpdateProtocolVersionRequest(
    val version: String? = null,
    val name: String? = null,
    @SerialName("serverVendorId") val serverVendorId: String? = null
)

@Serializable
data class AdminOrg(
    val id: String,
    val name: String,
    val type: String,
    val path: String? = null,
    @SerialName("parent_id") val parentId: String? = null
)

@Serializable
data class UpsertOrgRequest(
    val name: String,
    val type: String,
    val path: String = "",
    @SerialName("parent_id") val parentId: String? = null,
    val metadata: JsonObject? = null
)

@Serializable
data class UserGroupScope(
    @SerialName("state_id") val stateId: String? = null,
    @SerialName("authority_id") val authorityId: String? = null,
    @SerialName("project_id") val projectId: String? = null
)

@Serializable
data class AdminUserGroup(
    val id: String,
    val name: String,
    val description: String? = null,
    val scope: UserGroupScope? = null,
    @SerialName("default_role_ids") val defaultRoleIds: List<String> = emptyList()
)

@Serializable
data class AdminUserGroupsResponse(
    val groups: List<AdminUserGroup> = emptyList()
)

@Serializable
data class UpsertUserGroupRequest(
    val name: String,
    val description: String? = null,
    val scope: Map<String, String>,
    @SerialName("default_role_ids") val defaultRoleIds: List<String>
)

@Serializable
data class UpdateUserGroupRequest(
    val name: String? = null,
    val description: String? = null,
    @SerialName("default_role_ids") val defaultRoleIds: List<String>? = null
)

@Serializable
data class UserGroupMember(
    @SerialName("group_id") val groupId: String,
    @SerialName("user_id") val userId: String,
    @SerialName("username") val username: String? = null
)

@Serializable
data class UserGroupMembersResponse(
    val members: List<UserGroupMember> = emptyList()
)

@Serializable
data class AddUserGroupMemberRequest(
    @SerialName("user_id") val userId: String
)
