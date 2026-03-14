package com.autogridmobility.rmsmqtt1.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.clickable
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.ui.components.*
import com.autogridmobility.rmsmqtt1.ui.theme.*
import com.autogridmobility.rmsmqtt1.viewmodel.UxDashboardViewModel
import java.time.format.DateTimeFormatter

/**
 * UX Dashboard Screen - Advanced real-time monitoring with intelligent data correlation
 */
@Composable
fun UxDashboardScreen(
    viewModel: UxDashboardViewModel = viewModel()
) {
    val dashboardData by viewModel.dashboardData.collectAsState()
    // Hoist time-series collections here to avoid using viewModel inside child composables
    val frequencySeries by viewModel.frequencySeries.collectAsState()
    val powerSeries by viewModel.powerSeries.collectAsState()
    val flowSeries by viewModel.flowSeries.collectAsState()
    val currentSeries by viewModel.currentSeries.collectAsState()
    val voltageSeries by viewModel.voltageSeries.collectAsState()
    val batterySeries by viewModel.batterySeries.collectAsState()
    val temperatureSeries by viewModel.temperatureSeries.collectAsState()
    val ai11Series by viewModel.ai11Series.collectAsState()
    val ai21Series by viewModel.ai21Series.collectAsState()
    val ai31Series by viewModel.ai31Series.collectAsState()
    val ai41Series by viewModel.ai41Series.collectAsState()
    
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp)
            .verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // Header with device info and last update
        UxDashboardHeader(
            deviceImei = dashboardData.deviceImei,
            lastUpdate = dashboardData.lastUpdateTime?.format(
                DateTimeFormatter.ofPattern("HH:mm:ss")
            ) ?: "No Data"
        )
        
        // Communication & Connectivity Hub
        CommunicationStatusHub(
            communicationStatus = dashboardData.communicationStatus,
            rssi = dashboardData.rssi,
            modifier = Modifier.fillMaxWidth()
        )
        
        // Power & Battery System
        PowerBatterySection(
            batteryVoltage = dashboardData.batteryVoltage,
            batteryStatus = dashboardData.batteryStatus,
            powerState = dashboardData.powerState,
            temperature = dashboardData.temperature,
            batterySeries = batterySeries,
            temperatureSeries = temperatureSeries,
            batteryStale = dashboardData.batteryVoltage.isStale,
            temperatureStale = dashboardData.temperature.isStale
        )
        
        // Pump Operations Hub - Main gauges
        PumpOperationsSection(
            pumpRunning = dashboardData.pumpRunning,
            frequency = dashboardData.frequency,
            power = dashboardData.power,
            flowRate = dashboardData.flowRate,
            current = dashboardData.current,
            voltage = dashboardData.voltage,
            frequencySeries = frequencySeries,
            powerSeries = powerSeries,
            flowSeries = flowSeries,
            currentSeries = currentSeries,
            voltageSeries = voltageSeries,
            frequencyStale = dashboardData.frequency.isStale,
            powerStale = dashboardData.power.isStale,
            flowStale = dashboardData.flowRate.isStale,
            currentStale = dashboardData.current.isStale,
            voltageStale = dashboardData.voltage.isStale
        )

    // Removed inline trend graphs to keep dashboard focused on gauges; use Graphs screen for charts
        
        // Energy Monitoring
        EnergyMonitoringCard(
            dailyEnergy = dashboardData.dailyEnergy,
            totalEnergy = dashboardData.totalEnergy,
            dailyWater = dashboardData.dailyWater,
            totalWater = dashboardData.totalWater,
            dailyHours = dashboardData.dailyHours,
            totalHours = dashboardData.totalHours,
            modifier = Modifier.fillMaxWidth()
        )
        
        // System Health Grid
        SystemHealthSection(
            temperature = dashboardData.temperature,
            gpsStatus = dashboardData.gpsStatus,
            gpsLocation = dashboardData.gpsLocation,
            rfModule = dashboardData.rfModule,
            sdCard = dashboardData.sdCard,
            flashMemory = dashboardData.flashMemory
        )
        
        // Digital I/O Matrix
        DigitalIOSection(
            analogInputs = dashboardData.analogInputs,
            digitalInputs = dashboardData.digitalInputs,
            digitalOutputs = dashboardData.digitalOutputs,
            ai11Series = ai11Series,
            ai21Series = ai21Series,
            ai31Series = ai31Series,
            ai41Series = ai41Series
        )
    }
}

