package com.autogridmobility.rmsmqtt1.ui.screens

import android.widget.Toast
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.service.MqttConnectionState
import com.autogridmobility.rmsmqtt1.ui.components.ConnectionStatusIndicator
import com.autogridmobility.rmsmqtt1.ui.theme.DataBlue
import com.autogridmobility.rmsmqtt1.ui.theme.DataTeal
import com.autogridmobility.rmsmqtt1.ui.theme.ErrorRed
import com.autogridmobility.rmsmqtt1.ui.theme.SuccessGreen
import com.autogridmobility.rmsmqtt1.ui.theme.WarningYellow
import com.autogridmobility.rmsmqtt1.utils.CsvExportUtil
import com.autogridmobility.rmsmqtt1.viewmodel.SettingsViewModel
import kotlinx.coroutines.launch

@Composable
fun SettingsScreenWithExport(
    settingsViewModel: SettingsViewModel = viewModel(),
    rawDataViewModel: com.autogridmobility.rmsmqtt1.viewmodel.RawDataViewModel = viewModel()
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var isExporting by remember { mutableStateOf(false) }
    var passwordVisible by remember { mutableStateOf(false) }
    
    // MQTT Configuration state
    val url by settingsViewModel.url.collectAsState()
    val port by settingsViewModel.port.collectAsState()
    val username by settingsViewModel.username.collectAsState()
    val password by settingsViewModel.password.collectAsState()
    val clientId by settingsViewModel.clientId.collectAsState()
    val topicPrefix by settingsViewModel.topicPrefix.collectAsState()
    val fieldsEditable by settingsViewModel.fieldsEditable.collectAsState()
    val urlError by settingsViewModel.urlError.collectAsState()
    val portError by settingsViewModel.portError.collectAsState()
    val clientIdError by settingsViewModel.clientIdError.collectAsState()
    val topicPrefixError by settingsViewModel.topicPrefixError.collectAsState()
    val connectionStatus by settingsViewModel.connectionStatus.collectAsState()
    val currentState by settingsViewModel.currentState.collectAsState()
    val buttonConfig by settingsViewModel.buttonConfig.collectAsState()
    val isConnecting by settingsViewModel.isConnecting.collectAsState()
    val subscribedTopics by settingsViewModel.subscribedTopics.collectAsState()
    val errorMessage by settingsViewModel.errorMessage.collectAsState()
    
    // Simulation data
    val sendingInterval by settingsViewModel.sendingInterval.collectAsState()
    val isSimulating by settingsViewModel.isSimulating.collectAsState()
    val packetsPublished by settingsViewModel.packetsPublished.collectAsState()
    
    // Raw data for export
    val heartbeatHistory by rawDataViewModel.heartbeatHistory.collectAsState()
    val pumpHistory by rawDataViewModel.pumpHistory.collectAsState()
    val daqHistory by rawDataViewModel.daqHistory.collectAsState()
    
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp)
            .verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // Title
        Text(
            text = "Settings & Export",
            style = MaterialTheme.typography.headlineSmall,
            color = DataBlue,
            fontWeight = FontWeight.Medium
        )
        
        // Connection Settings Card - Use the original SettingsScreen content
        ConnectionSettingsCard(
            url = url,
            port = port,
            username = username,
            password = password,
            clientId = clientId,
            topicPrefix = topicPrefix,
            fieldsEditable = fieldsEditable,
            urlError = urlError,
            portError = portError,
            clientIdError = clientIdError,
            topicPrefixError = topicPrefixError,
            connectionStatus = connectionStatus,
            currentState = currentState,
            buttonConfig = buttonConfig,
            isConnecting = isConnecting,
            subscribedTopics = subscribedTopics,
            passwordVisible = passwordVisible,
            onPasswordVisibilityToggle = { passwordVisible = !passwordVisible },
            onUpdateUrl = settingsViewModel::updateUrl,
            onUpdatePort = settingsViewModel::updatePort,
            onUpdateUsername = settingsViewModel::updateUsername,
            onUpdatePassword = settingsViewModel::updatePassword,
            onUpdateClientId = settingsViewModel::updateClientId,
            onUpdateTopicPrefix = settingsViewModel::updateTopicPrefix,
            onGenerateClientId = settingsViewModel::generateNewClientId,
            onConnect = settingsViewModel::connect,
            onDisconnect = settingsViewModel::disconnect
        )
        
        // Data Simulation Card - Always visible for testability
        DataSimulationCard(
            sendingInterval = sendingInterval,
            isSimulating = isSimulating,
            packetsPublished = packetsPublished,
            isConnected = connectionStatus == "Active",
            onUpdateInterval = settingsViewModel::updateSendingInterval,
            onStartSimulation = settingsViewModel::startDataSimulation,
            onStopSimulation = settingsViewModel::stopDataSimulation
        )
        
        // Data Export Card
        Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            colors = CardDefaults.cardColors(
                containerColor = MaterialTheme.colorScheme.surface
            ),
            elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
        ) {
            Column(
                modifier = Modifier.padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Text(
                    text = "Data Export",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )
                
                Text(
                    text = "Export collected data as CSV files",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                
                // Data count info
                Column(
                    verticalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Text(
                        text = "Available Data:",
                        style = MaterialTheme.typography.labelMedium,
                        fontWeight = FontWeight.Bold
                    )
                    Text(
                        text = "• Heartbeat records: ${heartbeatHistory.size}",
                        style = MaterialTheme.typography.bodySmall
                    )
                    Text(
                        text = "• Pump data records: ${pumpHistory.size}",
                        style = MaterialTheme.typography.bodySmall
                    )
                    Text(
                        text = "• DAQ records: ${daqHistory.size}",
                        style = MaterialTheme.typography.bodySmall
                    )
                }
                
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Button(
                        onClick = {
                            scope.launch {
                                isExporting = true
                                try {
                                    val result = CsvExportUtil.exportDataToCsv(
                                        context = context,
                                        heartbeatHistory = heartbeatHistory,
                                        pumpHistory = pumpHistory,
                                        daqHistory = daqHistory,
                                        onDemandHistory = emptyList() // Can be added later
                                    )
                                    
                                    result.fold(
                                        onSuccess = { files ->
                                            if (files.isNotEmpty()) {
                                                CsvExportUtil.shareExportedFiles(context, files)
                                                Toast.makeText(
                                                    context,
                                                    "Exported ${files.size} CSV file(s)",
                                                    Toast.LENGTH_SHORT
                                                ).show()
                                            } else {
                                                Toast.makeText(
                                                    context,
                                                    "No data to export",
                                                    Toast.LENGTH_SHORT
                                                ).show()
                                            }
                                        },
                                        onFailure = { error ->
                                            Toast.makeText(
                                                context,
                                                "Export failed: ${error.message}",
                                                Toast.LENGTH_LONG
                                            ).show()
                                        }
                                    )
                                } finally {
                                    isExporting = false
                                }
                            }
                        },
                        modifier = Modifier.weight(1f),
                        enabled = !isExporting && (heartbeatHistory.isNotEmpty() || pumpHistory.isNotEmpty() || daqHistory.isNotEmpty()),
                        shape = RoundedCornerShape(12.dp),
                        colors = ButtonDefaults.buttonColors(
                            containerColor = DataTeal
                        )
                    ) {
                        Text(
                            text = if (isExporting) "Exporting..." else "Export All Data",
                            fontWeight = FontWeight.Bold,
                            color = Color.White
                        )
                    }
                    
                    OutlinedButton(
                        onClick = {
                            settingsViewModel.clearAllData()
                            Toast.makeText(
                                context, 
                                "All data cleared successfully", 
                                Toast.LENGTH_SHORT
                            ).show()
                        },
                        modifier = Modifier.weight(1f),
                        enabled = !isExporting && (heartbeatHistory.isNotEmpty() || pumpHistory.isNotEmpty() || daqHistory.isNotEmpty()),
                        shape = RoundedCornerShape(12.dp)
                    ) {
                        Text(
                            text = "Clear Data",
                            fontWeight = FontWeight.Bold
                        )
                    }
                }
            }
        }
        
        Spacer(modifier = Modifier.height(16.dp))
        
        // Info text
        Text(
            text = "CSV files will be saved to your device and can be shared via email, cloud storage, or other apps.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun ConnectionSettingsCard(
    url: String,
    port: String,
    username: String,
    password: String,
    clientId: String,
    topicPrefix: String,
    fieldsEditable: Boolean,
    urlError: String?,
    portError: String?,
    clientIdError: String?,
    topicPrefixError: String?,
    connectionStatus: String,
    currentState: MqttConnectionState,
    buttonConfig: com.autogridmobility.rmsmqtt1.service.ButtonConfig,
    isConnecting: Boolean,
    subscribedTopics: List<String>,
    passwordVisible: Boolean,
    onPasswordVisibilityToggle: () -> Unit,
    onUpdateUrl: (String) -> Unit,
    onUpdatePort: (String) -> Unit,
    onUpdateUsername: (String) -> Unit,
    onUpdatePassword: (String) -> Unit,
    onUpdateClientId: (String) -> Unit,
    onUpdateTopicPrefix: (String) -> Unit,
    onGenerateClientId: () -> Unit,
    onConnect: () -> Unit,
    onDisconnect: () -> Unit
) {
    Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
        // Connection Status Card
        Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            colors = CardDefaults.cardColors(
                containerColor = MaterialTheme.colorScheme.surface
            ),
            elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
        ) {
            Column(
                modifier = Modifier.padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Text(
                    text = "Connection Status",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )
                
                ConnectionStatusIndicator(
                    status = connectionStatus,
                    modifier = Modifier.clickable {
                        // Refresh status could be added here
                    }
                )
            }
        }
        
        // MQTT Configuration Card
        Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            colors = CardDefaults.cardColors(
                containerColor = MaterialTheme.colorScheme.surface
            ),
            elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
        ) {
            Column(
                modifier = Modifier.padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Text(
                    text = "MQTT Broker Configuration",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )
                
                // Broker URL
                OutlinedTextField(
                    value = url,
                    onValueChange = { if (fieldsEditable) onUpdateUrl(it) },
                    label = { Text("Broker URL") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEditable,
                    isError = urlError != null,
                    supportingText = urlError?.let { { Text(it, color = ErrorRed) } },
                    singleLine = true
                )
                
                // Port Number
                OutlinedTextField(
                    value = port,
                    onValueChange = { if (fieldsEditable) onUpdatePort(it) },
                    label = { Text("Port") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEditable,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    isError = portError != null,
                    supportingText = portError?.let { { Text(it, color = ErrorRed) } },
                    singleLine = true
                )
                
                // Username
                OutlinedTextField(
                    value = username,
                    onValueChange = { if (fieldsEditable) onUpdateUsername(it) },
                    label = { Text("Username (optional)") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEditable,
                    singleLine = true
                )
                
                // Password
                OutlinedTextField(
                    value = password,
                    onValueChange = { if (fieldsEditable) onUpdatePassword(it) },
                    label = { Text("Password (optional)") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEditable,
                    visualTransformation = if (passwordVisible) VisualTransformation.None else PasswordVisualTransformation(),
                    trailingIcon = {
                        IconButton(
                            onClick = onPasswordVisibilityToggle,
                            enabled = fieldsEditable
                        ) {
                            Icon(
                                imageVector = if (passwordVisible) Icons.Filled.Visibility else Icons.Filled.VisibilityOff,
                                contentDescription = if (passwordVisible) "Hide password" else "Show password"
                            )
                        }
                    },
                    singleLine = true
                )
                
                // Client ID with Generate button
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.Top
                ) {
                    OutlinedTextField(
                        value = clientId,
                        onValueChange = { if (fieldsEditable) onUpdateClientId(it) },
                        label = { Text("Client ID") },
                        modifier = Modifier.weight(1f),
                        enabled = fieldsEditable,
                        isError = clientIdError != null,
                        supportingText = clientIdError?.let { { Text(it, color = ErrorRed) } },
                        singleLine = true
                    )
                    
                    Button(
                        onClick = onGenerateClientId,
                        enabled = fieldsEditable,
                        modifier = Modifier.padding(top = 8.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = DataBlue)
                    ) {
                        Icon(
                            imageVector = Icons.Filled.Refresh,
                            contentDescription = "Generate new Client ID",
                            modifier = Modifier.padding(end = 4.dp)
                        )
                        Text("Generate")
                    }
                }
                
                // Topic Prefix (IMEI)
                OutlinedTextField(
                    value = topicPrefix,
                    onValueChange = { if (fieldsEditable) onUpdateTopicPrefix(it) },
                    label = { Text("Topic Prefix (IMEI)") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEditable,
                    isError = topicPrefixError != null,
                    supportingText = topicPrefixError?.let { { Text(it, color = ErrorRed) } },
                    singleLine = true
                )
                
                // Settings lock status indicator
                if (!fieldsEditable) {
                    Text(
                        text = "⚠️ Settings are locked while connected. Disconnect to modify.",
                        style = MaterialTheme.typography.bodySmall,
                        color = WarningYellow,
                        modifier = Modifier.padding(top = 8.dp)
                    )
                }
            }
        }
        
        // Connection Control Button
        Button(
            onClick = {
                when (currentState) {
                    MqttConnectionState.DISCONNECTED,
                    MqttConnectionState.ERROR,
                    MqttConnectionState.NETWORK_LOST -> onConnect()
                    MqttConnectionState.CONNECTED -> onDisconnect()
                    else -> {
                        // Do nothing for transitional states (CONNECTING, DISCONNECTING)
                    }
                }
            },
            modifier = Modifier.fillMaxWidth(),
            enabled = buttonConfig.enabled,
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(
                containerColor = when (currentState) {
                    MqttConnectionState.CONNECTED -> ErrorRed
                    MqttConnectionState.ERROR -> WarningYellow
                    else -> SuccessGreen
                },
                disabledContainerColor = DataBlue.copy(alpha = 0.5f)
            )
        ) {
            Text(
                text = buttonConfig.text,
                fontWeight = FontWeight.Bold,
                color = Color.White,
                modifier = Modifier.padding(8.dp)
            )
        }
        
        // Subscribed Topics Card
        if (subscribedTopics.isNotEmpty()) {
            Card(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(12.dp),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surface
                ),
                elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
            ) {
                Column(
                    modifier = Modifier.padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Text(
                        text = "Subscribed Topics",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold
                    )
                    
                    subscribedTopics.forEach { topic ->
                        Text(
                            text = "• $topic",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(start = 8.dp)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun DataSimulationCard(
    sendingInterval: String,
    isSimulating: Boolean,
    packetsPublished: Int,
    isConnected: Boolean,
    onUpdateInterval: (String) -> Unit,
    onStartSimulation: () -> Unit,
    onStopSimulation: () -> Unit
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
    ) {
        Column(
            modifier = Modifier.padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = "Data Simulation",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = DataTeal
            )
            
            Text(
                text = "Publish demo data to test the app functionality",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            
            // Sending Interval Input
            OutlinedTextField(
                value = sendingInterval,
                onValueChange = onUpdateInterval,
                label = { Text("Sending Interval (seconds)") },
                modifier = Modifier.fillMaxWidth(),
                enabled = !isSimulating,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                singleLine = true,
                placeholder = { Text("5") }
            )
            
            // Simulation Status
            if (isSimulating) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "Publishing every ${sendingInterval}s",
                        style = MaterialTheme.typography.bodySmall,
                        color = SuccessGreen
                    )
                    Text(
                        text = "Packets sent: $packetsPublished",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            
            // Simulation Control Button
            Button(
                onClick = {
                    if (isSimulating) {
                        onStopSimulation()
                    } else {
                        onStartSimulation()
                    }
                },
                modifier = Modifier.fillMaxWidth(),
                enabled = isConnected,
                shape = RoundedCornerShape(12.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = if (isSimulating) ErrorRed else SuccessGreen
                )
            ) {
                Text(
                    text = if (isSimulating) "Stop Data Simulation" else "Simulate Data",
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                    modifier = Modifier.padding(8.dp)
                )
            }
            
            // Info text
            Text(
                text = if (isSimulating) {
                    "Publishing 10 packets each to heartbeat, data, and daq topics every ${sendingInterval} seconds."
                } else if (!isConnected) {
                    "Connect MQTT first to enable simulation."
                } else {
                    "This will publish demo data to heartbeat, data, and daq topics for testing purposes."
                },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}
