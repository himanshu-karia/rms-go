package com.autogridmobility.rmsmqtt1.sync

import android.content.Context
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import java.util.concurrent.TimeUnit

object MobileSyncScheduler {

    fun schedulePeriodic(context: Context) {
        val constraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .setRequiresBatteryNotLow(true)
            .build()

        val request = PeriodicWorkRequestBuilder<MobileOutboxSyncWorker>(15, TimeUnit.MINUTES)
            .setConstraints(constraints)
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 15, TimeUnit.SECONDS)
            .addTag(TAG_PERIODIC)
            .build()

        WorkManager.getInstance(context.applicationContext).enqueueUniquePeriodicWork(
            MobileOutboxSyncWorker.WORK_NAME,
            ExistingPeriodicWorkPolicy.UPDATE,
            request
        )
    }

    fun triggerNow(context: Context) {
        val request = OneTimeWorkRequestBuilder<MobileOutboxSyncWorker>()
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build()
            )
            .addTag(TAG_IMMEDIATE)
            .build()

        WorkManager.getInstance(context.applicationContext).enqueueUniqueWork(
            "${MobileOutboxSyncWorker.WORK_NAME}_immediate",
            ExistingWorkPolicy.REPLACE,
            request
        )
    }

    private const val TAG_PERIODIC = "mobile-sync-periodic"
    private const val TAG_IMMEDIATE = "mobile-sync-immediate"
}
