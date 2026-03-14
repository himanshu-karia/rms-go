package com.autogridmobility.rmsmqtt1.data.outbox

import android.content.Context

data class RetentionPruneResult(
    val outboxRows: Int,
    val syncRunRows: Int,
    val commandCacheRows: Int
)

class MobileRetentionHelper(context: Context) {
    private val outboxRepository = OutboxRepository(context)
    private val syncRunRepository = SyncRunRepository(context)
    private val commandCacheRepository = CommandCacheRepository(context)

    suspend fun pruneAll(nowMs: Long = System.currentTimeMillis()): RetentionPruneResult {
        val outboxCutoff = nowMs - OUTBOX_RETENTION_MS
        val syncRunCutoff = nowMs - SYNC_RUN_RETENTION_MS

        val outboxRows = outboxRepository.pruneTerminalBefore(outboxCutoff)
        val syncRunRows = syncRunRepository.pruneFinishedBefore(syncRunCutoff)
        val commandCacheRows = commandCacheRepository.pruneExpired(nowMs)

        return RetentionPruneResult(
            outboxRows = outboxRows,
            syncRunRows = syncRunRows,
            commandCacheRows = commandCacheRows
        )
    }

    companion object {
        const val OUTBOX_RETENTION_MS = 14L * 24L * 60L * 60L * 1000L
        const val SYNC_RUN_RETENTION_MS = 30L * 24L * 60L * 60L * 1000L
    }
}
