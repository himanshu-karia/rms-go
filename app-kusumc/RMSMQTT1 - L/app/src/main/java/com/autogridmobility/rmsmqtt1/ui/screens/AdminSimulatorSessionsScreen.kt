package com.autogridmobility.rmsmqtt1.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.viewmodel.AdminOpsViewModel

@Composable
fun AdminSimulatorSessionsScreen(viewModel: AdminOpsViewModel = viewModel()) {
    val deviceUuid by viewModel.sessionDeviceUuid.collectAsState()
    val durationMins by viewModel.sessionDurationMins.collectAsState()
    val sessions by viewModel.simulatorSessions.collectAsState()
    val loading by viewModel.isLoadingSessions.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()
    val simImei by viewModel.simImei.collectAsState()
    val simTopic by viewModel.simTopic.collectAsState()
    val simClientId by viewModel.simClientId.collectAsState()
    val simUsername by viewModel.simUsername.collectAsState()
    val simPassword by viewModel.simPassword.collectAsState()
    val simPayload by viewModel.simPayload.collectAsState()
    val simHttpResult by viewModel.simHttpResult.collectAsState()
    val bootstrapping by viewModel.isBootstrapping.collectAsState()
    val bootstrapProjects by viewModel.bootstrapProjects.collectAsState()
    val bootstrapStates by viewModel.bootstrapStates.collectAsState()
    val bootstrapAuthorities by viewModel.bootstrapAuthorities.collectAsState()
    val bootstrapVendors by viewModel.bootstrapVendors.collectAsState()
    val bootstrapProtocols by viewModel.bootstrapProtocols.collectAsState()

    LaunchedEffect(Unit) {
        viewModel.loadSimulatorSessions()
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "Simulator Sessions",
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.SemiBold
        )

        Button(onClick = viewModel::bootstrapSimulatorContext, modifier = Modifier.fillMaxWidth()) {
            Text(if (bootstrapping) "Bootstrapping..." else "Bootstrap Simulator Context")
        }

        if (bootstrapProjects.isNotEmpty() || bootstrapStates.isNotEmpty() || bootstrapAuthorities.isNotEmpty() || bootstrapVendors.isNotEmpty() || bootstrapProtocols.isNotEmpty()) {
            Text(
                text = "Context: projects=${bootstrapProjects.size}, states=${bootstrapStates.size}, authorities=${bootstrapAuthorities.size}, vendors=${bootstrapVendors.size}, protocols=${bootstrapProtocols.size}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }

        OutlinedTextField(
            value = deviceUuid,
            onValueChange = {
                viewModel.clearError()
                viewModel.updateSessionDeviceUuid(it)
            },
            label = { Text("Device UUID") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )

        OutlinedTextField(
            value = durationMins,
            onValueChange = {
                viewModel.clearError()
                viewModel.updateSessionDurationMins(it)
            },
            label = { Text("Expires in minutes") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )

        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = viewModel::createSimulatorSession, modifier = Modifier.weight(1f)) {
                Text(if (loading) "Working..." else "Create Session")
            }
            Button(onClick = viewModel::loadSimulatorSessions, modifier = Modifier.weight(1f)) {
                Text("Refresh")
            }
        }

        if (!error.isNullOrBlank()) {
            Text(
                text = error ?: "",
                color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.bodySmall
            )
        }

        if (!info.isNullOrBlank()) {
            Text(
                text = info ?: "",
                color = MaterialTheme.colorScheme.primary,
                style = MaterialTheme.typography.bodySmall
            )
        }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Text(
                    text = "Device-open + HTTPS Ingest",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )

                OutlinedTextField(
                    value = simImei,
                    onValueChange = {
                        viewModel.clearError()
                        viewModel.clearInfo()
                        viewModel.updateSimImei(it)
                    },
                    label = { Text("IMEI") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )

                Button(onClick = viewModel::fetchDeviceOpenCredentials, modifier = Modifier.fillMaxWidth()) {
                    Text("Load Device-open Credentials")
                }

                OutlinedTextField(
                    value = simTopic,
                    onValueChange = viewModel::updateSimTopic,
                    label = { Text("Telemetry Topic") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )

                OutlinedTextField(
                    value = simClientId,
                    onValueChange = viewModel::updateSimClientId,
                    label = { Text("Client ID") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )

                OutlinedTextField(
                    value = simUsername,
                    onValueChange = viewModel::updateSimUsername,
                    label = { Text("Username") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )

                OutlinedTextField(
                    value = simPassword,
                    onValueChange = viewModel::updateSimPassword,
                    label = { Text("Password") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )

                OutlinedTextField(
                    value = simPayload,
                    onValueChange = viewModel::updateSimPayload,
                    label = { Text("Payload JSON") },
                    modifier = Modifier.fillMaxWidth(),
                    minLines = 4
                )

                Button(onClick = viewModel::sendHttpsIngest, modifier = Modifier.fillMaxWidth()) {
                    Text("Send HTTPS Ingest")
                }

                if (!simHttpResult.isNullOrBlank()) {
                    Text(simHttpResult ?: "", style = MaterialTheme.typography.bodySmall)
                }
            }
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            if (sessions.isEmpty() && !loading) {
                item {
                    Text(
                        text = "No simulator sessions available.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            items(sessions) { session ->
                Card(modifier = Modifier.fillMaxWidth()) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(12.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        Text("Session: ${session.sessionId}", style = MaterialTheme.typography.titleSmall)
                        Text("Device: ${session.deviceUuid}", style = MaterialTheme.typography.bodySmall)
                        Text("Status: ${session.status}", style = MaterialTheme.typography.bodySmall)
                        Text("Expires: ${session.expiresAt ?: "-"}", style = MaterialTheme.typography.bodySmall)
                        if (session.status == "active") {
                            Button(
                                onClick = { viewModel.revokeSimulatorSession(session.sessionId) },
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Text("Revoke")
                            }
                        }
                    }
                }
            }
        }
    }
}
