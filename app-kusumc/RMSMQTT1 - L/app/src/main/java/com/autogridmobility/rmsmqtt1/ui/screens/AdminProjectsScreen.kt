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
import com.autogridmobility.rmsmqtt1.data.mobile.AdminProject
import com.autogridmobility.rmsmqtt1.viewmodel.AdminManagementViewModel

@Composable
fun AdminProjectsScreen(viewModel: AdminManagementViewModel = viewModel()) {
    val projects by viewModel.projects.collectAsState()
    val loading by viewModel.isLoading.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()

    var editMode by remember { mutableStateOf(false) }
    var id by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }
    var type by remember { mutableStateOf("rms") }
    var location by remember { mutableStateOf("") }
    var ownerOrg by remember { mutableStateOf("") }
    var configText by remember { mutableStateOf("{}") }

    LaunchedEffect(Unit) {
        viewModel.loadProjects()
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text("Admin Projects", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Text(if (editMode) "Edit Project" else "Create Project", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(value = id, onValueChange = { id = it }, label = { Text("Project ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true, enabled = !editMode)
                OutlinedTextField(value = name, onValueChange = { name = it }, label = { Text("Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(value = type, onValueChange = { type = it }, label = { Text("Type") }, modifier = Modifier.weight(1f), singleLine = true)
                    OutlinedTextField(value = location, onValueChange = { location = it }, label = { Text("Location") }, modifier = Modifier.weight(1f), singleLine = true)
                }
                OutlinedTextField(value = ownerOrg, onValueChange = { ownerOrg = it }, label = { Text("Owner Org ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = configText, onValueChange = { configText = it }, label = { Text("Config JSON") }, modifier = Modifier.fillMaxWidth(), minLines = 3)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        viewModel.createOrUpdateProject(id, name, type, location, ownerOrg, configText, editMode)
                    }, modifier = Modifier.weight(1f)) {
                        Text(if (editMode) "Update" else "Create")
                    }
                    Button(onClick = {
                        editMode = false
                        id = ""
                        name = ""
                        type = "rms"
                        location = ""
                        ownerOrg = ""
                        configText = "{}"
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

        Button(onClick = viewModel::loadProjects, modifier = Modifier.fillMaxWidth()) {
            Text(if (loading) "Loading..." else "Refresh Projects")
        }

        LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            items(projects) { project ->
                ProjectItem(project,
                    onEdit = {
                        editMode = true
                        id = project.id
                        name = project.name ?: ""
                        type = project.type ?: "rms"
                        location = project.location ?: ""
                        ownerOrg = project.ownerOrgId ?: ""
                        configText = project.config?.toString() ?: "{}"
                    },
                    onDelete = { viewModel.deleteProject(project.id) }
                )
            }
        }
    }
}

@Composable
private fun ProjectItem(project: AdminProject, onEdit: () -> Unit, onDelete: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(project.name ?: project.id, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
            Text("ID: ${project.id}", style = MaterialTheme.typography.bodySmall)
            Text("Type: ${project.type ?: "-"} | Location: ${project.location ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onEdit, modifier = Modifier.weight(1f)) { Text("Edit") }
                Button(onClick = onDelete, modifier = Modifier.weight(1f)) { Text("Delete") }
            }
        }
    }
}
