package com.autogridmobility.rmsmqtt1.data.outbox

import android.content.Context

class CommandCacheRepository(context: Context) {
    private val dao = OutboxDatabase.getInstance(context).commandCacheDao()

    suspend fun upsert(
        commandId: String,
        projectId: String,
        deviceId: String,
        status: String,
        payloadJson: String?,
        ttlMs: Long = CommandCacheEntity.DEFAULT_TTL_MS
    ) {
        val now = System.currentTimeMillis()
        dao.upsert(
            CommandCacheEntity(
                commandId = commandId,
                projectId = projectId,
                deviceId = deviceId,
                status = status,
                payloadJson = payloadJson,
                updatedAt = now,
                expiresAt = now + ttlMs
            )
        )
    }

    suspend fun get(commandId: String): CommandCacheEntity? = dao.getByCommandId(commandId)

    suspend fun pruneExpired(nowMs: Long = System.currentTimeMillis()): Int = dao.pruneExpired(nowMs)
}
