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
import com.autogridmobility.rmsmqtt1.data.mobile.AdminOrg
import com.autogridmobility.rmsmqtt1.viewmodel.AdminOrgGroupsViewModel

@Composable
fun AdminOrgsScreen(viewModel: AdminOrgGroupsViewModel = viewModel()) {
    val orgs by viewModel.orgs.collectAsState()
    val loading by viewModel.isLoading.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()

    var editMode by remember { mutableStateOf(false) }
    var id by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }
    var type by remember { mutableStateOf("tenant") }
    var path by remember { mutableStateOf("") }
    var parentId by remember { mutableStateOf("") }
    var metadata by remember { mutableStateOf("{}") }

    LaunchedEffect(Unit) { viewModel.loadOrgs() }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text("Admin Orgs", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Text(if (editMode) "Edit Org" else "Create Org", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(value = id, onValueChange = { id = it }, label = { Text("Org ID (for update)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = name, onValueChange = { name = it }, label = { Text("Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = type, onValueChange = { type = it }, label = { Text("Type") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = path, onValueChange = { path = it }, label = { Text("Path") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = parentId, onValueChange = { parentId = it }, label = { Text("Parent ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = metadata, onValueChange = { metadata = it }, label = { Text("Metadata JSON") }, modifier = Modifier.fillMaxWidth(), minLines = 3)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        viewModel.createOrUpdateOrg(id, name, type, path, parentId, metadata, editMode)
                    }, modifier = Modifier.weight(1f)) {
                        Text(if (editMode) "Update" else "Create")
                    }
                    Button(onClick = {
                        editMode = false
                        id = ""
                        name = ""
                        type = "tenant"
                        path = ""
                        parentId = ""
                        metadata = "{}"
                        viewModel.clearMessages()
                    }, modifier = Modifier.weight(1f)) {
                        Text("Clear")
                    }
                }
            }
        }

        if (!error.isNullOrBlank()) {
            Text(error ?: "", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        if (!info.isNullOrBlank()) {
            Text(info ?: "", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodySmall)
        }

        Button(onClick = viewModel::loadOrgs, modifier = Modifier.fillMaxWidth()) {
            Text(if (loading) "Loading..." else "Refresh Orgs")
        }

        LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            items(orgs) { org ->
                OrgItem(org,
                    onEdit = {
                        editMode = true
                        id = org.id
                        name = org.name
                        type = org.type
                        path = org.path ?: ""
                        parentId = org.parentId ?: ""
                        metadata = "{}"
                    }
                )
            }
        }
    }
}

@Composable
private fun OrgItem(item: AdminOrg, onEdit: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(item.name, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
            Text("ID: ${item.id}", style = MaterialTheme.typography.bodySmall)
            Text("Type: ${item.type} | Parent: ${item.parentId ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Text("Path: ${item.path ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Button(onClick = onEdit, modifier = Modifier.fillMaxWidth()) { Text("Edit") }
        }
    }
}
