package com.autogridmobility.rmsmqtt1.flow

import android.app.Application
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.work.ListenableWorker
import androidx.work.testing.TestListenableWorkerBuilder
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAssignmentItem
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAssignmentsResponse
import com.autogridmobility.rmsmqtt1.data.mobile.MobileAuthApi
import com.autogridmobility.rmsmqtt1.data.mobile.MobileRefreshResponse
import com.autogridmobility.rmsmqtt1.data.mobile.MobileRequestOtpRequest
import com.autogridmobility.rmsmqtt1.data.mobile.MobileRequestOtpResponse
import com.autogridmobility.rmsmqtt1.data.mobile.MobileUser
import com.autogridmobility.rmsmqtt1.data.mobile.MobileVerifyOtpRequest
import com.autogridmobility.rmsmqtt1.data.mobile.MobileVerifyOtpResponse
import com.autogridmobility.rmsmqtt1.data.outbox.OutboxDatabase
import com.autogridmobility.rmsmqtt1.data.outbox.OutboxRepository
import com.autogridmobility.rmsmqtt1.sync.MobileOutboxSyncWorker
import com.autogridmobility.rmsmqtt1.sync.MobileSyncOrchestrator
import com.autogridmobility.rmsmqtt1.utils.MobileSessionManager
import com.autogridmobility.rmsmqtt1.utils.MobileSessionStore
import com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModel
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class MobileFlowInstrumentationTest {

    @Test
    fun login_assignment_enqueue_sync_skeleton() = runBlocking {
        val context = ApplicationProvider.getApplicationContext<Application>()
        OutboxDatabase.getInstance(context).clearAllTables()

        val fakeApi = FakeMobileAuthApi(
            assignments = listOf(
                MobileAssignmentItem(
                    projectId = "pm-kusum-solar-pump-msedcl",
                    deviceId = "imei-001",
                    role = "tech"
                )
            )
        )
        val fakeSession = FakeSessionStore()

        val vm = MobileAuthViewModel(
            application = context,
            apiClient = fakeApi,
            sessionManager = fakeSession,
            syncOrchestrator = NoOpSyncOrchestrator,
            ioDispatcher = Dispatchers.Unconfined
        )

        vm.updatePhone("9999999999")
        vm.requestOtp(onRequested = {})
        vm.verifyOtp("123456", onSuccess = {})

        assertTrue(vm.isAuthenticated.value)
        assertEquals(1, vm.assignments.value.size)

        val outboxRepository = OutboxRepository(context)
        val eventId = outboxRepository.enqueue(
            projectId = "pm-kusum-solar-pump-msedcl",
            deviceId = "imei-001",
            payloadJson = "{\"packet_type\":\"heartbeat\"}"
        )
        assertEquals(1, outboxRepository.pendingCount())

        MobileSessionManager(context).saveSession("token-abc", "refresh-abc", "9999999999")

        val worker = TestListenableWorkerBuilder<MobileOutboxSyncWorker>(context).build()
        val result = worker.doWork()

        assertTrue(result is ListenableWorker.Result.Retry)

        val updated = outboxRepository.getById(eventId)
        assertTrue(updated != null)
        assertEquals(1, updated?.attemptCount)
        assertTrue((updated?.nextRetryAt ?: 0L) > 0L)
    }

    private class FakeMobileAuthApi(
        private val assignments: List<MobileAssignmentItem>
    ) : MobileAuthApi {
        override suspend fun requestOtp(request: MobileRequestOtpRequest): Result<MobileRequestOtpResponse> {
            return Result.success(MobileRequestOtpResponse(status = "ok", otpRef = "otp-ref-1"))
        }

        override suspend fun verifyOtp(request: MobileVerifyOtpRequest): Result<MobileVerifyOtpResponse> {
            return Result.success(
                MobileVerifyOtpResponse(
                    accessToken = "access-token",
                    refreshToken = "refresh-token",
                    tokenType = "Bearer",
                    expiresInSec = 3600,
                    user = MobileUser(id = "u-1", phone = request.phone)
                )
            )
        }

        override suspend fun refreshToken(refreshToken: String): Result<MobileRefreshResponse> {
            return Result.success(
                MobileRefreshResponse(
                    accessToken = "access-token",
                    refreshToken = "refresh-token",
                    tokenType = "Bearer",
                    expiresInSec = 3600
                )
            )
        }

        override suspend fun logout(accessToken: String): Result<Unit> = Result.success(Unit)

        override suspend fun getAssignments(accessToken: String): Result<MobileAssignmentsResponse> {
            return Result.success(MobileAssignmentsResponse(items = assignments))
        }
    }

    private class FakeSessionStore : MobileSessionStore {
        private var accessToken: String = ""
        private var refreshToken: String = ""
        private var phone: String = ""

        override fun saveSession(accessToken: String, refreshToken: String, phone: String) {
            this.accessToken = accessToken
            this.refreshToken = refreshToken
            this.phone = phone
        }

        override fun clearSession() {
            accessToken = ""
            refreshToken = ""
            phone = ""
        }

        override fun getAccessToken(): String = accessToken

        override fun getRefreshToken(): String = refreshToken

        override fun getPhone(): String = phone

        override fun hasSession(): Boolean = accessToken.isNotBlank()
    }

    private object NoOpSyncOrchestrator : MobileSyncOrchestrator {
        override fun schedulePeriodic() = Unit

        override fun triggerNow() = Unit
    }
}
