package com.autogridmobility.rmsmqtt1.transport.command

class FallbackCommandTransport(
    private val primary: CommandTransport = HttpCommandTransport(),
    private val fallback: CommandTransport = WssCommandTransport()
) : CommandTransport {

    override suspend fun sendCommand(request: CommandRequest, bearerToken: String): Result<CommandSendResult> {
        return primary.sendCommand(request, bearerToken)
            .recoverCatching {
                fallback.sendCommand(request, bearerToken).getOrThrow()
            }
    }

    override suspend fun getLatestStatus(deviceId: String, projectId: String, bearerToken: String): Result<CommandStatusSnapshot?> {
        return primary.getLatestStatus(deviceId, projectId, bearerToken)
            .recoverCatching {
                fallback.getLatestStatus(deviceId, projectId, bearerToken).getOrThrow()
            }
    }
}
