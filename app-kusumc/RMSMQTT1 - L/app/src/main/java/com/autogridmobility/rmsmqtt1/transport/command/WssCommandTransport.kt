package com.autogridmobility.rmsmqtt1.transport.command

class WssCommandTransport : CommandTransport {
    override suspend fun sendCommand(request: CommandRequest, bearerToken: String): Result<CommandSendResult> {
        return Result.failure(UnsupportedOperationException("WSS command transport is not configured yet"))
    }

    override suspend fun getLatestStatus(deviceId: String, projectId: String, bearerToken: String): Result<CommandStatusSnapshot?> {
        return Result.failure(UnsupportedOperationException("WSS command status lookup is not configured yet"))
    }
}
