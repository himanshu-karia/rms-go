package com.autogridmobility.rmsmqtt1.transport.local

class BleLocalTransport : LocalTransport {
    override val type: LocalTransportType = LocalTransportType.BLE

    override suspend fun connect(target: String): Result<Unit> {
        return Result.failure(UnsupportedOperationException("BLE local transport not implemented yet"))
    }

    override suspend fun disconnect(): Result<Unit> {
        return Result.success(Unit)
    }

    override suspend fun readPacket(): Result<LocalTelemetryPacket?> {
        return Result.failure(UnsupportedOperationException("BLE read is not implemented yet"))
    }

    override suspend fun sendCommand(command: LocalCommand): Result<Unit> {
        return Result.failure(UnsupportedOperationException("BLE command send is not implemented yet"))
    }
}
