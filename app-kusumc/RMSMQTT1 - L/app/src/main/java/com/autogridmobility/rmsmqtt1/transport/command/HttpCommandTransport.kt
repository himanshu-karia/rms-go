package com.autogridmobility.rmsmqtt1.transport.command

import com.autogridmobility.rmsmqtt1.network.SecureHttpConnectionFactory
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URLEncoder

class HttpCommandTransport(
    private val baseUrl: String = "https://rms-iot.local:7443/api"
) : CommandTransport {
    private val json = Json { ignoreUnknownKeys = true }

    override suspend fun sendCommand(request: CommandRequest, bearerToken: String): Result<CommandSendResult> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = openConnection(
                path = "/commands/send",
                method = "POST",
                bearerToken = bearerToken
            ).apply {
                doOutput = true
                setRequestProperty("Content-Type", "application/json")
            }

            val body = buildJsonObject {
                put("deviceId", request.deviceId)
                put("projectId", request.projectId)
                put("commandId", request.commandId)
                put("payload", buildJsonObject {
                    request.payload.forEach { (key, value) ->
                        put(key, value.toJsonElement())
                    }
                })
            }
            val raw = json.encodeToString(JsonObject.serializer(), body)
            connection.outputStream.use { output -> output.write(raw.toByteArray()) }

            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code !in 200..299) {
                throw IllegalStateException(parseErrorMessage(text, code))
            }

            val payload = parseJsonObject(text)
            val correlationId = payload.readString("correlationId") ?: payload.readString("correlation_id")
            val status = payload.readString("status") ?: "submitted"
            if (correlationId.isNullOrBlank()) {
                throw IllegalStateException("Command send response missing correlationId")
            }

            CommandSendResult(
                correlationId = correlationId,
                status = status,
                transport = CommandTransportType.HTTP
            )
        }
    }

    override suspend fun getLatestStatus(deviceId: String, projectId: String, bearerToken: String): Result<CommandStatusSnapshot?> = withContext(Dispatchers.IO) {
        runCatching {
            val encodedDevice = URLEncoder.encode(deviceId, Charsets.UTF_8.name())
            val encodedProject = URLEncoder.encode(projectId, Charsets.UTF_8.name())
            val connection = openConnection(
                path = "/commands?deviceId=$encodedDevice&projectId=$encodedProject&limit=1",
                method = "GET",
                bearerToken = bearerToken
            )

            val code = connection.responseCode
            val text = readResponseText(connection)
            if (code !in 200..299) {
                throw IllegalStateException(parseErrorMessage(text, code))
            }
            if (text.isBlank()) return@runCatching null

            val root = json.parseToJsonElement(text)
            val first = when (root) {
                is JsonArray -> root.firstOrNull()?.jsonObject
                is JsonObject -> root["items"]?.jsonArray?.firstOrNull()?.jsonObject
                else -> null
            } ?: return@runCatching null

            val correlationId = first.readString("correlationId") ?: first.readString("correlation_id") ?: ""
            val status = first.readString("status") ?: "unknown"
            val updatedAt = first.readString("updatedAt") ?: first.readString("updated_at")

            CommandStatusSnapshot(
                correlationId = correlationId,
                status = status,
                updatedAt = updatedAt,
                transport = CommandTransportType.HTTP
            )
        }
    }

    private fun openConnection(path: String, method: String, bearerToken: String): HttpURLConnection {
        return (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 10_000
            readTimeout = 10_000
            doInput = true
            setRequestProperty("Accept", "application/json")
            setRequestProperty("Authorization", "Bearer $bearerToken")
        }
    }

    private fun parseJsonObject(text: String): JsonObject {
        if (text.isBlank()) return buildJsonObject { }
        return runCatching { json.parseToJsonElement(text).jsonObject }.getOrElse { buildJsonObject { } }
    }

    private fun JsonObject.readString(key: String): String? {
        return this[key]?.jsonPrimitive?.contentOrNull
    }

    private fun readResponseText(connection: HttpURLConnection): String {
        val stream = if (connection.responseCode in 200..299) connection.inputStream else connection.errorStream
        if (stream == null) return ""
        return BufferedReader(InputStreamReader(stream)).use { it.readText() }
    }

    private fun parseErrorMessage(responseText: String, code: Int): String {
        if (responseText.isBlank()) return "Request failed with status $code"
        val parsed = runCatching { json.parseToJsonElement(responseText) }.getOrNull()
        val message = (parsed as? JsonObject)?.readString("message")
            ?: (parsed as? JsonObject)?.readString("error")
        return message ?: "Request failed with status $code"
    }

    private fun Any?.toJsonElement(): JsonElement {
        return when (this) {
            null -> JsonPrimitive("")
            is String -> JsonPrimitive(this)
            is Number -> JsonPrimitive(this)
            is Boolean -> JsonPrimitive(this)
            else -> JsonPrimitive(this.toString())
        }
    }
}
