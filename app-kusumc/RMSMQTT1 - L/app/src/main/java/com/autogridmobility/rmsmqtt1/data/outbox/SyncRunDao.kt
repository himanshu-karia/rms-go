package com.autogridmobility.rmsmqtt1.data.outbox

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query

@Dao
interface SyncRunDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(syncRun: SyncRunEntity): Long

    @Query(
        """
        UPDATE sync_runs
        SET status = :status,
            finished_at = :finishedAt,
            processed_count = :processedCount,
            sent_count = :sentCount,
            failed_count = :failedCount,
            error_message = :errorMessage
        WHERE id = :id
        """
    )
    suspend fun completeRun(
        id: Long,
        status: String,
        finishedAt: Long,
        processedCount: Int,
        sentCount: Int,
        failedCount: Int,
        errorMessage: String?
    )

    @Query("DELETE FROM sync_runs WHERE finished_at IS NOT NULL AND finished_at < :cutoffMs")
    suspend fun pruneFinishedBefore(cutoffMs: Long): Int
}
