package com.autogridmobility.rmsmqtt1.data.mobile

import com.autogridmobility.rmsmqtt1.network.SecureHttpConnectionFactory
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URLEncoder
import java.nio.charset.StandardCharsets

class MobileApiClient(
    private val baseUrl: String = "https://rms-iot.local:7443/api/mobile"
) : MobileAuthApi {
    private val json = Json { ignoreUnknownKeys = true }

    override suspend fun requestOtp(request: MobileRequestOtpRequest): Result<MobileRequestOtpResponse> {
        val payload = json.encodeToString(MobileRequestOtpRequest.serializer(), request)
        return post("/auth/request-otp", payload, null) { body ->
            json.decodeFromString(MobileRequestOtpResponse.serializer(), body)
        }
    }

    override suspend fun verifyOtp(request: MobileVerifyOtpRequest): Result<MobileVerifyOtpResponse> {
        val payload = json.encodeToString(MobileVerifyOtpRequest.serializer(), request)
        return post("/auth/verify", payload, null) { body ->
            json.decodeFromString(MobileVerifyOtpResponse.serializer(), body)
        }
    }

    override suspend fun getLatestDevOtp(phone: String): Result<MobileDevOtpResponse> {
        val encodedPhone = URLEncoder.encode(phone, StandardCharsets.UTF_8.toString())
        return get("/auth/dev-otp/latest?phone=$encodedPhone", null, mapOf("X-Internal-Test" to "1")) { body ->
            json.decodeFromString(MobileDevOtpResponse.serializer(), body)
        }
    }

    override suspend fun refreshToken(refreshToken: String): Result<MobileRefreshResponse> {
        val payload = json.encodeToString(MobileRefreshRequest.serializer(), MobileRefreshRequest(refreshToken))
        return post("/auth/refresh", payload, null) { body ->
            json.decodeFromString(MobileRefreshResponse.serializer(), body)
        }
    }

    override suspend fun logout(accessToken: String): Result<Unit> {
        return post("/auth/logout", null, accessToken) { _ -> Unit }
    }

    override suspend fun getAssignments(accessToken: String): Result<MobileAssignmentsResponse> {
        return get("/me/assignments", accessToken) { body ->
            json.decodeFromString(MobileAssignmentsResponse.serializer(), body)
        }
    }

    private suspend fun <TResp> post(
        path: String,
        bodyJson: String?,
        bearerToken: String?,
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "POST"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                setRequestProperty("Accept", "application/json")
                if (!bearerToken.isNullOrBlank()) {
                    setRequestProperty("Authorization", "Bearer $bearerToken")
                }
                if (bodyJson != null) {
                    doOutput = true
                    setRequestProperty("Content-Type", "application/json")
                }
            }

            if (bodyJson != null) {
                connection.outputStream.use { out -> out.write(bodyJson.toByteArray()) }
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

    private suspend fun <TResp> get(
        path: String,
        bearerToken: String?,
        extraHeaders: Map<String, String> = emptyMap(),
        decoder: (String) -> TResp
    ): Result<TResp> = withContext(Dispatchers.IO) {
        runCatching {
            val connection = (SecureHttpConnectionFactory.open("${baseUrl.trimEnd('/')}$path") as HttpURLConnection).apply {
                requestMethod = "GET"
                connectTimeout = 10_000
                readTimeout = 10_000
                doInput = true
                setRequestProperty("Accept", "application/json")
                if (!bearerToken.isNullOrBlank()) {
                    setRequestProperty("Authorization", "Bearer $bearerToken")
                }
                extraHeaders.forEach { (key, value) ->
                    setRequestProperty(key, value)
                }
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
        val parsedMessage = if (responseText.isBlank()) {
            "Request failed with status $code"
        } else {
            try {
            val err = json.decodeFromString(MobileErrorResponse.serializer(), responseText)
            err.message ?: err.error ?: "Request failed with status $code"
            } catch (_: Exception) {
                "Request failed with status $code"
            }
        }

        return if (code == 401) {
            "AUTH_401: $parsedMessage"
        } else {
            parsedMessage
        }
    }
}