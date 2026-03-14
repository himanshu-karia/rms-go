package com.autogridmobility.rmsmqtt1.data.outbox

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query

@Dao
interface OutboxEventDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(event: OutboxEventEntity): Long

    @Query(
        """
        SELECT * FROM outbox_events
        WHERE status = :pendingStatus AND (next_retry_at = 0 OR next_retry_at <= :nowMs)
        ORDER BY created_at ASC
        LIMIT :limit
        """
    )
    suspend fun getPendingBatch(
        pendingStatus: String = OutboxEventEntity.STATUS_PENDING,
        nowMs: Long,
        limit: Int
    ): List<OutboxEventEntity>

    @Query(
        """
        UPDATE outbox_events
        SET status = :sentStatus,
            updated_at = :updatedAt
        WHERE id = :id
        """
    )
    suspend fun markSent(
        id: Long,
        sentStatus: String = OutboxEventEntity.STATUS_SENT,
        updatedAt: Long
    )

    @Query(
        """
        UPDATE outbox_events
        SET attempt_count = :attemptCount,
            next_retry_at = :nextRetryAt,
            updated_at = :updatedAt
        WHERE id = :id
        """
    )
    suspend fun markRetry(
        id: Long,
        attemptCount: Int,
        nextRetryAt: Long,
        updatedAt: Long
    )

    @Query(
        """
        UPDATE outbox_events
        SET status = :failedStatus,
            updated_at = :updatedAt
        WHERE id = :id
        """
    )
    suspend fun markFailed(
        id: Long,
        failedStatus: String = OutboxEventEntity.STATUS_FAILED,
        updatedAt: Long
    )

    @Query("SELECT COUNT(*) FROM outbox_events WHERE status = :pendingStatus")
    suspend fun pendingCount(pendingStatus: String = OutboxEventEntity.STATUS_PENDING): Int

    @Query("SELECT * FROM outbox_events WHERE id = :id LIMIT 1")
    suspend fun getById(id: Long): OutboxEventEntity?

    @Query(
        """
        DELETE FROM outbox_events
        WHERE status IN (:sentStatus, :failedStatus)
          AND updated_at < :cutoffMs
        """
    )
    suspend fun pruneTerminalBefore(
        cutoffMs: Long,
        sentStatus: String = OutboxEventEntity.STATUS_SENT,
        failedStatus: String = OutboxEventEntity.STATUS_FAILED
    ): Int
}
