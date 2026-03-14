package com.autogridmobility.rmsmqtt1.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.Star
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
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.data.ParameterMappings
import com.autogridmobility.rmsmqtt1.ui.components.RawDataTable
import com.autogridmobility.rmsmqtt1.ui.navigation.RawDataTab
import com.autogridmobility.rmsmqtt1.viewmodel.RawDataViewModel

@Composable
fun RawDataScreen(
    viewModel: RawDataViewModel = viewModel()
) {
    var selectedTab by remember { mutableStateOf(RawDataTab.HEARTBEAT) }
    
    val heartbeatHistory by viewModel.heartbeatHistory.collectAsState()
    val pumpHistory by viewModel.pumpHistory.collectAsState()
    val daqHistory by viewModel.daqHistory.collectAsState()
    
    Column(
        modifier = Modifier.fillMaxSize()
    ) {
        // Bottom Navigation for tabs
        NavigationBar(
            containerColor = MaterialTheme.colorScheme.surface,
            modifier = Modifier.fillMaxWidth()
        ) {
            RawDataTab.values().forEach { tab ->
                NavigationBarItem(
                    icon = {
                        Icon(
                            imageVector = when (tab) {
                                RawDataTab.HEARTBEAT -> Icons.Default.Favorite
                                RawDataTab.DATA -> Icons.Default.Star
                                RawDataTab.DAQ -> Icons.Default.Build
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
        when (selectedTab) {
            RawDataTab.HEARTBEAT -> {
                RawDataContent(
                    title = "Heartbeat Raw Data",
                    history = heartbeatHistory,
                    parameterMapping = ParameterMappings.heartbeatMapping
                )
            }
            
            RawDataTab.DATA -> {
                RawDataContent(
                    title = "Pump Data Raw Data",
                    history = pumpHistory,
                    parameterMapping = ParameterMappings.pumpDataMapping
                )
            }
            
            RawDataTab.DAQ -> {
                RawDataContent(
                    title = "DAQ Raw Data",
                    history = daqHistory,
                    parameterMapping = ParameterMappings.daqDataMapping
                )
            }
        }
    }
}

@Composable
private fun RawDataContent(
    title: String,
    history: List<Pair<String, String>>,
    parameterMapping: Map<String, com.autogridmobility.rmsmqtt1.data.ParameterInfo>
) {
    if (history.isEmpty()) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Text(
                text = "No raw data received yet",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    } else {
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            items(history.reversed()) { (timestamp, jsonData) ->
                RawDataTable(
                    title = "$title - $timestamp",
                    jsonData = jsonData,
                    parameterMapping = parameterMapping
                )
            }
        }
    }
}
