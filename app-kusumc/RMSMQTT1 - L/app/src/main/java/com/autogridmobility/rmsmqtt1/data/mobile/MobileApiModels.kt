package com.autogridmobility.rmsmqtt1.data.mobile

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class MobileRequestOtpRequest(
    val phone: String,
    @SerialName("device_fingerprint") val deviceFingerprint: String,
    @SerialName("device_name") val deviceName: String,
    @SerialName("app_version") val appVersion: String
)

@Serializable
data class MobileRequestOtpResponse(
    val status: String,
    @SerialName("otp_ref") val otpRef: String
)

@Serializable
data class MobileVerifyOtpRequest(
    val phone: String,
    val otp: String,
    @SerialName("otp_ref") val otpRef: String
)

@Serializable
data class MobileUser(
    val id: String,
    val phone: String,
    @SerialName("display_name") val displayName: String? = null
)

@Serializable
data class MobileVerifyOtpResponse(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
    @SerialName("token_type") val tokenType: String,
    @SerialName("expires_in_sec") val expiresInSec: Int,
    val user: MobileUser
)

@Serializable
data class MobileDevOtpResponse(
    val phone: String,
    val otp: String,
    @SerialName("otp_ref") val otpRef: String? = null
)

@Serializable
data class MobileRefreshRequest(
    @SerialName("refresh_token") val refreshToken: String
)

@Serializable
data class MobileRefreshResponse(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
    @SerialName("token_type") val tokenType: String,
    @SerialName("expires_in_sec") val expiresInSec: Int
)

@Serializable
data class MobileAssignmentItem(
    @SerialName("project_id") val projectId: String,
    @SerialName("device_id") val deviceId: String? = null,
    val role: String? = null
)

@Serializable
data class MobileAssignmentsResponse(
    val items: List<MobileAssignmentItem>
)

@Serializable
data class MobileErrorResponse(
    val code: String? = null,
    val message: String? = null,
    @SerialName("request_id") val requestId: String? = null,
    val error: String? = null
)