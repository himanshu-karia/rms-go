package com.autogridmobility.rmsmqtt1.data.mobile

interface AdminApi {
    suspend fun listCommandCatalog(accessToken: String, projectId: String, deviceId: String): Result<List<CommandCatalogItem>>
    suspend fun upsertCommandCatalog(accessToken: String, request: UpsertCommandCatalogRequest): Result<UpsertCommandCatalogResponse>
    suspend fun deleteCommandCatalog(accessToken: String, commandId: String): Result<Unit>

    suspend fun listSimulatorSessions(accessToken: String, limit: Int = 25): Result<SimulatorSessionsResponse>
    suspend fun createSimulatorSession(accessToken: String, request: CreateSimulatorSessionRequest): Result<SimulatorSession>
    suspend fun revokeSimulatorSession(accessToken: String, sessionId: String): Result<SimulatorSession>

    suspend fun loadDeviceOpenCredentials(imei: String): Result<DeviceOpenResponse>
    suspend fun sendHttpsIngest(
        topic: String,
        imei: String,
        clientId: String,
        username: String,
        password: String,
        payloadJson: String
    ): Result<HttpsIngestResult>

    suspend fun listProjects(accessToken: String): Result<List<AdminProject>>
    suspend fun createProject(accessToken: String, request: UpsertProjectRequest): Result<AdminProject>
    suspend fun updateProject(accessToken: String, projectId: String, request: UpsertProjectRequest): Result<AdminProject>
    suspend fun deleteProject(accessToken: String, projectId: String): Result<Unit>

    suspend fun listUsers(accessToken: String): Result<AdminUsersResponse>
    suspend fun createUser(accessToken: String, request: CreateAdminUserRequest): Result<AdminUsersResponse>
    suspend fun updateUser(accessToken: String, userId: String, request: UpdateAdminUserRequest): Result<AdminUsersResponse>
    suspend fun resetUserPassword(accessToken: String, userId: String, request: ResetAdminPasswordRequest): Result<AdminUsersResponse>
    suspend fun deleteUser(accessToken: String, userId: String): Result<Unit>

    suspend fun listApiKeys(accessToken: String): Result<List<ApiKeyRecord>>
    suspend fun createApiKey(accessToken: String, request: CreateApiKeyRequest): Result<CreateApiKeyResponse>
    suspend fun revokeApiKey(accessToken: String, keyId: String): Result<Unit>

    suspend fun listStates(accessToken: String): Result<List<AdminStateItem>>
    suspend fun createState(accessToken: String, request: CreateAdminStateRequest): Result<AdminStateItem>
    suspend fun updateState(accessToken: String, stateId: String, request: UpdateAdminStateRequest): Result<AdminStateItem>
    suspend fun deleteState(accessToken: String, stateId: String): Result<Unit>

    suspend fun listAuthorities(accessToken: String, stateId: String? = null): Result<List<AdminAuthorityItem>>
    suspend fun createAuthority(accessToken: String, request: CreateAdminAuthorityRequest): Result<AdminAuthorityItem>
    suspend fun updateAuthority(accessToken: String, authorityId: String, request: UpdateAdminAuthorityRequest): Result<AdminAuthorityItem>
    suspend fun deleteAuthority(accessToken: String, authorityId: String): Result<Unit>

    suspend fun listVendors(accessToken: String, endpoint: String): Result<AdminVendorsResponse>
    suspend fun createVendor(accessToken: String, endpoint: String, request: CreateVendorRequest): Result<AdminVendorItem>
    suspend fun updateVendor(accessToken: String, endpoint: String, id: String, request: UpdateVendorRequest): Result<AdminVendorItem>
    suspend fun deleteVendor(accessToken: String, endpoint: String, id: String): Result<Unit>

    suspend fun listProtocolVersions(accessToken: String, projectId: String): Result<AdminProtocolVersionsResponse>
    suspend fun createProtocolVersion(accessToken: String, request: CreateProtocolVersionRequest): Result<AdminProtocolVersionItem>
    suspend fun updateProtocolVersion(accessToken: String, id: String, request: UpdateProtocolVersionRequest): Result<AdminProtocolVersionItem>

    suspend fun listOrgs(accessToken: String): Result<List<AdminOrg>>
    suspend fun createOrg(accessToken: String, request: UpsertOrgRequest): Result<AdminOrg>
    suspend fun updateOrg(accessToken: String, id: String, request: UpsertOrgRequest): Result<AdminOrg>

    suspend fun listUserGroups(accessToken: String): Result<AdminUserGroupsResponse>
    suspend fun createUserGroup(accessToken: String, request: UpsertUserGroupRequest): Result<AdminUserGroup>
    suspend fun updateUserGroup(accessToken: String, groupId: String, request: UpdateUserGroupRequest): Result<AdminUserGroup>
    suspend fun deleteUserGroup(accessToken: String, groupId: String): Result<Unit>
    suspend fun listUserGroupMembers(accessToken: String, groupId: String): Result<UserGroupMembersResponse>
    suspend fun addUserGroupMember(accessToken: String, groupId: String, request: AddUserGroupMemberRequest): Result<Unit>
    suspend fun removeUserGroupMember(accessToken: String, groupId: String, userId: String): Result<Unit>
}
