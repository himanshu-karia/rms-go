package com.autogridmobility.rmsmqtt1.transport.command

enum class CommandTransportType {
    HTTP,
    WSS
}

data class CommandRequest(
    val deviceId: String,
    val projectId: String,
    val commandId: String,
    val payload: Map<String, Any?>
)

data class CommandSendResult(
    val correlationId: String,
    val status: String,
    val transport: CommandTransportType
)

data class CommandStatusSnapshot(
    val correlationId: String,
    val status: String,
    val updatedAt: String? = null,
    val transport: CommandTransportType
)

interface CommandTransport {
    suspend fun sendCommand(request: CommandRequest, bearerToken: String): Result<CommandSendResult>
    suspend fun getLatestStatus(deviceId: String, projectId: String, bearerToken: String): Result<CommandStatusSnapshot?>
}
