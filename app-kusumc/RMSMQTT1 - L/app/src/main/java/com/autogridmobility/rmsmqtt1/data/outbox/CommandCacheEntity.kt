package com.autogridmobility.rmsmqtt1.data.outbox

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey

@Entity(
    tableName = "command_cache",
    indices = [
        Index(value = ["device_id", "updated_at"]),
        Index(value = ["expires_at"])
    ]
)
data class CommandCacheEntity(
    @PrimaryKey
    @ColumnInfo(name = "command_id")
    val commandId: String,
    @ColumnInfo(name = "project_id")
    val projectId: String,
    @ColumnInfo(name = "device_id")
    val deviceId: String,
    @ColumnInfo(name = "status")
    val status: String,
    @ColumnInfo(name = "payload_json")
    val payloadJson: String? = null,
    @ColumnInfo(name = "updated_at")
    val updatedAt: Long = System.currentTimeMillis(),
    @ColumnInfo(name = "expires_at")
    val expiresAt: Long = System.currentTimeMillis() + DEFAULT_TTL_MS
) {
    companion object {
        const val DEFAULT_TTL_MS = 7L * 24L * 60L * 60L * 1000L
    }
}
