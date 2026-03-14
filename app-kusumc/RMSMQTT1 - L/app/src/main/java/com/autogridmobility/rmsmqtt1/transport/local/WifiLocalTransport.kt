package com.autogridmobility.rmsmqtt1.transport.local

class WifiLocalTransport : LocalTransport {
    override val type: LocalTransportType = LocalTransportType.WIFI_LOCAL

    override suspend fun connect(target: String): Result<Unit> {
        return Result.failure(UnsupportedOperationException("WiFi-local transport not implemented yet"))
    }

    override suspend fun disconnect(): Result<Unit> {
        return Result.success(Unit)
    }

    override suspend fun readPacket(): Result<LocalTelemetryPacket?> {
        return Result.failure(UnsupportedOperationException("WiFi-local read is not implemented yet"))
    }

    override suspend fun sendCommand(command: LocalCommand): Result<Unit> {
        return Result.failure(UnsupportedOperationException("WiFi-local command send is not implemented yet"))
    }
}
