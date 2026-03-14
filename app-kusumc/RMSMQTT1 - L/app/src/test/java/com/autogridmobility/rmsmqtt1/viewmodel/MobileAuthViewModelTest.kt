package com.autogridmobility.rmsmqtt1.viewmodel

import android.app.Application
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAssignmentItem
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAssignmentsResponse
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAuthApi
import com.autogridmobility.rmsmqtt1.data.mobile.MobileRefreshResponse
import com.autogridmobility.rmsmqtt1.data.mobile.MobileRequestOtpRequest
import com.autogridmobility.rmsmqtt1.data.mobile.MobileRequestOtpResponse
import com.autogridmobility.rmsmqtt1.data.mobile.MobileUser
import com.autogridmobility.rmsmqtt1.data.mobile.MobileVerifyOtpRequest
import com.autogridmobility.rmsmqtt1.data.mobile.MobileVerifyOtpResponse
import com.autogridmobility.rmsmqtt1.sync.MobileSyncOrchestrator
import com.autogridmobility.rmsmqtt1.utils.MobileSessionStore
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class MobileAuthViewModelTest {

    @Test
    fun bootstrapSession_refreshSuccess_setsAuthenticated_andLoadsAssignments() = runTest {
        val dispatcher = StandardTestDispatcher(testScheduler)
        val fakeApi = FakeMobileAuthApi().apply {
            refreshResult = Result.success(
                MobileRefreshResponse(
                    accessToken = "new-access",
                    refreshToken = "new-refresh",
                    tokenType = "Bearer",
                    expiresInSec = 3600
                )
            )
            assignmentsResult = Result.success(
                MobileAssignmentsResponse(
                    items = listOf(MobileAssignmentItem(projectId = "proj-1", deviceId = "dev-1", role = "tech"))
                )
            )
        }
        val sessionStore = FakeSessionStore(
            accessToken = "old-access",
            refreshToken = "old-refresh",
            phone = "9999999999"
        )

        val vm = MobileAuthViewModel(
            application = Application(),
            apiClient = fakeApi,
            sessionManager = sessionStore,
            syncOrchestrator = NoOpSyncOrchestrator,
            ioDispatcher = dispatcher
        )

        advanceUntilIdle()

        assertTrue(vm.isAuthenticated.value)
        assertEquals(1, vm.assignments.value.size)
        assertEquals("new-access", sessionStore.accessTokenValue)
        assertEquals("new-refresh", sessionStore.refreshTokenValue)
    }

    @Test
    fun bootstrapSession_refreshFailure_clearsSession_andUnauthenticates() = runTest {
        val dispatcher = StandardTestDispatcher(testScheduler)
        val fakeApi = FakeMobileAuthApi().apply {
            refreshResult = Result.failure(IllegalStateException("refresh failed"))
        }
        val sessionStore = FakeSessionStore(
            accessToken = "old-access",
            refreshToken = "bad-refresh",
            phone = "9999999999"
        )

        val vm = MobileAuthViewModel(
            application = Application(),
            apiClient = fakeApi,
            sessionManager = sessionStore,
            syncOrchestrator = NoOpSyncOrchestrator,
            ioDispatcher = dispatcher
        )

        advanceUntilIdle()

        assertFalse(vm.isAuthenticated.value)
        assertTrue(sessionStore.cleared)
        assertEquals("", sessionStore.accessTokenValue)
        assertEquals("", sessionStore.refreshTokenValue)
    }

    @Test
    fun verifyOtp_failure_setsError_andKeepsUnauthenticated() = runTest {
        val dispatcher = StandardTestDispatcher(testScheduler)
        val fakeApi = FakeMobileAuthApi().apply {
            verifyResult = Result.failure(IllegalStateException("invalid otp"))
        }
        val sessionStore = FakeSessionStore()

        val vm = MobileAuthViewModel(
            application = Application(),
            apiClient = fakeApi,
            sessionManager = sessionStore,
            syncOrchestrator = NoOpSyncOrchestrator,
            ioDispatcher = dispatcher
        )

        vm.updatePhone("9999999999")
        fakeApi.requestResult = Result.success(MobileRequestOtpResponse(status = "ok", otpRef = "ref-123"))
        vm.requestOtp(onRequested = {})
        advanceUntilIdle()

        vm.verifyOtp("123456", onSuccess = {})
        advanceUntilIdle()

        assertFalse(vm.isAuthenticated.value)
        assertEquals("invalid otp", vm.error.value)
        assertEquals("", sessionStore.accessTokenValue)
    }

    private class FakeMobileAuthApi : MobileAuthApi {
        var requestResult: Result<MobileRequestOtpResponse> =
            Result.success(MobileRequestOtpResponse(status = "ok", otpRef = "ref-default"))
        var verifyResult: Result<MobileVerifyOtpResponse> =
            Result.success(
                MobileVerifyOtpResponse(
                    accessToken = "access",
                    refreshToken = "refresh",
                    tokenType = "Bearer",
                    expiresInSec = 3600,
                    user = MobileUser(id = "u1", phone = "9999999999")
                )
            )
        var refreshResult: Result<MobileRefreshResponse> =
            Result.success(
                MobileRefreshResponse(
                    accessToken = "refresh-access",
                    refreshToken = "refresh-token",
                    tokenType = "Bearer",
                    expiresInSec = 3600
                )
            )
        var assignmentsResult: Result<MobileAssignmentsResponse> =
            Result.success(MobileAssignmentsResponse(emptyList()))

        override suspend fun requestOtp(request: MobileRequestOtpRequest): Result<MobileRequestOtpResponse> = requestResult

        override suspend fun verifyOtp(request: MobileVerifyOtpRequest): Result<MobileVerifyOtpResponse> = verifyResult

        override suspend fun refreshToken(refreshToken: String): Result<MobileRefreshResponse> = refreshResult

        override suspend fun logout(accessToken: String): Result<Unit> = Result.success(Unit)

        override suspend fun getAssignments(accessToken: String): Result<MobileAssignmentsResponse> = assignmentsResult
    }

    private class FakeSessionStore(
        accessToken: String = "",
        refreshToken: String = "",
        phone: String = ""
    ) : MobileSessionStore {
        var accessTokenValue: String = accessToken
        var refreshTokenValue: String = refreshToken
        var phoneValue: String = phone
        var cleared: Boolean = false

        override fun saveSession(accessToken: String, refreshToken: String, phone: String) {
            this.accessTokenValue = accessToken
            this.refreshTokenValue = refreshToken
            this.phoneValue = phone
        }

        override fun clearSession() {
            cleared = true
            accessTokenValue = ""
            refreshTokenValue = ""
            phoneValue = ""
        }

        override fun getAccessToken(): String = accessTokenValue

        override fun getRefreshToken(): String = refreshTokenValue

        override fun getPhone(): String = phoneValue

        override fun hasSession(): Boolean = accessTokenValue.isNotBlank()
    }

    private object NoOpSyncOrchestrator : MobileSyncOrchestrator {
        override fun schedulePeriodic() = Unit
        override fun triggerNow() = Unit
    }
}
