package com.autogridmobility.rmsmqtt1.data.mobile

interface MobileAuthApi {
    suspend fun requestOtp(request: MobileRequestOtpRequest): Result<MobileRequestOtpResponse>
    suspend fun getLatestDevOtp(phone: String): Result<MobileDevOtpResponse>
    suspend fun verifyOtp(request: MobileVerifyOtpRequest): Result<MobileVerifyOtpResponse>
    suspend fun refreshToken(refreshToken: String): Result<MobileRefreshResponse>
    suspend fun logout(accessToken: String): Result<Unit>
    suspend fun getAssignments(accessToken: String): Result<MobileAssignmentsResponse>
}
