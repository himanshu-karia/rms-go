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
import com.autogridmobility.rmsmqtt1.data.mobile.AdminAuthorityItem
import com.autogridmobility.rmsmqtt1.data.mobile.AdminStateItem
import com.autogridmobility.rmsmqtt1.viewmodel.AdminHierarchyCatalogViewModel

@Composable
fun AdminHierarchyScreen(viewModel: AdminHierarchyCatalogViewModel = viewModel()) {
    val states by viewModel.states.collectAsState()
    val authorities by viewModel.authorities.collectAsState()
    val loading by viewModel.isLoading.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()

    var selectedStateId by remember { mutableStateOf("") }
    var stateName by remember { mutableStateOf("") }
    var stateIso by remember { mutableStateOf("") }
    var editStateId by remember { mutableStateOf("") }

    var authorityName by remember { mutableStateOf("") }
    var authorityType by remember { mutableStateOf("nodal") }
    var editAuthorityId by remember { mutableStateOf("") }

    LaunchedEffect(Unit) { viewModel.loadHierarchy() }

    Column(modifier = Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text("Admin Hierarchy", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
        OutlinedTextField(value = selectedStateId, onValueChange = { selectedStateId = it }, label = { Text("Filter by State ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
        Button(onClick = { viewModel.loadHierarchy(selectedStateId.ifBlank { null }) }, modifier = Modifier.fillMaxWidth()) { Text(if (loading) "Loading..." else "Refresh Hierarchy") }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(if (editStateId.isBlank()) "Create State" else "Update State", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(value = stateName, onValueChange = { stateName = it }, label = { Text("State Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = stateIso, onValueChange = { stateIso = it }, label = { Text("ISO Code") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        if (editStateId.isBlank()) viewModel.createState(stateName, stateIso) else viewModel.updateState(editStateId, stateName, stateIso)
                    }, modifier = Modifier.weight(1f)) { Text(if (editStateId.isBlank()) "Create" else "Update") }
                    Button(onClick = {
                        editStateId = ""; stateName = ""; stateIso = ""; viewModel.clearMessages()
                    }, modifier = Modifier.weight(1f)) { Text("Clear") }
                }
            }
        }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(if (editAuthorityId.isBlank()) "Create Authority" else "Update Authority", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(value = authorityName, onValueChange = { authorityName = it }, label = { Text("Authority Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = authorityType, onValueChange = { authorityType = it }, label = { Text("Authority Type") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                Button(onClick = {
                    if (editAuthorityId.isBlank()) {
                        viewModel.createAuthority(selectedStateId, authorityName, authorityType)
                    } else {
                        viewModel.updateAuthority(editAuthorityId, authorityName, authorityType, selectedStateId.ifBlank { null })
                    }
                }, modifier = Modifier.fillMaxWidth()) { Text(if (editAuthorityId.isBlank()) "Create Authority" else "Update Authority") }
            }
        }

        if (!error.isNullOrBlank()) Text(error ?: "", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        if (!info.isNullOrBlank()) Text(info ?: "", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodySmall)

        LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            item {
                Text("States", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
            }
            items(states) { state ->
                StateCard(state,
                    onSelect = {
                        selectedStateId = state.id
                        editStateId = state.id
                        stateName = state.name
                        stateIso = state.isoCode ?: ""
                    },
                    onDelete = { viewModel.deleteState(state.id) }
                )
            }
            item {
                Text("Authorities", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
            }
            items(authorities) { authority ->
                AuthorityCard(authority,
                    onSelect = {
                        selectedStateId = authority.stateId
                        editAuthorityId = authority.id
                        authorityName = authority.name
                        authorityType = authority.type ?: "nodal"
                    },
                    onDelete = { viewModel.deleteAuthority(authority.id, selectedStateId.ifBlank { null }) }
                )
            }
        }
    }
}

@Composable
private fun StateCard(state: AdminStateItem, onSelect: () -> Unit, onDelete: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(state.name, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
            Text("ID: ${state.id} | ISO: ${state.isoCode ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onSelect, modifier = Modifier.weight(1f)) { Text("Select") }
                Button(onClick = onDelete, modifier = Modifier.weight(1f)) { Text("Delete") }
            }
        }
    }
}

@Composable
private fun AuthorityCard(item: AdminAuthorityItem, onSelect: () -> Unit, onDelete: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(item.name, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
            Text("ID: ${item.id} | State: ${item.stateId} | Type: ${item.type ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onSelect, modifier = Modifier.weight(1f)) { Text("Select") }
                Button(onClick = onDelete, modifier = Modifier.weight(1f)) { Text("Delete") }
            }
        }
    }
}
