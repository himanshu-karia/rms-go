package com.autogridmobility.rmsmqtt1.transport.local

enum class LocalTransportType {
    BLE,
    WIFI_LOCAL,
    MOCK
}

data class LocalTelemetryPacket(
    val source: LocalTransportType,
    val timestampMs: Long,
    val payload: String
)

data class LocalCommand(
    val command: String,
    val params: Map<String, String> = emptyMap()
)

interface LocalTransport {
    val type: LocalTransportType

    suspend fun connect(target: String): Result<Unit>
    suspend fun disconnect(): Result<Unit>
    suspend fun readPacket(): Result<LocalTelemetryPacket?>
    suspend fun sendCommand(command: LocalCommand): Result<Unit>
}
