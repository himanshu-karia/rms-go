package com.autogridmobility.rmsmqtt1.data.outbox

import android.content.Context

class OutboxRepository(context: Context) {
    private val dao = OutboxDatabase.getInstance(context).outboxEventDao()

    suspend fun enqueue(projectId: String, deviceId: String, payloadJson: String): Long {
        val now = System.currentTimeMillis()
        val event = OutboxEventEntity(
            projectId = projectId,
            deviceId = deviceId,
            payloadJson = payloadJson,
            status = OutboxEventEntity.STATUS_PENDING,
            createdAt = now,
            updatedAt = now,
            nextRetryAt = 0
        )
        return dao.insert(event)
    }

    suspend fun getReadyBatch(limit: Int = 25): List<OutboxEventEntity> {
        return dao.getPendingBatch(nowMs = System.currentTimeMillis(), limit = limit)
    }

    suspend fun markSent(id: Long) {
        dao.markSent(id = id, updatedAt = System.currentTimeMillis())
    }

    suspend fun markRetry(id: Long, attemptCount: Int, retryAfterMs: Long) {
        dao.markRetry(
            id = id,
            attemptCount = attemptCount,
            nextRetryAt = System.currentTimeMillis() + retryAfterMs,
            updatedAt = System.currentTimeMillis()
        )
    }

    suspend fun markFailed(id: Long) {
        dao.markFailed(id = id, updatedAt = System.currentTimeMillis())
    }

    suspend fun pendingCount(): Int = dao.pendingCount()

    suspend fun getById(id: Long): OutboxEventEntity? = dao.getById(id)

    suspend fun pruneTerminalBefore(cutoffMs: Long): Int = dao.pruneTerminalBefore(cutoffMs)
}
