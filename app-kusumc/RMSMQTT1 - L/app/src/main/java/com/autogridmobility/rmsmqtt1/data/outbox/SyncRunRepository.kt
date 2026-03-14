package com.autogridmobility.rmsmqtt1.data.outbox

import android.content.Context

class SyncRunRepository(context: Context) {
    private val dao = OutboxDatabase.getInstance(context).syncRunDao()

    suspend fun startRun(): Long {
        return dao.insert(
            SyncRunEntity(
                status = SyncRunEntity.STATUS_RUNNING,
                startedAt = System.currentTimeMillis()
            )
        )
    }

    suspend fun completeSuccess(runId: Long, processedCount: Int, sentCount: Int, failedCount: Int) {
        dao.completeRun(
            id = runId,
            status = SyncRunEntity.STATUS_SUCCESS,
            finishedAt = System.currentTimeMillis(),
            processedCount = processedCount,
            sentCount = sentCount,
            failedCount = failedCount,
            errorMessage = null
        )
    }

    suspend fun completeFailure(runId: Long, processedCount: Int, sentCount: Int, failedCount: Int, errorMessage: String?) {
        dao.completeRun(
            id = runId,
            status = SyncRunEntity.STATUS_FAILED,
            finishedAt = System.currentTimeMillis(),
            processedCount = processedCount,
            sentCount = sentCount,
            failedCount = failedCount,
            errorMessage = errorMessage
        )
    }

    suspend fun pruneFinishedBefore(cutoffMs: Long): Int = dao.pruneFinishedBefore(cutoffMs)
}
