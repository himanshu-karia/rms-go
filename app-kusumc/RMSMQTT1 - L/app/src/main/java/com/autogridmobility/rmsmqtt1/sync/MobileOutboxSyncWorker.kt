package com.autogridmobility.rmsmqtt1.sync

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import com.autogridmobility.rmsmqtt1.data.outbox.MobileRetentionHelper
import com.autogridmobility.rmsmqtt1.data.outbox.OutboxRepository
import com.autogridmobility.rmsmqtt1.data.outbox.SyncRunRepository
import com.autogridmobility.rmsmqtt1.utils.MobileSessionManager
import kotlin.math.min

class MobileOutboxSyncWorker(
    appContext: Context,
    workerParams: WorkerParameters
) : CoroutineWorker(appContext, workerParams) {

    private val repository = OutboxRepository(appContext)
    private val syncRunRepository = SyncRunRepository(appContext)
    private val retentionHelper = MobileRetentionHelper(appContext)
    private val sessionManager = MobileSessionManager(appContext)

    override suspend fun doWork(): Result {
        val runId = syncRunRepository.startRun()

        val accessToken = sessionManager.getAccessToken()
        if (accessToken.isBlank()) {
            syncRunRepository.completeFailure(
                runId = runId,
                processedCount = 0,
                sentCount = 0,
                failedCount = 0,
                errorMessage = "Missing access token"
            )
            return Result.retry()
        }

        val batch = repository.getReadyBatch(limit = 25)
        if (batch.isEmpty()) {
            syncRunRepository.completeSuccess(
                runId = runId,
                processedCount = 0,
                sentCount = 0,
                failedCount = 0
            )
            retentionHelper.pruneAll()
            return Result.success()
        }

        var hasRetryCandidate = false
        var sentCount = 0
        var failedCount = 0
        batch.forEach { event ->
            val nextAttempt = event.attemptCount + 1
            if (nextAttempt >= MAX_ATTEMPTS) {
                repository.markFailed(event.id)
                failedCount++
            } else {
                val retryDelayMs = computeBackoffMs(nextAttempt)
                repository.markRetry(event.id, nextAttempt, retryDelayMs)
                hasRetryCandidate = true
                sentCount++
            }
        }

        syncRunRepository.completeSuccess(
            runId = runId,
            processedCount = batch.size,
            sentCount = sentCount,
            failedCount = failedCount
        )
        retentionHelper.pruneAll()

        return if (hasRetryCandidate) Result.retry() else Result.success()
    }

    private fun computeBackoffMs(attempt: Int): Long {
        val raw = BASE_BACKOFF_MS * (1L shl min(attempt, 6))
        return min(raw, MAX_BACKOFF_MS)
    }

    companion object {
        const val WORK_NAME = "mobile_outbox_sync"
        private const val MAX_ATTEMPTS = 5
        private const val BASE_BACKOFF_MS = 15_000L
        private const val MAX_BACKOFF_MS = 30 * 60 * 1000L
    }
}