@Composable
private fun UxDashboardHeader(
    deviceImei: String,
    lastUpdate: String
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = DataBlue.copy(alpha = 0.1f)
        )
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column {
                Text(
                    text = "UX Dashboard",
                    style = MaterialTheme.typography.headlineSmall,
                    color = DataBlue,
                    fontWeight = FontWeight.Bold
                )
                Text(
                    text = "Device: $deviceImei",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            
            Column(
                horizontalAlignment = Alignment.End
            ) {
                Text(
                    text = "Last Update",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = lastUpdate,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = DataTeal
                )
            }
        }
    }
}

@Composable
private fun PowerBatterySection(
    batteryVoltage: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    batteryStatus: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    powerState: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    temperature: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    batterySeries: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    temperatureSeries: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    batteryStale: Boolean,
    temperatureStale: Boolean
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        )
    ) {
        Column(
            modifier = Modifier.padding(16.dp)
        ) {
            Text(
                text = "Power & Battery System",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(bottom = 12.dp)
            )

            // Status icons row (moved here to avoid overlapping gauges)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(32.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                UxStatusIcon(
                    value = batteryStatus,
                    label = "Battery",
                    onIcon = {
                        Icon(
                            Icons.Filled.Battery4Bar,
                            contentDescription = "Battery Good",
                            modifier = Modifier.size(28.dp),
                            tint = SuccessGreen
                        )
                    },
                    offIcon = {
                        Icon(
                            Icons.Filled.BatteryAlert,
                            contentDescription = "Battery Warning",
                            modifier = Modifier.size(28.dp),
                            tint = ErrorRed
                        )
                    }
                )
                UxStatusIcon(
                    value = powerState,
                    label = "Power",
                    onIcon = {
                        Icon(
                            Icons.Filled.Power,
                            contentDescription = "Power On",
                            modifier = Modifier.size(28.dp),
                            tint = SuccessGreen
                        )
                    },
                    offIcon = {
                        Icon(
                            Icons.Filled.PowerOff,
                            contentDescription = "Power Off",
                            modifier = Modifier.size(28.dp),
                            tint = Color.Gray
                        )
                    }
                )
            }
            
            // 2-column grid layout
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly
            ) {
                // Column 1: Battery voltage gauge (mini trend removed; see Graphs screen)
                UxGridCell(modifier = Modifier.weight(1f)) {
                    UxGauge(
                        value = batteryVoltage,
                        label = "Battery",
                        unit = "V",
                        minValue = 0f,
                        maxValue = 5f,
                        warningThreshold = 3.2f,
                        criticalThreshold = 3.0f
                    )
                }
                
                // Column 2: Temperature gauge
                UxGridCell(modifier = Modifier.weight(1f)) {
                    // Temperature gauge
                    UxGauge(
                        value = temperature,
                        label = "Temperature",
                        unit = "°C",
                        minValue = -10f,
                        maxValue = 60f,
                        warningThreshold = 45f,
                        criticalThreshold = 55f
                    )
                }
            }
        }
    }
}

@Composable
private fun PumpOperationsSection(
    pumpRunning: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    frequency: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    power: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    flowRate: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    current: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    voltage: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    frequencySeries: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    powerSeries: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    flowSeries: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    currentSeries: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    voltageSeries: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    frequencyStale: Boolean,
    powerStale: Boolean,
    flowStale: Boolean,
    currentStale: Boolean,
    voltageStale: Boolean
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        )
    ) {
        Column(
            modifier = Modifier.padding(16.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "Pump Operations",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )
                
                // Pump running status
                Row(
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Icon(
                        imageVector = if (pumpRunning.value == "1" && !pumpRunning.isStale) 
                            Icons.Filled.PlayArrow else Icons.Filled.Stop,
                        contentDescription = "Pump Status",
                        tint = if (pumpRunning.value == "1" && !pumpRunning.isStale) 
                            SuccessGreen else ErrorRed,
                        modifier = Modifier.size(20.dp)
                    )
                    Text(
                        text = if (pumpRunning.isStale) "No Data" 
                               else if (pumpRunning.value == "1") "Running" else "Stopped",
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                        color = if (pumpRunning.value == "1" && !pumpRunning.isStale) 
                                SuccessGreen else ErrorRed,
                        modifier = Modifier.padding(start = 4.dp)
                    )
                }
            }
            
            Spacer(modifier = Modifier.height(16.dp))
            
            // 2-column grid layout for gauges
            Column(
                verticalArrangement = Arrangement.spacedBy(16.dp)
            ) {
                // Row 1
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceEvenly
                ) {
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        UxGauge(
                            value = frequency,
                            label = "Frequency",
                            unit = "Hz",
                            minValue = 0f,
                            maxValue = 60f,
                            warningThreshold = 55f,
                            criticalThreshold = 58f
                        )
                    }
                    
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        UxGauge(
                            value = power,
                            label = "Power",
                            unit = "kW",
                            minValue = 0f,
                            maxValue = 10f,
                            warningThreshold = 8f,
                            criticalThreshold = 9f
                        )
                    }
                }
                
                // Row 2
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceEvenly
                ) {
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        UxGauge(
                            value = flowRate,
                            label = "Flow Rate",
                            unit = "L/min",
                            minValue = 0f,
                            maxValue = 500f
                        )
                    }
                    
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        UxGauge(
                            value = current,
                            label = "Current",
                            unit = "A",
                            minValue = 0f,
                            maxValue = 50f,
                            warningThreshold = 40f,
                            criticalThreshold = 45f
                        )
                    }
                }
                
                // Row 3
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceEvenly
                ) {
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        UxGauge(
                            value = voltage,
                            label = "Voltage",
                            unit = "V",
                            minValue = 300f,
                            maxValue = 400f,
                            warningThreshold = 390f,
                            criticalThreshold = 395f
                        )
                    }
                    
                    // Empty cell for future expansion
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        // Placeholder for future gauge
                    }
                }
                
                // Mini pump metric graphs removed; refer to Graphs screen for detailed time-series
            }
        }
    }
}

