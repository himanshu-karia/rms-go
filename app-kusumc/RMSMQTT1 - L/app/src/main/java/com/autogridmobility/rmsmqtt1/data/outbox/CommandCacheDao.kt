package com.autogridmobility.rmsmqtt1.data.outbox

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query

@Dao
interface CommandCacheDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(entry: CommandCacheEntity)

    @Query("SELECT * FROM command_cache WHERE command_id = :commandId LIMIT 1")
    suspend fun getByCommandId(commandId: String): CommandCacheEntity?

    @Query("DELETE FROM command_cache WHERE expires_at < :nowMs")
    suspend fun pruneExpired(nowMs: Long): Int
}
