package com.autogridmobility.rmsmqtt1.data.outbox

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey

@Entity(
    tableName = "sync_runs",
    indices = [
        Index(value = ["status", "started_at"]),
        Index(value = ["finished_at"])
    ]
)
data class SyncRunEntity(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    @ColumnInfo(name = "status")
    val status: String = STATUS_RUNNING,
    @ColumnInfo(name = "started_at")
    val startedAt: Long = System.currentTimeMillis(),
    @ColumnInfo(name = "finished_at")
    val finishedAt: Long? = null,
    @ColumnInfo(name = "processed_count")
    val processedCount: Int = 0,
    @ColumnInfo(name = "sent_count")
    val sentCount: Int = 0,
    @ColumnInfo(name = "failed_count")
    val failedCount: Int = 0,
    @ColumnInfo(name = "error_message")
    val errorMessage: String? = null
) {
    companion object {
        const val STATUS_RUNNING = "running"
        const val STATUS_SUCCESS = "success"
        const val STATUS_FAILED = "failed"
    }
}
