package com.autogridmobility.rmsmqtt1.data.mobile

import com.autogridmobility.rmsmqtt1.network.SecureHttpConnectionFactory
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URLEncoder
import java.nio.charset.StandardCharsets
import java.util.Base64

class AdminApiClient(
    private val baseUrl: String = "https://rms-iot.local:7443/api"
) : AdminApi {
    private val json = Json { ignoreUnknownKeys = true }

    override suspend fun listCommandCatalog(accessToken: String, projectId: String, deviceId: String): Result<List<CommandCatalogItem>> {
        val project = URLEncoder.encode(projectId, StandardCharsets.UTF_8.toString())
        val device = URLEncoder.encode(deviceId, StandardCharsets.UTF_8.toString())
        return get("/commands/catalog-admin?projectId=$project&deviceId=$device", accessToken) { body ->
            json.decodeFromString(ListSerializer(CommandCatalogItem.serializer()), body)
        }
    }

    override suspend fun upsertCommandCatalog(
        accessToken: String,
        request: UpsertCommandCatalogRequest
    ): Result<UpsertCommandCatalogResponse> {
        val payload = json.encodeToString(UpsertCommandCatalogRequest.serializer(), request)
        return post("/commands/catalog", payload, accessToken) { body ->
            json.decodeFromString(UpsertCommandCatalogResponse.serializer(), body)
        }
    }

    override suspend fun deleteCommandCatalog(accessToken: String, commandId: String): Result<Unit> {
        val id = URLEncoder.encode(commandId, StandardCharsets.UTF_8.toString())
        return delete("/commands/catalog/$id", accessToken)
    }

    override suspend fun listSimulatorSessions(accessToken: String, limit: Int): Result<SimulatorSessionsResponse> {
        return get("/simulator/sessions?limit=$limit", accessToken) { body ->
            json.decodeFromString(SimulatorSessionsResponse.serializer(), body)
        }
    }

    override suspend fun createSimulatorSession(
        accessToken: String,
        request: CreateSimulatorSessionRequest
    ): Result<SimulatorSession> {
        val payload = json.encodeToString(CreateSimulatorSessionRequest.serializer(), request)
        return post("/simulator/sessions", payload, accessToken) { body ->
            json.decodeFromString(SimulatorSession.serializer(), body)
        }
    }

    override suspend fun revokeSimulatorSession(accessToken: String, sessionId: String): Result<SimulatorSession> {
        val id = URLEncoder.encode(sessionId, StandardCharsets.UTF_8.toString())
        return delete("/simulator/sessions/$id", accessToken) { body ->
            json.decodeFromString(SimulatorSession.serializer(), body)
        }
    }

    override suspend fun loadDeviceOpenCredentials(imei: String): Result<DeviceOpenResponse> {
        val encodedImei = URLEncoder.encode(imei, StandardCharsets.UTF_8.toString())
        return getWithoutBearer("/device-open/credentials/local?imei=$encodedImei") { body ->
            json.decodeFromString(DeviceOpenResponse.serializer(), body)
        }
    }

    override suspend fun sendHttpsIngest(
        topic: String,
        imei: String,
        clientId: String,
        username: String,
        password: String,
        payloadJson: String
    ): Result<HttpsIngestResult> = withContext(Dispatchers.IO) {
        runCatching {
            val sanitizedTopic = topic.trim().trimStart('/')
            val endpoint = "${baseUrl.trimEnd('/')}/telemetry/$sanitizedTopic"
            val basic = Base64.getEncoder().encodeToString("$username:$password".toByteArray())
            val connection = (SecureHttpConnectionFactory.open(endpoint) as HttpURLConnection).apply {
                requestMethod = "POST"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                doOutput = true
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Content-Type", "application/json")
                setRequestProperty("Authorization", "Basic $basic")
                setRequestProperty("X-RMS-IMEI", imei)
                setRequestProperty("X-RMS-ClientId", clientId)
                setRequestProperty("X-RMS-MsgId", "mobile-sim-${System.currentTimeMillis()}")
            }

            connection.outputStream.use { out -> out.write(payloadJson.toByteArray()) }
            val code = connection.responseCode
            val body = readResponseText(connection)
            HttpsIngestResult(code, body)
        }
    }

    override suspend fun listProjects(accessToken: String): Result<List<AdminProject>> {
        return get("/admin/projects", accessToken) { body ->
            json.decodeFromString(ListSerializer(AdminProject.serializer()), body)
        }
    }

    override suspend fun createProject(accessToken: String, request: UpsertProjectRequest): Result<AdminProject> {
        val payload = json.encodeToString(UpsertProjectRequest.serializer(), request)
        return post("/admin/projects", payload, accessToken) { body ->
            json.decodeFromString(AdminProject.serializer(), body)
        }
    }

    override suspend fun updateProject(accessToken: String, projectId: String, request: UpsertProjectRequest): Result<AdminProject> {
        val id = URLEncoder.encode(projectId, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpsertProjectRequest.serializer(), request)
        return put("/admin/projects/$id", payload, accessToken) { body ->
            json.decodeFromString(AdminProject.serializer(), body)
        }
    }

    override suspend fun deleteProject(accessToken: String, projectId: String): Result<Unit> {
        val id = URLEncoder.encode(projectId, StandardCharsets.UTF_8.toString())
        return delete("/admin/projects/$id", accessToken)
    }

    override suspend fun listUsers(accessToken: String): Result<AdminUsersResponse> {
        return get("/admin/users", accessToken) { body ->
            json.decodeFromString(AdminUsersResponse.serializer(), body)
        }
    }

    override suspend fun createUser(accessToken: String, request: CreateAdminUserRequest): Result<AdminUsersResponse> {
        val payload = json.encodeToString(CreateAdminUserRequest.serializer(), request)
        return post("/admin/users", payload, accessToken) { body ->
            val user = json.parseToJsonElement(body).jsonObject["user"]
            val userObj = user?.jsonObject ?: JsonObject(emptyMap())
            val wrapped = JsonObject(mapOf("users" to kotlinx.serialization.json.JsonArray(listOf(userObj))))
            json.decodeFromString(AdminUsersResponse.serializer(), wrapped.toString())
        }
    }

    override suspend fun updateUser(accessToken: String, userId: String, request: UpdateAdminUserRequest): Result<AdminUsersResponse> {
        val id = URLEncoder.encode(userId, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpdateAdminUserRequest.serializer(), request)
        return patch("/admin/users/$id", payload, accessToken) { body ->
            val user = json.parseToJsonElement(body).jsonObject["user"]
            val userObj = user?.jsonObject ?: JsonObject(emptyMap())
            val wrapped = JsonObject(mapOf("users" to kotlinx.serialization.json.JsonArray(listOf(userObj))))
            json.decodeFromString(AdminUsersResponse.serializer(), wrapped.toString())
        }
    }

    override suspend fun resetUserPassword(accessToken: String, userId: String, request: ResetAdminPasswordRequest): Result<AdminUsersResponse> {
        val id = URLEncoder.encode(userId, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(ResetAdminPasswordRequest.serializer(), request)
        return post("/admin/users/$id/password", payload, accessToken) { body ->
            val user = json.parseToJsonElement(body).jsonObject["user"]
            val userObj = user?.jsonObject ?: JsonObject(emptyMap())
            val wrapped = JsonObject(mapOf("users" to kotlinx.serialization.json.JsonArray(listOf(userObj))))
            json.decodeFromString(AdminUsersResponse.serializer(), wrapped.toString())
        }
    }

    override suspend fun deleteUser(accessToken: String, userId: String): Result<Unit> {
        val id = URLEncoder.encode(userId, StandardCharsets.UTF_8.toString())
        return delete("/admin/users/$id", accessToken)
    }

    override suspend fun listApiKeys(accessToken: String): Result<List<ApiKeyRecord>> {
        return get("/admin/apikeys", accessToken) { body ->
            json.decodeFromString(ListSerializer(ApiKeyRecord.serializer()), body)
        }
    }

    override suspend fun createApiKey(accessToken: String, request: CreateApiKeyRequest): Result<CreateApiKeyResponse> {
        val payload = json.encodeToString(CreateApiKeyRequest.serializer(), request)
        return post("/admin/apikeys", payload, accessToken) { body ->
            json.decodeFromString(CreateApiKeyResponse.serializer(), body)
        }
    }

    override suspend fun revokeApiKey(accessToken: String, keyId: String): Result<Unit> {
        val id = URLEncoder.encode(keyId, StandardCharsets.UTF_8.toString())
        return delete("/admin/apikeys/$id", accessToken)
    }

    override suspend fun listStates(accessToken: String): Result<List<AdminStateItem>> {
        return get("/admin/states", accessToken) { body ->
            json.decodeFromString(ListSerializer(AdminStateItem.serializer()), body)
        }
    }

    override suspend fun createState(accessToken: String, request: CreateAdminStateRequest): Result<AdminStateItem> {
        val payload = json.encodeToString(CreateAdminStateRequest.serializer(), request)
        return post("/admin/states", payload, accessToken) { body ->
            val state = json.parseToJsonElement(body).jsonObject["state"]
            json.decodeFromString(AdminStateItem.serializer(), state.toString())
        }
    }

    override suspend fun updateState(accessToken: String, stateId: String, request: UpdateAdminStateRequest): Result<AdminStateItem> {
        val id = URLEncoder.encode(stateId, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpdateAdminStateRequest.serializer(), request)
        return put("/admin/states/$id", payload, accessToken) { body ->
            val state = json.parseToJsonElement(body).jsonObject["state"]
            json.decodeFromString(AdminStateItem.serializer(), state.toString())
        }
    }

    override suspend fun deleteState(accessToken: String, stateId: String): Result<Unit> {
        val id = URLEncoder.encode(stateId, StandardCharsets.UTF_8.toString())
        return delete("/admin/states/$id", accessToken)
    }

    override suspend fun listAuthorities(accessToken: String, stateId: String?): Result<List<AdminAuthorityItem>> {
        val query = if (!stateId.isNullOrBlank()) "?stateId=${URLEncoder.encode(stateId, StandardCharsets.UTF_8.toString())}" else ""
        return get("/admin/state-authorities$query", accessToken) { body ->
            json.decodeFromString(ListSerializer(AdminAuthorityItem.serializer()), body)
        }
    }

    override suspend fun createAuthority(accessToken: String, request: CreateAdminAuthorityRequest): Result<AdminAuthorityItem> {
        val payload = json.encodeToString(CreateAdminAuthorityRequest.serializer(), request)
        return post("/admin/state-authorities", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["authority"]
            json.decodeFromString(AdminAuthorityItem.serializer(), item.toString())
        }
    }

    override suspend fun updateAuthority(accessToken: String, authorityId: String, request: UpdateAdminAuthorityRequest): Result<AdminAuthorityItem> {
        val id = URLEncoder.encode(authorityId, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpdateAdminAuthorityRequest.serializer(), request)
        return patch("/admin/state-authorities/$id", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["authority"]
            json.decodeFromString(AdminAuthorityItem.serializer(), item.toString())
        }
    }

    override suspend fun deleteAuthority(accessToken: String, authorityId: String): Result<Unit> {
        val id = URLEncoder.encode(authorityId, StandardCharsets.UTF_8.toString())
        return delete("/admin/state-authorities/$id", accessToken)
    }

    override suspend fun listVendors(accessToken: String, endpoint: String): Result<AdminVendorsResponse> {
        return get("/admin/$endpoint", accessToken) { body ->
            json.decodeFromString(AdminVendorsResponse.serializer(), body)
        }
    }

    override suspend fun createVendor(accessToken: String, endpoint: String, request: CreateVendorRequest): Result<AdminVendorItem> {
        val payload = json.encodeToString(CreateVendorRequest.serializer(), request)
        return post("/admin/$endpoint", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["vendor"]
            json.decodeFromString(AdminVendorItem.serializer(), item.toString())
        }
    }

    override suspend fun updateVendor(accessToken: String, endpoint: String, id: String, request: UpdateVendorRequest): Result<AdminVendorItem> {
        val encodedId = URLEncoder.encode(id, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpdateVendorRequest.serializer(), request)
        return patch("/admin/$endpoint/$encodedId", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["vendor"]
            json.decodeFromString(AdminVendorItem.serializer(), item.toString())
        }
    }

    override suspend fun deleteVendor(accessToken: String, endpoint: String, id: String): Result<Unit> {
        val encodedId = URLEncoder.encode(id, StandardCharsets.UTF_8.toString())
        return delete("/admin/$endpoint/$encodedId", accessToken)
    }

    override suspend fun listProtocolVersions(accessToken: String, projectId: String): Result<AdminProtocolVersionsResponse> {
        val project = URLEncoder.encode(projectId, StandardCharsets.UTF_8.toString())
        return get("/admin/protocol-versions?projectId=$project", accessToken) { body ->
            json.decodeFromString(AdminProtocolVersionsResponse.serializer(), body)
        }
    }

    override suspend fun createProtocolVersion(accessToken: String, request: CreateProtocolVersionRequest): Result<AdminProtocolVersionItem> {
        val payload = json.encodeToString(CreateProtocolVersionRequest.serializer(), request)
        return post("/admin/protocol-versions", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["protocol_version"]
            json.decodeFromString(AdminProtocolVersionItem.serializer(), item.toString())
        }
    }

    override suspend fun updateProtocolVersion(accessToken: String, id: String, request: UpdateProtocolVersionRequest): Result<AdminProtocolVersionItem> {
        val encodedId = URLEncoder.encode(id, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpdateProtocolVersionRequest.serializer(), request)
        return patch("/admin/protocol-versions/$encodedId", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["protocol_version"]
            json.decodeFromString(AdminProtocolVersionItem.serializer(), item.toString())
        }
    }

    override suspend fun listOrgs(accessToken: String): Result<List<AdminOrg>> {
        return get("/orgs", accessToken) { body ->
            json.decodeFromString(ListSerializer(AdminOrg.serializer()), body)
        }
    }

    override suspend fun createOrg(accessToken: String, request: UpsertOrgRequest): Result<AdminOrg> {
        val payload = json.encodeToString(UpsertOrgRequest.serializer(), request)
        return post("/orgs", payload, accessToken) { body ->
            json.decodeFromString(AdminOrg.serializer(), body)
        }
    }

    override suspend fun updateOrg(accessToken: String, id: String, request: UpsertOrgRequest): Result<AdminOrg> {
        val encodedId = URLEncoder.encode(id, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpsertOrgRequest.serializer(), request)
        return put("/orgs/$encodedId", payload, accessToken) { body ->
            json.decodeFromString(AdminOrg.serializer(), body)
        }
    }

    override suspend fun listUserGroups(accessToken: String): Result<AdminUserGroupsResponse> {
        return get("/user-groups", accessToken) { body ->
            json.decodeFromString(AdminUserGroupsResponse.serializer(), body)
        }
    }

    override suspend fun createUserGroup(accessToken: String, request: UpsertUserGroupRequest): Result<AdminUserGroup> {
        val payload = json.encodeToString(UpsertUserGroupRequest.serializer(), request)
        return post("/user-groups", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["group"]
            json.decodeFromString(AdminUserGroup.serializer(), item.toString())
        }
    }

    override suspend fun updateUserGroup(accessToken: String, groupId: String, request: UpdateUserGroupRequest): Result<AdminUserGroup> {
        val encodedGroupId = URLEncoder.encode(groupId, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(UpdateUserGroupRequest.serializer(), request)
        return patch("/user-groups/$encodedGroupId", payload, accessToken) { body ->
            val item = json.parseToJsonElement(body).jsonObject["group"]
            json.decodeFromString(AdminUserGroup.serializer(), item.toString())
        }
    }

    override suspend fun deleteUserGroup(accessToken: String, groupId: String): Result<Unit> {
        val encodedGroupId = URLEncoder.encode(groupId, StandardCharsets.UTF_8.toString())
        return delete("/user-groups/$encodedGroupId", accessToken)
    }

    override suspend fun listUserGroupMembers(accessToken: String, groupId: String): Result<UserGroupMembersResponse> {
        val encodedGroupId = URLEncoder.encode(groupId, StandardCharsets.UTF_8.toString())
        return get("/user-groups/$encodedGroupId/members", accessToken) { body ->
            json.decodeFromString(UserGroupMembersResponse.serializer(), body)
        }
    }

    override suspend fun addUserGroupMember(accessToken: String, groupId: String, request: AddUserGroupMemberRequest): Result<Unit> {
        val encodedGroupId = URLEncoder.encode(groupId, StandardCharsets.UTF_8.toString())
        val payload = json.encodeToString(AddUserGroupMemberRequest.serializer(), request)
        return post("/user-groups/$encodedGroupId/members", payload, accessToken) { Unit }
    }

    override suspend fun removeUserGroupMember(accessToken: String, groupId: String, userId: String): Result<Unit> {
        val encodedGroupId = URLEncoder.encode(groupId, StandardCharsets.UTF_8.toString())
        val encodedUserId = URLEncoder.encode(userId, StandardCharsets.UTF_8.toString())
        return delete("/user-groups/$encodedGroupId/members/$encodedUserId", accessToken)
    }

    private suspend fun <TResp> post(
        path: String,
        bodyJson: String,
        bearerToken: String,
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "POST"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                doOutput = true
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Content-Type", "application/json")
                setRequestProperty("Authorization", "Bearer $bearerToken")
            }

            connection.outputStream.use { out -> out.write(bodyJson.toByteArray()) }

            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code in 200..299) {
                decoder(text)
            } else {
                throw IllegalStateException(parseErrorMessage(text, code))
            }
        }
    }

    private suspend fun <TResp> put(
        path: String,
        bodyJson: String,
        bearerToken: String,
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "PUT"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                doOutput = true
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Content-Type", "application/json")
                setRequestProperty("Authorization", "Bearer $bearerToken")
            }

            connection.outputStream.use { out -> out.write(bodyJson.toByteArray()) }
            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code in 200..299) decoder(text) else throw IllegalStateException(parseErrorMessage(text, code))
        }
    }

    private suspend fun <TResp> patch(
        path: String,
        bodyJson: String,
        bearerToken: String,
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "PATCH"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                doOutput = true
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Content-Type", "application/json")
                setRequestProperty("Authorization", "Bearer $bearerToken")
            }

            connection.outputStream.use { out -> out.write(bodyJson.toByteArray()) }
            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code in 200..299) decoder(text) else throw IllegalStateException(parseErrorMessage(text, code))
        }
    }

    private suspend fun <TResp> get(
        path: String,
        bearerToken: String,
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "GET"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Authorization", "Bearer $bearerToken")
            }

            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code in 200..299) {
                decoder(text)
            } else {
                throw IllegalStateException(parseErrorMessage(text, code))
            }
        }
    }

    private suspend fun <TResp> getWithoutBearer(
        path: String,
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "GET"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                setRequestProperty("Accept", "application/json")
            }

            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code in 200..299) {
                decoder(text)
            } else {
                throw IllegalStateException(parseErrorMessage(text, code))
            }
        }
    }

    private suspend fun delete(path: String, bearerToken: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "DELETE"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Authorization", "Bearer $bearerToken")
            }

            val code = connection.responseCode
            if (code !in 200..299) {
                val text = readResponseText(connection)
                throw IllegalStateException(parseErrorMessage(text, code))
            }
        }
    }

    private suspend fun <TResp> delete(
        path: String,
        bearerToken: String,
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "DELETE"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                setRequestProperty("Accept", "application/json")
                setRequestProperty("Authorization", "Bearer $bearerToken")
            }

            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code in 200..299) {
                decoder(text)
            } else {
                throw IllegalStateException(parseErrorMessage(text, code))
            }
        }
    }

    private fun readResponseText(connection: HttpURLConnection): String {
        val stream = if (connection.responseCode in 200..299) connection.inputStream else connection.errorStream
        if (stream == null) return ""
        return BufferedReader(InputStreamReader(stream)).use { it.readText() }
    }

    private fun parseErrorMessage(responseText: String, code: Int): String {
        if (responseText.isBlank()) return "Request failed with status $code"
        return try {
            val err = json.decodeFromString(MobileErrorResponse.serializer(), responseText)
            err.message ?: err.error ?: "Request failed with status $code"
        } catch (_: Exception) {
            "Request failed with status $code"
        }
    }
}
