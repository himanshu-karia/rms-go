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
import com.autogridmobility.rmsmqtt1.data.mobile.ApiKeyRecord
import com.autogridmobility.rmsmqtt1.viewmodel.AdminManagementViewModel

@Composable
fun AdminApiKeysScreen(viewModel: AdminManagementViewModel = viewModel()) {
    val keys by viewModel.apiKeys.collectAsState()
    val loading by viewModel.isLoading.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()
    val createdSecret by viewModel.createdApiSecret.collectAsState()

    var name by remember { mutableStateOf("") }
    var scopes by remember { mutableStateOf("read:telemetry") }

    LaunchedEffect(Unit) {
        viewModel.loadApiKeys()
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text("Admin API Keys", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(value = name, onValueChange = { name = it }, label = { Text("Key Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = scopes, onValueChange = { scopes = it }, label = { Text("Scopes (comma separated)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = { viewModel.createApiKey(name, scopes) }, modifier = Modifier.weight(1f)) { Text("Create Key") }
                    Button(onClick = viewModel::loadApiKeys, modifier = Modifier.weight(1f)) { Text(if (loading) "Loading..." else "Refresh") }
                }
            }
        }

        if (!error.isNullOrBlank()) {
            Text(error ?: "", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        if (!info.isNullOrBlank()) {
            Text(info ?: "", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodySmall)
        }
        if (!createdSecret.isNullOrBlank()) {
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("New Secret (copy now)", style = MaterialTheme.typography.titleSmall)
                    Text(createdSecret ?: "", style = MaterialTheme.typography.bodySmall)
                    Button(onClick = viewModel::clearApiSecret, modifier = Modifier.fillMaxWidth()) { Text("I copied it") }
                }
            }
        }

        LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            items(keys) { key ->
                ApiKeyItem(key, onRevoke = { viewModel.revokeApiKey(key.id) })
            }
        }
    }
}

@Composable
private fun ApiKeyItem(key: ApiKeyRecord, onRevoke: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(key.name, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
            Text("Prefix: ${key.prefix ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Text("Scopes: ${if (key.scopes.isEmpty()) "-" else key.scopes.joinToString(",")}", style = MaterialTheme.typography.bodySmall)
            Text("Status: ${if (key.isActive) "active" else "revoked"}", style = MaterialTheme.typography.bodySmall)
            Button(onClick = onRevoke, modifier = Modifier.fillMaxWidth(), enabled = key.isActive) { Text("Revoke") }
        }
    }
}
