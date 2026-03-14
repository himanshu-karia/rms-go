package com.autogridmobility.rmsmqtt1.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.data.mobile.CommandCatalogItem
import com.autogridmobility.rmsmqtt1.viewmodel.AdminOpsViewModel

@Composable
fun AdminCommandCatalogScreen(viewModel: AdminOpsViewModel = viewModel()) {
    val projectId by viewModel.projectId.collectAsState()
    val deviceId by viewModel.deviceId.collectAsState()
    val items by viewModel.commandItems.collectAsState()
    val loading by viewModel.isLoadingCatalog.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()

    var editId by remember { mutableStateOf("") }
    var commandName by remember { mutableStateOf("") }
    var scope by remember { mutableStateOf("project") }
    var transport by remember { mutableStateOf("mqtt") }
    var protocolId by remember { mutableStateOf("") }
    var modelId by remember { mutableStateOf("") }
    var payloadSchema by remember { mutableStateOf("{\n  \"type\": \"object\",\n  \"properties\": {}\n}") }
    var deviceIdsRaw by remember { mutableStateOf("") }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "Admin Command Catalog",
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.SemiBold
        )

        OutlinedTextField(
            value = projectId,
            onValueChange = {
                viewModel.clearError()
                viewModel.updateProjectId(it)
            },
            label = { Text("Project ID") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )

        OutlinedTextField(
            value = deviceId,
            onValueChange = {
                viewModel.clearError()
                viewModel.updateDeviceId(it)
            },
            label = { Text("Device ID or IMEI") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )

        Button(
            onClick = viewModel::loadCommandCatalog,
            modifier = Modifier.fillMaxWidth()
        ) {
            Text(if (loading) "Loading..." else "Load Catalog")
        }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Text(
                    text = if (editId.isBlank()) "Create Command" else "Edit Command",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )

                OutlinedTextField(
                    value = commandName,
                    onValueChange = {
                        viewModel.clearError()
                        viewModel.clearInfo()
                        commandName = it
                    },
                    label = { Text("Name") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )

                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(
                        value = scope,
                        onValueChange = {
                            viewModel.clearError()
                            viewModel.clearInfo()
                            scope = it
                        },
                        label = { Text("Scope") },
                        modifier = Modifier.weight(1f),
                        singleLine = true
                    )
                    OutlinedTextField(
                        value = transport,
                        onValueChange = {
                            viewModel.clearError()
                            viewModel.clearInfo()
                            transport = it
                        },
                        label = { Text("Transport") },
                        modifier = Modifier.weight(1f),
                        singleLine = true
                    )
                }

                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(
                        value = protocolId,
                        onValueChange = {
                            viewModel.clearError()
                            viewModel.clearInfo()
                            protocolId = it
                        },
                        label = { Text("Protocol ID") },
                        modifier = Modifier.weight(1f),
                        singleLine = true
                    )
                    OutlinedTextField(
                        value = modelId,
                        onValueChange = {
                            viewModel.clearError()
                            viewModel.clearInfo()
                            modelId = it
                        },
                        label = { Text("Model ID") },
                        modifier = Modifier.weight(1f),
                        singleLine = true
                    )
                }

                OutlinedTextField(
                    value = deviceIdsRaw,
                    onValueChange = {
                        viewModel.clearError()
                        viewModel.clearInfo()
                        deviceIdsRaw = it
                    },
                    label = { Text("Device IDs (comma separated)") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )

                OutlinedTextField(
                    value = payloadSchema,
                    onValueChange = {
                        viewModel.clearError()
                        viewModel.clearInfo()
                        payloadSchema = it
                    },
                    label = { Text("Payload Schema JSON") },
                    modifier = Modifier.fillMaxWidth(),
                    minLines = 4
                )

                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(
                        onClick = {
                            viewModel.upsertCommandCatalog(
                                id = editId.ifBlank { null },
                                name = commandName,
                                scope = scope,
                                transport = transport,
                                protocolId = protocolId,
                                modelId = modelId,
                                payloadSchemaText = payloadSchema,
                                deviceIdsRaw = deviceIdsRaw
                            )
                        },
                        modifier = Modifier.weight(1f)
                    ) {
                        Text(if (editId.isBlank()) "Create" else "Update")
                    }
                    Button(
                        onClick = {
                            editId = ""
                            commandName = ""
                            scope = "project"
                            transport = "mqtt"
                            protocolId = ""
                            modelId = ""
                            payloadSchema = "{\n  \"type\": \"object\",\n  \"properties\": {}\n}"
                            deviceIdsRaw = ""
                            viewModel.clearError()
                            viewModel.clearInfo()
                        },
                        modifier = Modifier.weight(1f)
                    ) {
                        Text("Clear")
                    }
                }
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

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            if (items.isEmpty() && !loading) {
                item {
                    Text(
                        text = "No command definitions found for current scope.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            items(items) { item ->
                Card(modifier = Modifier.fillMaxWidth()) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(12.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        Text(item.name, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
                        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("Scope: ${item.scope}", style = MaterialTheme.typography.bodySmall)
                            Text("Transport: ${item.transport ?: "mqtt"}", style = MaterialTheme.typography.bodySmall)
                        }
                        Text("Project: ${item.projectId ?: "-"}", style = MaterialTheme.typography.bodySmall)
                        Text("Created: ${item.createdAt ?: "-"}", style = MaterialTheme.typography.bodySmall)

                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Button(
                                onClick = {
                                    populateEditor(item) { id, name, scopeValue, transportValue, protocol, model, payload ->
                                        editId = id
                                        commandName = name
                                        scope = scopeValue
                                        transport = transportValue
                                        protocolId = protocol
                                        modelId = model
                                        payloadSchema = payload
                                        deviceIdsRaw = ""
                                    }
                                },
                                modifier = Modifier.weight(1f)
                            ) {
                                Text("Edit")
                            }
                            Button(
                                onClick = { viewModel.deleteCommandCatalog(item.id) },
                                modifier = Modifier.weight(1f)
                            ) {
                                Text("Delete")
                            }
                        }
                    }
                }
            }
        }
    }
}

private fun populateEditor(
    item: CommandCatalogItem,
    apply: (
        id: String,
        name: String,
        scope: String,
        transport: String,
        protocolId: String,
        modelId: String,
        payloadSchema: String
    ) -> Unit
) {
    apply(
        item.id,
        item.name,
        item.scope,
        item.transport ?: "mqtt",
        item.protocolId ?: "",
        item.modelId ?: "",
        item.payloadSchema?.toString() ?: "{\n  \"type\": \"object\",\n  \"properties\": {}\n}"
    )
}
