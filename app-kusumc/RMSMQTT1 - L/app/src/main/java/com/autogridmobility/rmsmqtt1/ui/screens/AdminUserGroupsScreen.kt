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
import com.autogridmobility.rmsmqtt1.data.mobile.AdminUserGroup
import com.autogridmobility.rmsmqtt1.data.mobile.UserGroupMember
import com.autogridmobility.rmsmqtt1.viewmodel.AdminOrgGroupsViewModel

@Composable
fun AdminUserGroupsScreen(viewModel: AdminOrgGroupsViewModel = viewModel()) {
    val groups by viewModel.groups.collectAsState()
    val members by viewModel.members.collectAsState()
    val selectedGroupId by viewModel.selectedGroupId.collectAsState()
    val loading by viewModel.isLoading.collectAsState()
    val error by viewModel.error.collectAsState()
    val info by viewModel.info.collectAsState()

    var groupId by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }
    var description by remember { mutableStateOf("") }
    var stateId by remember { mutableStateOf("") }
    var authorityId by remember { mutableStateOf("") }
    var projectId by remember { mutableStateOf("") }
    var defaultRoleIds by remember { mutableStateOf("") }
    var memberUserId by remember { mutableStateOf("") }

    LaunchedEffect(Unit) { viewModel.loadUserGroups() }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text("Admin User Groups", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Text(if (groupId.isBlank()) "Create Group" else "Update Group", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(value = groupId, onValueChange = { groupId = it }, label = { Text("Group ID (for update)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = name, onValueChange = { name = it }, label = { Text("Name") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = description, onValueChange = { description = it }, label = { Text("Description") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = defaultRoleIds, onValueChange = { defaultRoleIds = it }, label = { Text("Default Role IDs (CSV)") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = stateId, onValueChange = { stateId = it }, label = { Text("Scope State ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = authorityId, onValueChange = { authorityId = it }, label = { Text("Scope Authority ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                OutlinedTextField(value = projectId, onValueChange = { projectId = it }, label = { Text("Scope Project ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)

                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        if (groupId.isBlank()) {
                            viewModel.createUserGroup(name, description, stateId, authorityId, projectId, defaultRoleIds)
                        } else {
                            viewModel.updateUserGroup(groupId, name, description, defaultRoleIds)
                        }
                    }, modifier = Modifier.weight(1f)) {
                        Text(if (groupId.isBlank()) "Create" else "Update")
                    }
                    Button(onClick = {
                        groupId = ""
                        name = ""
                        description = ""
                        stateId = ""
                        authorityId = ""
                        projectId = ""
                        defaultRoleIds = ""
                        viewModel.clearMessages()
                    }, modifier = Modifier.weight(1f)) {
                        Text("Clear")
                    }
                }
            }
        }

        Card(modifier = Modifier.fillMaxWidth()) {
            Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Group Members", style = MaterialTheme.typography.titleMedium)
                Text("Selected Group: ${selectedGroupId ?: "-"}", style = MaterialTheme.typography.bodySmall)
                OutlinedTextField(value = memberUserId, onValueChange = { memberUserId = it }, label = { Text("User ID") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        val selected = selectedGroupId ?: groupId
                        viewModel.addMember(selected, memberUserId)
                    }, modifier = Modifier.weight(1f)) { Text("Add Member") }
                    Button(onClick = {
                        val selected = selectedGroupId ?: groupId
                        viewModel.loadMembers(selected)
                    }, modifier = Modifier.weight(1f)) { Text("Refresh Members") }
                }
            }
        }

        if (!error.isNullOrBlank()) {
            Text(error ?: "", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        if (!info.isNullOrBlank()) {
            Text(info ?: "", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodySmall)
        }

        Button(onClick = viewModel::loadUserGroups, modifier = Modifier.fillMaxWidth()) {
            Text(if (loading) "Loading..." else "Refresh Groups")
        }

        LazyColumn(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            item { Text("Groups", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold) }
            items(groups) { group ->
                UserGroupItem(
                    item = group,
                    onSelect = {
                        groupId = group.id
                        name = group.name
                        description = group.description ?: ""
                        stateId = group.scope?.stateId ?: ""
                        authorityId = group.scope?.authorityId ?: ""
                        projectId = group.scope?.projectId ?: ""
                        defaultRoleIds = group.defaultRoleIds.joinToString(",")
                        viewModel.loadMembers(group.id)
                    },
                    onDelete = { viewModel.deleteUserGroup(group.id) }
                )
            }
            item { Text("Members", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold) }
            items(members) { member ->
                UserGroupMemberItem(
                    item = member,
                    onRemove = {
                        val selected = selectedGroupId ?: return@UserGroupMemberItem
                        viewModel.removeMember(selected, member.userId)
                    }
                )
            }
        }
    }
}

@Composable
private fun UserGroupItem(item: AdminUserGroup, onSelect: () -> Unit, onDelete: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(item.name, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
            Text("ID: ${item.id}", style = MaterialTheme.typography.bodySmall)
            Text("Scope: state=${item.scope?.stateId ?: "-"}, authority=${item.scope?.authorityId ?: "-"}, project=${item.scope?.projectId ?: "-"}", style = MaterialTheme.typography.bodySmall)
            Text("Default Roles: ${item.defaultRoleIds.joinToString(",").ifBlank { "-" }}", style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onSelect, modifier = Modifier.weight(1f)) { Text("Select") }
                Button(onClick = onDelete, modifier = Modifier.weight(1f)) { Text("Delete") }
            }
        }
    }
}

@Composable
private fun UserGroupMemberItem(item: UserGroupMember, onRemove: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(item.username ?: item.userId, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
            Text("User ID: ${item.userId}", style = MaterialTheme.typography.bodySmall)
            Text("Group ID: ${item.groupId}", style = MaterialTheme.typography.bodySmall)
            Button(onClick = onRemove, modifier = Modifier.fillMaxWidth()) { Text("Remove") }
        }
    }
}
