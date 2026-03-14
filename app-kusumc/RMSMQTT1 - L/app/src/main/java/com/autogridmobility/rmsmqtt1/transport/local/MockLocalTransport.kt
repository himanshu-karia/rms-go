package com.autogridmobility.rmsmqtt1.transport.local

class MockLocalTransport : LocalTransport {
    override val type: LocalTransportType = LocalTransportType.MOCK

    private var connected = false
    private var lastCommand: LocalCommand? = null

    override suspend fun connect(target: String): Result<Unit> {
        connected = true
        return Result.success(Unit)
    }

    override suspend fun disconnect(): Result<Unit> {
        connected = false
        return Result.success(Unit)
    }

    override suspend fun readPacket(): Result<LocalTelemetryPacket?> {
        if (!connected) {
            return Result.failure(IllegalStateException("Mock transport not connected"))
        }
        val payload = buildString {
            append("{\"packet_type\":\"heartbeat\",\"source\":\"mock\"")
            if (lastCommand != null) {
                append(",\"last_command\":\"")
                append(lastCommand!!.command)
                append("\"")
            }
            append("}")
        }
        return Result.success(
            LocalTelemetryPacket(
                source = type,
                timestampMs = System.currentTimeMillis(),
                payload = payload
            )
        )
    }

    override suspend fun sendCommand(command: LocalCommand): Result<Unit> {
        if (!connected) {
            return Result.failure(IllegalStateException("Mock transport not connected"))
        }
        lastCommand = command
        return Result.success(Unit)
    }
}
