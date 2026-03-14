package com.autogridmobility.rmsmqtt1.ui.screens

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
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.service.MqttConnectionState
import com.autogridmobility.rmsmqtt1.ui.components.ConnectionStatusIndicator
import com.autogridmobility.rmsmqtt1.ui.theme.DataBlue
import com.autogridmobility.rmsmqtt1.ui.theme.ErrorRed
import com.autogridmobility.rmsmqtt1.ui.theme.SuccessGreen
import com.autogridmobility.rmsmqtt1.ui.theme.WarningYellow
import com.autogridmobility.rmsmqtt1.viewmodel.SettingsViewModel

@Composable
fun SettingsScreen(
    viewModel: SettingsViewModel = viewModel()
) {
    val url by viewModel.url.collectAsState()
    val port by viewModel.port.collectAsState()
    val username by viewModel.username.collectAsState()
    val password by viewModel.password.collectAsState()
    val clientId by viewModel.clientId.collectAsState()
    val topicPrefix by viewModel.topicPrefix.collectAsState()
    val fieldsEditable by viewModel.fieldsEditable.collectAsState()
    val urlError by viewModel.urlError.collectAsState()
    val portError by viewModel.portError.collectAsState()
    val clientIdError by viewModel.clientIdError.collectAsState()
    val topicPrefixError by viewModel.topicPrefixError.collectAsState()
    val connectionStatus by viewModel.connectionStatus.collectAsState()
    val isConnecting by viewModel.isConnecting.collectAsState()
    val buttonConfig by viewModel.buttonConfig.collectAsState()
    val currentState by viewModel.currentState.collectAsState()
    val sendingInterval by viewModel.sendingInterval.collectAsState()
    val isSimulating by viewModel.isSimulating.collectAsState()
    val packetsPublished by viewModel.packetsPublished.collectAsState()
    val subscribedTopics by viewModel.subscribedTopics.collectAsState()
    val errorMessage by viewModel.errorMessage.collectAsState()
    
    var passwordVisible by remember { mutableStateOf(false) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp)
            .verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // Error/info Snackbar
        if (errorMessage != null) {
            androidx.compose.material3.Snackbar(
                modifier = Modifier.padding(bottom = 8.dp),
                action = {
                    Text(
                        text = "Dismiss",
                        modifier = Modifier.clickable { viewModel.clearErrorMessage() },
                        color = ErrorRed
                    )
                },
                content = {
                    Text(text = errorMessage ?: "")
                }
            )
        }

        
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
                        viewModel.refreshConnectionStatus()
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
                    onValueChange = { if (fieldsEditable) viewModel.updateUrl(it) },
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
                    onValueChange = { if (fieldsEditable) viewModel.updatePort(it) },
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
                    onValueChange = { if (fieldsEditable) viewModel.updateUsername(it) },
                    label = { Text("Username (optional)") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEditable,
                    singleLine = true
                )
                
                // Password
                OutlinedTextField(
                    value = password,
                    onValueChange = { if (fieldsEditable) viewModel.updatePassword(it) },
                    label = { Text("Password (optional)") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEditable,
                    visualTransformation = if (passwordVisible) VisualTransformation.None else PasswordVisualTransformation(),
                    trailingIcon = {
                        IconButton(
                            onClick = { passwordVisible = !passwordVisible },
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
                        onValueChange = { if (fieldsEditable) viewModel.updateClientId(it) },
                        label = { Text("Client ID") },
                        modifier = Modifier.weight(1f),
                        enabled = fieldsEditable,
                        isError = clientIdError != null,
                        supportingText = clientIdError?.let { { Text(it, color = ErrorRed) } },
                        singleLine = true
                    )
                    
                    Button(
                        onClick = { viewModel.generateNewClientId() },
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
                    onValueChange = { if (fieldsEditable) viewModel.updateTopicPrefix(it) },
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
        
        // Connection Control - Using State Manager
        Button(
            onClick = {
                // Use state-based button logic
                when (currentState) {
                    com.autogridmobility.rmsmqtt1.service.MqttConnectionState.DISCONNECTED,
                    com.autogridmobility.rmsmqtt1.service.MqttConnectionState.ERROR,
                    com.autogridmobility.rmsmqtt1.service.MqttConnectionState.NETWORK_LOST -> {
                        viewModel.connect()
                    }
                    com.autogridmobility.rmsmqtt1.service.MqttConnectionState.CONNECTED -> {
                        viewModel.disconnect()
                    }
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
        
        // Reconnect Button (shown only when disconnected)
        if (currentState == MqttConnectionState.DISCONNECTED || currentState == MqttConnectionState.ERROR) {
            Spacer(modifier = Modifier.height(8.dp))
            
            Button(
                onClick = { viewModel.reconnect() },
                modifier = Modifier.fillMaxWidth(),
                enabled = !isConnecting,
                shape = RoundedCornerShape(12.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = DataBlue,
                    disabledContainerColor = DataBlue.copy(alpha = 0.5f)
                )
            ) {
                Text(
                    text = "Reconnect",
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                    modifier = Modifier.padding(8.dp)
                )
            }
        }
        
        // Data Simulation Settings Card
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
                    fontWeight = FontWeight.SemiBold
                )
                
                // Sending Interval
                OutlinedTextField(
                    value = sendingInterval,
                    onValueChange = viewModel::updateSendingInterval,
                    label = { Text("Sending Interval (seconds)") },
                    modifier = Modifier.fillMaxWidth(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    singleLine = true
                )
                
                // Simulation Status
                Text(
                    text = "Status: ${if (isSimulating) "Running" else "Stopped"}",
                    style = MaterialTheme.typography.bodyMedium,
                    color = if (isSimulating) SuccessGreen else MaterialTheme.colorScheme.onSurfaceVariant
                )
                
                // Packets Published Counter
                Text(
                    text = "Packets Published: $packetsPublished",
                    style = MaterialTheme.typography.bodyMedium,
                    color = DataBlue
                )
                
                // Simulation Control Button
                Button(
                    onClick = {
                        if (isSimulating) {
                            viewModel.stopSimulation()
                        } else {
                            viewModel.startSimulation()
                        }
                    },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = currentState == MqttConnectionState.CONNECTED,
                    shape = RoundedCornerShape(12.dp),
                    colors = ButtonDefaults.buttonColors(
                        containerColor = if (isSimulating) ErrorRed else SuccessGreen
                    )
                ) {
                    Text(
                        text = if (isSimulating) "Stop Simulation" else "Start Simulation",
                        fontWeight = FontWeight.Bold,
                        color = Color.White
                    )
                }
            }
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
        
        Spacer(modifier = Modifier.height(16.dp))
        
        // Force Reset Button
        Spacer(modifier = Modifier.height(8.dp))
        Button(
            onClick = { viewModel.forceResetMqtt() },
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(
                containerColor = ErrorRed,
                disabledContainerColor = ErrorRed.copy(alpha = 0.5f)
            )
        ) {
            Text(
                text = "Force Reset MQTT",
                fontWeight = FontWeight.Bold,
                color = Color.White,
                modifier = Modifier.padding(8.dp)
            )
        }

        // Info text
        Text(
            text = "Configure your MQTT broker settings above. All settings are prefilled with defaults for test.mosquitto.org. " +
                   "Fields are locked while connected - disconnect to modify settings. Press Connect to establish connection using your configuration. " +
                   "Topics will be auto-subscribed based on your Topic Prefix. Use Force Reset if the connection is stuck or unrecoverable.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
