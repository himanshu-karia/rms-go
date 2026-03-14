package com.autogridmobility.rmsmqtt1.ui.screens

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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
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
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.data.ParameterMappings
import com.autogridmobility.rmsmqtt1.ui.components.DataTable
import com.autogridmobility.rmsmqtt1.ui.navigation.DashboardTab
import com.autogridmobility.rmsmqtt1.ui.theme.ErrorRed
import com.autogridmobility.rmsmqtt1.ui.theme.SuccessGreen
import com.autogridmobility.rmsmqtt1.viewmodel.DashboardViewModel

@Composable
fun DashboardScreen(
    viewModel: DashboardViewModel = viewModel()
) {
    var selectedTab by remember { mutableStateOf(DashboardTab.HEARTBEAT) }
    
    val heartbeatData by viewModel.heartbeatData.collectAsState()
    val pumpData by viewModel.pumpData.collectAsState()
    val daqData by viewModel.daqData.collectAsState()
    val lastOnDemandResponse by viewModel.lastOnDemandResponse.collectAsState()
    
    Column(
        modifier = Modifier.fillMaxSize()
    ) {
        // Bottom Navigation for tabs
        NavigationBar(
            containerColor = MaterialTheme.colorScheme.surface,
            modifier = Modifier.fillMaxWidth()
        ) {
            DashboardTab.values().forEach { tab ->
                NavigationBarItem(
                    icon = {
                        Icon(
                            imageVector = when (tab) {
                                DashboardTab.HEARTBEAT -> Icons.Default.Favorite
                                DashboardTab.DATA -> Icons.Default.Star
                                DashboardTab.DAQ -> Icons.Default.Build
                                DashboardTab.ON_DEMAND -> Icons.Default.Settings
                            },
                            contentDescription = tab.title
                        )
                    },
                    label = { Text(tab.title) },
                    selected = selectedTab == tab,
                    onClick = { selectedTab = tab }
                )
            }
        }
        
        // Content based on selected tab
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(16.dp)
                .verticalScroll(rememberScrollState())
        ) {
            when (selectedTab) {
                DashboardTab.HEARTBEAT -> {
                    heartbeatData?.let { data ->
                        val dataMap = mapOf(
                            "VD" to data.VD,
                            "TIMESTAMP" to data.TIMESTAMP,
                            "IMEI" to data.IMEI,
                            "LAT" to data.LAT,
                            "LONG" to data.LONG,
                            "RSSI" to data.RSSI,
                            "TEMP" to data.TEMP,
                            "VBATT" to data.VBATT,
                            "GSM" to data.GSM,
                            "GPRS" to data.GPRS,
                            "GPS" to data.GPS,
                            "ONLINE" to data.ONLINE
                        )
                        DataTable(
                            title = "Heartbeat Data (VD=0)",
                            data = dataMap,
                            parameterMapping = ParameterMappings.heartbeatMapping
                        )
                    } ?: run {
                        NoDataCard("No heartbeat data received yet")
                    }
                }
                
                DashboardTab.DATA -> {
                    pumpData?.let { data ->
                        val dataMap = mapOf(
                            "VD" to data.VD,
                            "TIMESTAMP" to data.TIMESTAMP,
                            "IMEI" to data.IMEI,
                            "PDKWH1" to data.PDKWH1,
                            "PTOTKWH1" to data.PTOTKWH1,
                            "POPDWD1" to data.POPDWD1,
                            "POPTOTWD1" to data.POPTOTWD1,
                            "POPKW1" to data.POPKW1,
                            "PRUNST1" to data.PRUNST1,
                            "POPV1" to data.POPV1,
                            "PDC1V1" to data.PDC1V1,
                            "POPFREQ1" to data.POPFREQ1
                        )
                        DataTable(
                            title = "Pump Data (VD=1)",
                            data = dataMap,
                            parameterMapping = ParameterMappings.pumpDataMapping
                        )
                    } ?: run {
                        NoDataCard("No pump data received yet")
                    }
                }
                
                DashboardTab.DAQ -> {
                    daqData?.let { data ->
                        val dataMap = mapOf(
                            "VD" to data.VD,
                            "TIMESTAMP" to data.TIMESTAMP,
                            "IMEI" to data.IMEI,
                            "AI11" to data.AI11,
                            "AI21" to data.AI21,
                            "AI31" to data.AI31,
                            "AI41" to data.AI41,
                            "DI11" to data.DI11,
                            "DI21" to data.DI21,
                            "DO11" to data.DO11,
                            "DO21" to data.DO21
                        )
                        DataTable(
                            title = "DAQ Data (VD=12)",
                            data = dataMap,
                            parameterMapping = ParameterMappings.daqDataMapping
                        )
                    } ?: run {
                        NoDataCard("No DAQ data received yet")
                    }
                }
                
                DashboardTab.ON_DEMAND -> {
                    OnDemandTabContent(
                        onTurnOn = { viewModel.sendPumpOnCommand() },
                        onTurnOff = { viewModel.sendPumpOffCommand() },
                        lastResponse = lastOnDemandResponse
                    )
                }
            }
        }
    }
}

@Composable
private fun OnDemandTabContent(
    onTurnOn: () -> Unit,
    onTurnOff: () -> Unit,
    lastResponse: com.autogridmobility.rmsmqtt1.data.OnDemandResponse?
) {
    Column(
        verticalArrangement = Arrangement.spacedBy(24.dp)
    ) {
        // Pump Control Section
        Text(
            text = "Pump Control",
            style = MaterialTheme.typography.titleLarge,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Button(
                onClick = onTurnOn,
                modifier = Modifier.weight(1f),
                shape = RoundedCornerShape(12.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = SuccessGreen
                )
            ) {
                Text(
                    text = "Turn ON",
                    fontWeight = FontWeight.Bold,
                    color = Color.White
                )
            }
            
            Button(
                onClick = onTurnOff,
                modifier = Modifier.weight(1f),
                shape = RoundedCornerShape(12.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = ErrorRed
                )
            ) {
                Text(
                    text = "Turn OFF",
                    fontWeight = FontWeight.Bold,
                    color = Color.White
                )
            }
        }
        
        // Latest Command Status Section
        Text(
            text = "Latest Command Status",
            style = MaterialTheme.typography.titleLarge,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        
        lastResponse?.let { response ->
            val responseMap = mapOf(
                "timestamp" to response.timestamp,
                "DO1" to if (response.DO1 == 1) "ON" else "OFF",
                "PRUNST1" to if (response.PRUNST1 == "1") "Running" else "Stopped"
            )
            DataTable(
                title = "Command Response",
                data = responseMap,
                parameterMapping = ParameterMappings.onDemandResponseMapping
            )
        } ?: run {
            NoDataCard("No commands sent yet")
        }
        
        Text(
            text = "This status reflects a simulated response from the device after a command is sent.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun NoDataCard(message: String) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(
            text = message,
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
