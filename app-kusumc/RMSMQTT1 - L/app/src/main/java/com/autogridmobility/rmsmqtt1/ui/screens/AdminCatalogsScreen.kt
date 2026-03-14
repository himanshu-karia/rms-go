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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.data.mobile.AdminProtocolVersionItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminVendorItem
import com.autogridmobility.rmsmqtt1.viewmodel.AdminHierarchyCatalogViewModel

@Composable
fun AdminCatalogsScreen(viewModel: AdminHierarchyCatalogViewModel = viewModel()) {
    val vendors by viewModel.serverVendors.collectAsState()
    val versions by viewModel.protocolVersions.collectAsState()
    val loading by viewModel.isLoading.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()

    var projectId by remember { mutableStateOf("") }
    var vendorName by remember { mutableStateOf("") }
    var selectedVendorId by remember { mutableStateOf("") }
    var vendorEndpoint by remember { mutableStateOf("server-vendors") }

    var protocolId by remember { mutableStateOf("") }
    var stateId by remember { mutableStateOf("") }
    var authorityId by remember { mutableStateOf("") }
    var protocolVersion by remember { mutableStateOf("") }
    var protocolName by remember { mutableStateOf("") }

    LaunchedEffect(Unit) { viewModel.clearMessages() }

    Column(modifier = Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text("Admin Catalogs", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
        OutlinedTextField(value = projectId, onValueChange = { projectId = it }, label = { Text("Project ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
        Button(onClick = { viewModel.loadCatalogs(projectId, vendorEndpoint) }, modifier = Modifier.fillMaxWidth()) { Text(if (loading) "Loading..." else "Load Catalog Data") }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Vendor Category", style = MaterialTheme.typography.titleMedium)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = { vendorEndpoint = "server-vendors"; selectedVendorId = ""; viewModel.loadCatalogs(projectId, vendorEndpoint) }, modifier = Modifier.weight(1f)) { Text("Server") }
                    Button(onClick = { vendorEndpoint = "solar-pump-vendors"; selectedVendorId = ""; viewModel.loadCatalogs(projectId, vendorEndpoint) }, modifier = Modifier.weight(1f)) { Text("Solar") }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = { vendorEndpoint = "vfd-drive-manufacturers"; selectedVendorId = ""; viewModel.loadCatalogs(projectId, vendorEndpoint) }, modifier = Modifier.weight(1f)) { Text("VFD") }
                    Button(onClick = { vendorEndpoint = "rms-manufacturers"; selectedVendorId = ""; viewModel.loadCatalogs(projectId, vendorEndpoint) }, modifier = Modifier.weight(1f)) { Text("RMS") }
                }
            }
        }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Server Vendors", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(value = vendorName, onValueChange = { vendorName = it }, label = { Text("Vendor Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        if (selectedVendorId.isBlank()) {
                            viewModel.createVendor(vendorName, vendorEndpoint, projectId)
                        } else {
                            viewModel.updateVendor(selectedVendorId, vendorName, vendorEndpoint, projectId)
                        }
                    }, modifier = Modifier.weight(1f)) { Text(if (selectedVendorId.isBlank()) "Create" else "Update") }
                    Button(onClick = {
                        selectedVendorId = ""
                        vendorName = ""
                        viewModel.clearMessages()
                    }, modifier = Modifier.weight(1f)) { Text("Clear") }
                }
            }
        }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Protocol Versions", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(value = protocolId, onValueChange = { protocolId = it }, label = { Text("Protocol Version ID (for update)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = stateId, onValueChange = { stateId = it }, label = { Text("State ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = authorityId, onValueChange = { authorityId = it }, label = { Text("Authority ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = protocolVersion, onValueChange = { protocolVersion = it }, label = { Text("Version") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = protocolName, onValueChange = { protocolName = it }, label = { Text("Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                Button(onClick = {
                    if (protocolId.isBlank()) {
                        viewModel.createProtocolVersion(stateId, authorityId, projectId, selectedVendorId, protocolVersion, protocolName)
                    } else {
                        viewModel.updateProtocolVersion(protocolId, protocolVersion, protocolName, selectedVendorId, projectId)
                    }
                }, modifier = Modifier.fillMaxWidth()) { Text(if (protocolId.isBlank()) "Create Protocol Version" else "Update Protocol Version") }
            }
        }

        if (!error.isNullOrBlank()) Text(error ?: "", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        if (!info.isNullOrBlank()) Text(info ?: "", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodySmall)

        LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            item { Text("Vendors", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold) }
            items(vendors) { vendor ->
                VendorCard(vendor,
                    onSelect = { selectedVendorId = vendor.id; vendorName = vendor.name },
                    onDelete = { viewModel.deleteVendor(vendor.id, vendorEndpoint, projectId) }
                )
            }
            item { Text("Protocol Versions", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold) }
            items(versions) { item ->
                ProtocolVersionCard(item, onSelect = {
                    protocolId = item.id
                    stateId = item.stateId
                    authorityId = item.authorityId
                    protocolVersion = item.version
                    protocolName = item.name ?: ""
                    selectedVendorId = item.serverVendorId
                })
            }
        }
    }
}

@Composable
private fun VendorCard(item: AdminVendorItem, onSelect: () -> Unit, onDelete: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(item.name, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
            Text("ID: ${item.id} | Category: ${item.category ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onSelect, modifier = Modifier.weight(1f)) { Text("Select") }
                Button(onClick = onDelete, modifier = Modifier.weight(1f)) { Text("Delete") }
            }
        }
    }
}

@Composable
private fun ProtocolVersionCard(item: AdminProtocolVersionItem, onSelect: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(item.name ?: item.version, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
            Text("ID: ${item.id}", style = MaterialTheme.typography.bodySmall)
            Text("Project: ${item.projectId} | State: ${item.stateId} | Authority: ${item.authorityId}", style = MaterialTheme.typography.bodySmall)
            Text("Server Vendor: ${item.serverVendorId} | Version: ${item.version}", style = MaterialTheme.typography.bodySmall)
            Button(onClick = onSelect, modifier = Modifier.fillMaxWidth()) { Text("Select") }
        }
    }
}