@Composable
private fun SystemHealthSection(
    temperature: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    gpsStatus: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    gpsLocation: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    rfModule: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    sdCard: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    flashMemory: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        )
    ) {
        Column(
            modifier = Modifier.padding(16.dp)
        ) {
            Text(
                text = "System Health",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(bottom = 12.dp)
            )
            
            // 2-column grid layout
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly
            ) {
                // Column 1: Temperature gauge
                UxGridCell(modifier = Modifier.weight(1f)) {
                    UxGauge(
                        value = temperature,
                        label = "Temperature",
                        unit = "°C",
                        minValue = -10f,
                        maxValue = 60f,
                        warningThreshold = 45f,
                        criticalThreshold = 55f
                    )
                }
                
                // Column 2: Status icons in 2x2 sub-grid
                UxGridCell(modifier = Modifier.weight(1f)) {
                    UxIconGrid(
                        topLeft = {
                            UxStatusIcon(
                                value = gpsStatus,
                                label = "GPS",
                                onIcon = {
                                    Icon(
                                        Icons.Filled.Satellite,
                                        contentDescription = "GPS Active",
                                        modifier = Modifier.size(20.dp)
                                    )
                                },
                                offIcon = {
                                    Icon(
                                        Icons.Filled.SatelliteAlt,
                                        contentDescription = "GPS Inactive",
                                        modifier = Modifier.size(20.dp)
                                    )
                                }
                            )
                        },
                        topRight = {
                            UxStatusIcon(
                                value = gpsLocation,
                                label = "Location",
                                onIcon = {
                                    Icon(
                                        Icons.Filled.LocationOn,
                                        contentDescription = "Location Fixed",
                                        modifier = Modifier.size(20.dp)
                                    )
                                },
                                offIcon = {
                                    Icon(
                                        Icons.Filled.LocationOff,
                                        contentDescription = "Location Lost",
                                        modifier = Modifier.size(20.dp)
                                    )
                                }
                            )
                        },
                        bottomLeft = {
                            UxStatusIcon(
                                value = rfModule,
                                label = "RF Module",
                                onIcon = {
                                    Icon(
                                        Icons.Filled.Wifi,
                                        contentDescription = "RF Active",
                                        modifier = Modifier.size(20.dp)
                                    )
                                },
                                offIcon = {
                                    Icon(
                                        Icons.Filled.WifiOff,
                                        contentDescription = "RF Inactive",
                                        modifier = Modifier.size(20.dp)
                                    )
                                }
                            )
                        },
                        bottomRight = {
                            UxStatusIcon(
                                value = sdCard,
                                label = "SD Card",
                                onIcon = {
                                    Icon(
                                        Icons.Filled.SdCard,
                                        contentDescription = "SD Card OK",
                                        modifier = Modifier.size(20.dp)
                                    )
                                },
                                offIcon = {
                                    Icon(
                                        Icons.Filled.SdCardAlert,
                                        contentDescription = "SD Card Error",
                                        modifier = Modifier.size(20.dp)
                                    )
                                }
                            )
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun DigitalIOSection(
    analogInputs: List<com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>>,
    digitalInputs: List<com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>>,
    digitalOutputs: List<com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>>,
    ai11Series: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    ai21Series: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    ai31Series: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>,
    ai41Series: List<com.autogridmobility.rmsmqtt1.viewmodel.DataPoint>
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        )
    ) {
        Column(
            modifier = Modifier.padding(16.dp)
        ) {
            Text(
                text = "Digital I/O Matrix",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(bottom = 12.dp)
            )
            
            // Analog Inputs - 2 columns with 2 gauges each
            Text(
                text = "Analog Inputs",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
                color = DataTeal,
                modifier = Modifier.padding(bottom = 8.dp)
            )
            
            Column(
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                // Row 1: AI1 and AI2
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceEvenly
                ) {
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        if (analogInputs.isNotEmpty()) {
                            UxCompactGauge(
                                value = analogInputs[0],
                                label = "AI1",
                                unit = "",
                                minValue = 0f,
                                maxValue = 100f
                            )
                        }
                    }
                    
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        if (analogInputs.size > 1) {
                            UxCompactGauge(
                                value = analogInputs[1],
                                label = "AI2",
                                unit = "",
                                minValue = 0f,
                                maxValue = 100f
                            )
                        }
                    }
                }
                
                // Row 2: AI3 and AI4
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceEvenly
                ) {
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        if (analogInputs.size > 2) {
                            UxCompactGauge(
                                value = analogInputs[2],
                                label = "AI3",
                                unit = "",
                                minValue = 0f,
                                maxValue = 100f
                            )
                        }
                    }
                    
                    UxGridCell(modifier = Modifier.weight(1f)) {
                        if (analogInputs.size > 3) {
                            UxCompactGauge(
                                value = analogInputs[3],
                                label = "AI4",
                                unit = "",
                                minValue = 0f,
                                maxValue = 100f
                            )
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(16.dp))
            
            // Digital Inputs and Outputs in 2 columns
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly
            ) {
                // Column 1: Digital Inputs
                UxGridCell(modifier = Modifier.weight(1f)) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally
                    ) {
                        Text(
                            text = "Digital Inputs",
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Medium,
                            color = DataTeal,
                            modifier = Modifier.padding(bottom = 8.dp)
                        )
                        
                        // Create 2x2 grid for 4 digital inputs
                        UxIconGrid(
                            topLeft = if (digitalInputs.isNotEmpty()) {
                                { DigitalIOIndicator("DI1", digitalInputs[0], true) }
                            } else null,
                            topRight = if (digitalInputs.size > 1) {
                                { DigitalIOIndicator("DI2", digitalInputs[1], true) }
                            } else null,
                            bottomLeft = if (digitalInputs.size > 2) {
                                { DigitalIOIndicator("DI3", digitalInputs[2], true) }
                            } else null,
                            bottomRight = if (digitalInputs.size > 3) {
                                { DigitalIOIndicator("DI4", digitalInputs[3], true) }
                            } else null
                        )
                    }
                }
                
                // Column 2: Digital Outputs
                UxGridCell(modifier = Modifier.weight(1f)) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally
                    ) {
                        Text(
                            text = "Digital Outputs",
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Medium,
                            color = DataTeal,
                            modifier = Modifier.padding(bottom = 8.dp)
                        )
                        
                        // Create 2x2 grid for 4 digital outputs
                        UxIconGrid(
                            topLeft = if (digitalOutputs.isNotEmpty()) {
                                { DigitalIOIndicator("DO1", digitalOutputs[0], false) }
                            } else null,
                            topRight = if (digitalOutputs.size > 1) {
                                { DigitalIOIndicator("DO2", digitalOutputs[1], false) }
                            } else null,
                            bottomLeft = if (digitalOutputs.size > 2) {
                                { DigitalIOIndicator("DO3", digitalOutputs[2], false) }
                            } else null,
                            bottomRight = if (digitalOutputs.size > 3) {
                                { DigitalIOIndicator("DO4", digitalOutputs[3], false) }
                            } else null
                        )
                    }
                }
            }
        }
    }
}

// ExpandableSeriesGraph removed; trends live on Graphs screen only

@Composable
private fun DigitalIOIndicator(
    label: String,
    value: com.autogridmobility.rmsmqtt1.viewmodel.UxValue<String>,
    isInput: Boolean
) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        val isActive = value.value == "1" && !value.isStale
        val color = when {
            value.isStale -> Color.Gray.copy(alpha = 0.5f)
            isActive -> SuccessGreen // Same color for both inputs and outputs
            else -> Color.Gray
        }
        
        Box(
            modifier = Modifier
                .size(24.dp)
                .padding(2.dp),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = if (isActive) Icons.Filled.Circle else Icons.Filled.RadioButtonUnchecked,
                contentDescription = "$label: ${if (isActive) "Active" else "Inactive"}",
                tint = color,
                modifier = Modifier.size(20.dp)
            )
        }
        
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall.copy(fontSize = 10.sp),
            color = color
        )
    }
}
