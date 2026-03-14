package com.autogridmobility.rmsmqtt1.sync

import android.content.Context

interface MobileSyncOrchestrator {
    fun schedulePeriodic()
    fun triggerNow()
}

class WorkManagerMobileSyncOrchestrator(
    private val context: Context
) : MobileSyncOrchestrator {
    override fun schedulePeriodic() {
        MobileSyncScheduler.schedulePeriodic(context)
    }

    override fun triggerNow() {
        MobileSyncScheduler.triggerNow(context)
    }
}
