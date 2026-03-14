package com.autogridmobility.rmsmqtt1.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.viewmodel.UxDashboardViewModel
import com.autogridmobility.rmsmqtt1.ui.components.UxTimeSeriesGraph
import com.autogridmobility.rmsmqtt1.ui.components.UxMultiSeriesGraph
import com.autogridmobility.rmsmqtt1.ui.components.exportSeriesCsv
import com.autogridmobility.rmsmqtt1.viewmodel.DataPoint

/** Dedicated graphs screen with multi-series overlay support and pause controls */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun GraphsScreen(vm: UxDashboardViewModel = viewModel()) {
    val freq by vm.frequencySeries.collectAsState()
    val power by vm.powerSeries.collectAsState()
    val flow by vm.flowSeries.collectAsState()
    val current by vm.currentSeries.collectAsState()
    val voltage by vm.voltageSeries.collectAsState()
    val battery by vm.batterySeries.collectAsState()
    val temp by vm.temperatureSeries.collectAsState()

    var overlayEnabled by remember { mutableStateOf(false) }
    var primarySelection by remember { mutableStateOf("Frequency") }
    var secondarySelection by remember { mutableStateOf("Power") }

    val seriesMap: Map<String, List<DataPoint>> = mapOf(
        "Frequency" to freq,
        "Power" to power,
        "Flow" to flow,
        "Current" to current,
        "Voltage" to voltage,
        "Battery" to battery,
        "Temp" to temp
    )

    val retentionMinutes by vm.retentionMinutes.collectAsState()
    var retention by remember(retentionMinutes) { mutableStateOf(retentionMinutes.toFloat()) }
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        Text("Graphs", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold)
        Text("Retention (minutes): ${retention.toInt()}", style = MaterialTheme.typography.bodySmall)
    Slider(value = retention, onValueChange = { retention = it; vm.setRetentionMinutes(retention.toLong()) }, valueRange = 1f..60f, steps = 59)
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            FilterChip(selected = overlayEnabled, onClick = { overlayEnabled = !overlayEnabled }, label = { Text("Overlay") })
            if (overlayEnabled) {
                SeriesDropdown("Primary", primarySelection, seriesMap.keys.toList()) { primarySelection = it }
                SeriesDropdown("Secondary", secondarySelection, seriesMap.keys.toList()) { secondarySelection = it }
            }
        }

        if (overlayEnabled) {
            val a = seriesMap[primarySelection].orEmpty()
            val b = seriesMap[secondarySelection].orEmpty()
            UxMultiSeriesGraph(a, b, primarySelection, secondarySelection)
        }

        seriesMap.forEach { (name, list) ->
            PausableGraph(name, list)
        }

        // Unified export controls
        Text("Export", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
        // Simple wrapping chips (manual rows) to avoid experimental FlowRow
        val chipsPerRow = 3
        val entries = seriesMap.entries.filter { it.value.isNotEmpty() }
        entries.chunked(chipsPerRow).forEach { chunk ->
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                chunk.forEach { (name, list) ->
                    AssistChip(onClick = { exportSeriesCsv(name, list) }, label = { Text(name) })
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SeriesDropdown(label: String, current: String, options: List<String>, onChange: (String) -> Unit) {
    var expanded by remember { mutableStateOf(false) }
    ExposedDropdownMenuBox(expanded = expanded, onExpandedChange = { expanded = !expanded }) {
        OutlinedTextField(value = current, onValueChange = {}, readOnly = true, label = { Text(label) }, modifier = Modifier.menuAnchor().width(140.dp))
        ExposedDropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            options.forEach { opt -> DropdownMenuItem(text = { Text(opt) }, onClick = { onChange(opt); expanded = false }) }
        }
    }
}

@Composable
private fun PausableGraph(name: String, points: List<DataPoint>) {
    var paused by remember { mutableStateOf(false) }
    if (points.isEmpty()) {
        PlaceholderCard(name)
    } else {
        UxTimeSeriesGraph(points = points, label = name, paused = paused, onTogglePause = { paused = !paused })
    }
}

@Composable
private fun OverlayGraph(nameA: String, a: List<DataPoint>, nameB: String, b: List<DataPoint>) {
    if (a.isEmpty() && b.isEmpty()) { PlaceholderCard("Overlay: $nameA + $nameB"); return }
    Text("Overlay: $nameA + $nameB", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
    UxTimeSeriesGraph(points = a, label = nameA, showYAxisTicks = false)
    Spacer(Modifier.height(4.dp))
    UxTimeSeriesGraph(points = b, label = nameB, showYAxisTicks = false)
}

@Composable
private fun PlaceholderCard(label: String) {
    ElevatedCard(modifier = Modifier.fillMaxWidth().height(120.dp)) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Text(
                "Waiting for $label data...",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}
