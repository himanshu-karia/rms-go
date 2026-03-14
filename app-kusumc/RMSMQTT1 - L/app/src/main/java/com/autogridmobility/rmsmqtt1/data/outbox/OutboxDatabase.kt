package com.autogridmobility.rmsmqtt1.data.outbox

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase

@Database(
    entities = [
        OutboxEventEntity::class,
        SyncRunEntity::class,
        CommandCacheEntity::class
    ],
    version = 2,
    exportSchema = false
)
abstract class OutboxDatabase : RoomDatabase() {
    abstract fun outboxEventDao(): OutboxEventDao
    abstract fun syncRunDao(): SyncRunDao
    abstract fun commandCacheDao(): CommandCacheDao

    companion object {
        @Volatile
        private var instance: OutboxDatabase? = null

        fun getInstance(context: Context): OutboxDatabase {
            return instance ?: synchronized(this) {
                instance ?: Room.databaseBuilder(
                    context.applicationContext,
                    OutboxDatabase::class.java,
                    "mobile_sync.db"
                ).fallbackToDestructiveMigration().build().also { instance = it }
            }
        }
    }
}
