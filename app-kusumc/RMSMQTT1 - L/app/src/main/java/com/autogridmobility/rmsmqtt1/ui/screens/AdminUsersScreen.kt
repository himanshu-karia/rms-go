package com.autogridmobility.rmsmqtt1.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.ExposedDropdownMenuDefaults
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
import com.autogridmobility.rmsmqtt1.data.mobile.AdminUserSummary
import com.autogridmobility.rmsmqtt1.viewmodel.AdminManagementViewModel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AdminUsersScreen(viewModel: AdminManagementViewModel = viewModel()) {
    val users by viewModel.users.collectAsState()
    val loading by viewModel.isLoading.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()

    var selectedUserId by remember { mutableStateOf("") }
    var username by remember { mutableStateOf("") }
    var displayName by remember { mutableStateOf("") }
    var email by remember { mutableStateOf("") }
    var phone by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var status by remember { mutableStateOf("active") }
    var statusExpanded by remember { mutableStateOf(false) }
    var showCreatePanel by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        viewModel.loadUsers()
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text("Admin Users", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Text(if (selectedUserId.isBlank()) "Create User" else "Update Selected User", style = MaterialTheme.typography.titleMedium)
                    if (selectedUserId.isBlank()) {
                        Button(onClick = { showCreatePanel = !showCreatePanel }) {
                            Text(if (showCreatePanel) "Hide" else "New User")
                        }
                    }
                }

                if (showCreatePanel || selectedUserId.isNotBlank()) {
                    OutlinedTextField(value = username, onValueChange = { username = it }, label = { Text("Username") }, modifier = Modifier.fillMaxWidth(), singleLine = true, enabled = selectedUserId.isBlank())
                    OutlinedTextField(value = displayName, onValueChange = { displayName = it }, label = { Text("Display Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                    if (selectedUserId.isBlank()) {
                        OutlinedTextField(value = email, onValueChange = { email = it }, label = { Text("Email (optional)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                        OutlinedTextField(value = phone, onValueChange = { phone = it.filter { ch -> ch.isDigit() }.take(10) }, label = { Text("Phone (10 digits, optional)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                    }

                    ExposedDropdownMenuBox(expanded = statusExpanded, onExpandedChange = { statusExpanded = !statusExpanded }) {
                        OutlinedTextField(
                            value = status,
                            onValueChange = {},
                            readOnly = true,
                            label = { Text("Status") },
                            trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = statusExpanded) },
                            modifier = Modifier.menuAnchor().fillMaxWidth(),
                            singleLine = true
                        )
                        ExposedDropdownMenu(expanded = statusExpanded, onDismissRequest = { statusExpanded = false }) {
                            DropdownMenuItem(text = { Text("active") }, onClick = { status = "active"; statusExpanded = false })
                            DropdownMenuItem(text = { Text("disabled") }, onClick = { status = "disabled"; statusExpanded = false })
                        }
                    }

                    OutlinedTextField(value = password, onValueChange = { password = it }, label = { Text("Password (create/reset)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)

                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(onClick = {
                            if (selectedUserId.isBlank()) {
                                viewModel.createUser(username, displayName, email, phone, password, status, mustRotate = true)
                            } else {
                                viewModel.updateUser(selectedUserId, displayName, status, mustRotate = false)
                            }
                        }, modifier = Modifier.weight(1f)) {
                            Text(if (selectedUserId.isBlank()) "Create" else "Update")
                        }
                        Button(onClick = {
                            if (selectedUserId.isNotBlank() && password.isNotBlank()) {
                                viewModel.resetUserPassword(selectedUserId, password, requireChange = true)
                            }
                        }, modifier = Modifier.weight(1f)) {
                            Text("Reset Password")
                        }
                    }

                    Button(onClick = {
                        selectedUserId = ""
                        username = ""
                        displayName = ""
                        email = ""
                        phone = ""
                        password = ""
                        status = "active"
                        showCreatePanel = false
                        viewModel.clearMessages()
                    }, modifier = Modifier.fillMaxWidth()) { Text("Clear") }
                }
            }
        }

        if (!error.isNullOrBlank()) {
            Text(error ?: "", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        if (!info.isNullOrBlank()) {
            Text(info ?: "", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodySmall)
        }

        Button(onClick = viewModel::loadUsers, modifier = Modifier.fillMaxWidth()) {
            Text(if (loading) "Loading..." else "Refresh Users")
        }

        LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            items(users) { user ->
                UserItem(user,
                    onSelect = {
                        selectedUserId = user.id
                        showCreatePanel = true
                        username = user.username
                        displayName = user.displayName ?: ""
                        status = user.status ?: "active"
                    },
                    onDelete = { viewModel.deleteUser(user.id) }
                )
            }
        }
    }
}

@Composable
private fun UserItem(user: AdminUserSummary, onSelect: () -> Unit, onDelete: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(user.username, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
            Text("Display: ${user.displayName ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Text("Status: ${user.status ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onSelect, modifier = Modifier.weight(1f)) { Text("Select") }
                Button(onClick = onDelete, modifier = Modifier.weight(1f)) { Text("Delete") }
            }
        }
    }
}
