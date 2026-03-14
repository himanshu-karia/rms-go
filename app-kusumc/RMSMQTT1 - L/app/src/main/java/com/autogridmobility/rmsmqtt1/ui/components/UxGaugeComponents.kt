package com.autogridmobility.rmsmqtt1.ui.components

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.*
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.*
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.autogridmobility.rmsmqtt1.viewmodel.UxValue
import kotlin.math.*

/**
 * Semi-circular animated gauge component for UX Dashboard
 */
@Composable
fun UxGauge(
    value: UxValue<String>,
    label: String,
    unit: String,
    minValue: Float = 0f,
    maxValue: Float = 100f,
    modifier: Modifier = Modifier,
    warningThreshold: Float? = null,
    criticalThreshold: Float? = null
) {
    val currentValue = value.value.toFloatOrNull() ?: 0f
    
    // Animated value for smooth transitions
    val animatedValue by animateFloatAsState(
        targetValue = currentValue,
        animationSpec = tween(
            durationMillis = 1000,
            easing = EaseInOutCubic
        ),
        label = "gauge_value"
    )
    
    Column(
        modifier = modifier.padding(4.dp), // Reduced padding from 8dp to 4dp
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            modifier = Modifier.size(120.dp),
            contentAlignment = Alignment.BottomCenter // Align to bottom
        ) {
            Canvas(
                modifier = Modifier.fillMaxSize()
            ) {
                drawSemiCircularGauge(
                    value = animatedValue,
                    minValue = minValue,
                    maxValue = maxValue,
                    isFresh = value.isFresh,
                    isStale = value.isStale,
                    warningThreshold = warningThreshold,
                    criticalThreshold = criticalThreshold
                )
            }
            
            // Value and unit text aligned to bottom of gauge
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                modifier = Modifier.padding(bottom = 8.dp) // Align with gauge bottom
            ) {
                Text(
                    text = if (value.isStale) "No Data" else String.format("%.1f", animatedValue),
                    style = MaterialTheme.typography.titleMedium.copy(
                        fontWeight = FontWeight.Bold,
                        fontSize = 18.sp // Increased from 16sp by 2 points
                    ),
                    color = getValueColor(value)
                )
                Text(
                    text = unit,
                    style = MaterialTheme.typography.bodySmall,
                    color = getValueColor(value).copy(alpha = 0.7f)
                )
            }
        }
        
        // Label below the gauge
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall,
            color = getValueColor(value),
            modifier = Modifier.padding(top = 4.dp)
        )
    }
}

/**
 * Draw semi-circular gauge with color zones
 */
private fun DrawScope.drawSemiCircularGauge(
    value: Float,
    minValue: Float,
    maxValue: Float,
    isFresh: Boolean,
    isStale: Boolean,
    warningThreshold: Float?,
    criticalThreshold: Float?
) {
    val strokeWidth = 6.dp.toPx() // Made thinner - half of original 12dp
    val radius = (size.minDimension - strokeWidth) / 2
    val center = Offset(size.width / 2, size.height - 8.dp.toPx()) // Adjusted for better alignment
    
    // Background arc
    val backgroundBrush = if (isStale) {
        SolidColor(Color.Gray.copy(alpha = 0.3f))
    } else {
        SolidColor(Color.Gray.copy(alpha = 0.2f))
    }
    
    drawArc(
        brush = backgroundBrush,
        startAngle = 180f,
        sweepAngle = 180f,
        useCenter = false,
        topLeft = Offset(center.x - radius, center.y - radius),
        size = Size(radius * 2, radius * 2),
        style = Stroke(width = strokeWidth, cap = StrokeCap.Round)
    )
    
    if (!isStale && value > minValue) {
        // Calculate value position
        val normalizedValue = ((value - minValue) / (maxValue - minValue)).coerceIn(0f, 1f)
        val sweepAngle = normalizedValue * 180f
        
        // Determine color based on thresholds and freshness
        val gaugeColor = when {
            criticalThreshold != null && value >= criticalThreshold -> Color.Red
            warningThreshold != null && value >= warningThreshold -> Color.Yellow
            else -> if (isFresh) Color.Green else Color.Blue
        }
        
        val alpha = if (isFresh) 1f else 0.6f
        
        // Value arc
        drawArc(
            color = gaugeColor.copy(alpha = alpha),
            startAngle = 180f,
            sweepAngle = sweepAngle,
            useCenter = false,
            topLeft = Offset(center.x - radius, center.y - radius),
            size = Size(radius * 2, radius * 2),
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round)
        )
        
        // Value indicator dot
        val angle = Math.toRadians((180 + sweepAngle).toDouble())
        val dotX = center.x + radius * cos(angle).toFloat()
        val dotY = center.y + radius * sin(angle).toFloat()
        
        drawCircle(
            color = gaugeColor.copy(alpha = alpha),
            radius = strokeWidth / 2,
            center = Offset(dotX, dotY)
        )
    }
    
    // Scale markers
    drawScaleMarkers(center, radius, minValue, maxValue, strokeWidth)
}

/**
 * Draw scale markers on gauge
 */
private fun DrawScope.drawScaleMarkers(
    center: Offset,
    radius: Float,
    minValue: Float,
    maxValue: Float,
    strokeWidth: Float
) {
    val markerCount = 5
    val markerLength = 2.dp.toPx() // Reduced to 1/4 of original (was 8dp)
    val markerRadius = radius + strokeWidth / 2 + markerLength
    
    for (i in 0..markerCount) {
        val angle = Math.toRadians((180 + (i * 180f / markerCount)).toDouble())
        val startX = center.x + radius * cos(angle).toFloat()
        val startY = center.y + radius * sin(angle).toFloat()
        val endX = center.x + markerRadius * cos(angle).toFloat()
        val endY = center.y + markerRadius * sin(angle).toFloat()
        
        drawLine(
            color = Color.Gray.copy(alpha = 0.5f),
            start = Offset(startX, startY),
            end = Offset(endX, endY),
            strokeWidth = 1.dp.toPx(), // Also made thinner
            cap = StrokeCap.Round
        )
    }
}

/**
 * Status icon with smart coloring based on value freshness
 */
@Composable
fun UxStatusIcon(
    value: UxValue<String>,
    label: String,
    onIcon: @Composable () -> Unit,
    offIcon: @Composable () -> Unit,
    modifier: Modifier = Modifier
) {
    Column(
        modifier = modifier.padding(4.dp), // Reduced padding for grid layout
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        val iconColor = getValueColor(value)
        
        CompositionLocalProvider(
            LocalContentColor provides iconColor
        ) {
            if (value.isStale) {
                // Show grayed out "off" state for stale data
                CompositionLocalProvider(
                    LocalContentColor provides Color.Gray.copy(alpha = 0.5f)
                ) {
                    offIcon()
                }
            } else {
                val isOn = value.value == "1" || value.value.lowercase() == "true" || value.value.lowercase() == "on"
                if (isOn) onIcon() else offIcon()
            }
        }
        
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall.copy(fontSize = 10.sp), // Smaller text for grid
            color = iconColor,
            modifier = Modifier.padding(top = 2.dp)
        )
        
        if (value.isStale) {
            Text(
                text = "No Data",
                style = MaterialTheme.typography.bodySmall.copy(fontSize = 8.sp),
                color = Color.Gray.copy(alpha = 0.7f)
            )
        }
    }
}

/**
 * Grid cell that can contain either 1 gauge or up to 4 icons in sub-cells
 */
@Composable
fun UxGridCell(
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    Box(
        modifier = modifier
            .padding(4.dp)
            .fillMaxWidth(),
        contentAlignment = Alignment.Center
    ) {
        content()
    }
}

/**
 * Icon grid for displaying 4 icons in a 2x2 sub-grid within a cell
 */
@Composable
fun UxIconGrid(
    topLeft: @Composable (() -> Unit)? = null,
    topRight: @Composable (() -> Unit)? = null,
    bottomLeft: @Composable (() -> Unit)? = null,
    bottomRight: @Composable (() -> Unit)? = null,
    modifier: Modifier = Modifier
) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Row(
            horizontalArrangement = Arrangement.SpaceEvenly,
            modifier = Modifier.fillMaxWidth()
        ) {
            Box(modifier = Modifier.weight(1f), contentAlignment = Alignment.Center) {
                topLeft?.invoke()
            }
            Box(modifier = Modifier.weight(1f), contentAlignment = Alignment.Center) {
                topRight?.invoke()
            }
        }
        
        Row(
            horizontalArrangement = Arrangement.SpaceEvenly,
            modifier = Modifier.fillMaxWidth()
        ) {
            Box(modifier = Modifier.weight(1f), contentAlignment = Alignment.Center) {
                bottomLeft?.invoke()
            }
            Box(modifier = Modifier.weight(1f), contentAlignment = Alignment.Center) {
                bottomRight?.invoke()
            }
        }
    }
}

/**
 * Communication status hub with hierarchical icon logic
 */
@Composable
fun CommunicationStatusHub(
    communicationStatus: UxValue<String>,
    rssi: UxValue<String>,
    modifier: Modifier = Modifier
) {
    Card(
        modifier = modifier,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        )
    ) {
        Column(
            modifier = Modifier.padding(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = "Communication",
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.SemiBold
            )
            
            Spacer(modifier = Modifier.height(8.dp))
            
            // Smart communication icon
            val iconColor = getValueColor(communicationStatus)
            Icon(
                imageVector = when (communicationStatus.value) {
                    "Online" -> androidx.compose.material.icons.Icons.Filled.CheckCircle
                    "Connected" -> androidx.compose.material.icons.Icons.Filled.Warning
                    else -> androidx.compose.material.icons.Icons.Filled.Cancel
                },
                contentDescription = communicationStatus.value,
                tint = when (communicationStatus.value) {
                    "Online" -> if (communicationStatus.isFresh) Color.Green else Color.Green.copy(alpha = 0.6f)
                    "Connected" -> if (communicationStatus.isFresh) Color.Yellow else Color.Yellow.copy(alpha = 0.6f)
                    else -> Color.Red.copy(alpha = 0.6f)
                },
                modifier = Modifier.size(32.dp)
            )
            
            Text(
                text = communicationStatus.value,
                style = MaterialTheme.typography.bodyMedium,
                color = iconColor,
                fontWeight = FontWeight.Medium
            )
            
            // RSSI display
            if (!rssi.isStale) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier.padding(top = 4.dp)
                ) {
                    Icon(
                        imageVector = androidx.compose.material.icons.Icons.Filled.SignalCellularAlt,
                        contentDescription = "Signal Strength",
                        tint = getValueColor(rssi),
                        modifier = Modifier.size(16.dp)
                    )
                    Text(
                        text = "${rssi.value} dBm",
                        style = MaterialTheme.typography.bodySmall,
                        color = getValueColor(rssi),
                        modifier = Modifier.padding(start = 4.dp)
                    )
                }
            }
        }
    }
}

/**
 * Energy monitoring card with counters
 */
@Composable
fun EnergyMonitoringCard(
    dailyEnergy: UxValue<String>,
    totalEnergy: UxValue<String>,
    dailyWater: UxValue<String>,
    totalWater: UxValue<String>,
    dailyHours: UxValue<String>,
    totalHours: UxValue<String>,
    modifier: Modifier = Modifier
) {
    Card(
        modifier = modifier,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        )
    ) {
        Column(
            modifier = Modifier.padding(16.dp)
        ) {
            Text(
                text = "Energy Monitoring",
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(bottom = 12.dp)
            )
            
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    EnergyCounter("Daily Energy", dailyEnergy, "kWh")
                    EnergyCounter("Daily Water", dailyWater, "L")
                    EnergyCounter("Daily Hours", dailyHours, "hrs")
                }
                
                Column(modifier = Modifier.weight(1f)) {
                    EnergyCounter("Total Energy", totalEnergy, "kWh")
                    EnergyCounter("Total Water", totalWater, "L")
                    EnergyCounter("Total Hours", totalHours, "hrs")
                }
            }
        }
    }
}

@Composable
private fun EnergyCounter(
    label: String,
    value: UxValue<String>,
    unit: String
) {
    Column(
        modifier = Modifier.padding(vertical = 4.dp)
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall,
            color = getValueColor(value)
        )
        Text(
            text = if (value.isStale) "No Data" else "${value.value} $unit",
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium,
            color = getValueColor(value)
        )
    }
}

/**
 * Get color based on value freshness and staleness
 */
@Composable
private fun getValueColor(value: UxValue<String>): Color {
    return when {
        value.isStale -> Color.Gray.copy(alpha = 0.5f)
        value.isFresh -> MaterialTheme.colorScheme.onSurface
        else -> MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
    }
}

/**
 * Compact gauge for analog inputs with improved bottom alignment
 */
@Composable
fun UxCompactGauge(
    value: UxValue<String>,
    label: String,
    unit: String,
    minValue: Float = 0f,
    maxValue: Float = 100f,
    modifier: Modifier = Modifier,
    warningThreshold: Float? = null,
    criticalThreshold: Float? = null
) {
    val currentValue = value.value.toFloatOrNull() ?: 0f
    
    // Animated value for smooth transitions
    val animatedValue by animateFloatAsState(
        targetValue = currentValue,
        animationSpec = tween(
            durationMillis = 1000,
            easing = EaseInOutCubic
        ),
        label = "compact_gauge_value"
    )
    
    Column(
        modifier = modifier.padding(2.dp), // Minimal padding for compact size
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            modifier = Modifier.size(80.dp),
            contentAlignment = Alignment.BottomCenter // Better bottom alignment for compact gauge
        ) {
            Canvas(
                modifier = Modifier.fillMaxSize()
            ) {
                drawCompactSemiCircularGauge(
                    value = animatedValue,
                    minValue = minValue,
                    maxValue = maxValue,
                    isFresh = value.isFresh,
                    isStale = value.isStale,
                    warningThreshold = warningThreshold,
                    criticalThreshold = criticalThreshold
                )
            }
            
            // Value and unit text perfectly aligned to bottom of gauge
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                modifier = Modifier.padding(bottom = 4.dp) // Better alignment for smaller gauge
            ) {
                Text(
                    text = if (value.isStale) "No Data" else String.format("%.0f", animatedValue),
                    style = MaterialTheme.typography.titleMedium.copy(
                        fontWeight = FontWeight.Bold,
                        fontSize = 14.sp // Smaller for compact gauge
                    ),
                    color = getValueColor(value)
                )
                if (unit.isNotEmpty()) {
                    Text(
                        text = unit,
                        style = MaterialTheme.typography.bodySmall.copy(fontSize = 10.sp),
                        color = getValueColor(value).copy(alpha = 0.7f)
                    )
                }
            }
        }
        
        // Label below the gauge
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall.copy(fontSize = 10.sp),
            color = getValueColor(value),
            modifier = Modifier.padding(top = 2.dp)
        )
    }
}

/**
 * Draw compact semi-circular gauge for analog inputs
 */
private fun DrawScope.drawCompactSemiCircularGauge(
    value: Float,
    minValue: Float,
    maxValue: Float,
    isFresh: Boolean,
    isStale: Boolean,
    warningThreshold: Float?,
    criticalThreshold: Float?
) {
    val strokeWidth = 4.dp.toPx() // Thinner for compact gauge
    val radius = (size.minDimension - strokeWidth) / 2
    val center = Offset(size.width / 2, size.height - 4.dp.toPx()) // Better bottom alignment
    
    // Background arc
    val backgroundBrush = if (isStale) {
        SolidColor(Color.Gray.copy(alpha = 0.3f))
    } else {
        SolidColor(Color.Gray.copy(alpha = 0.2f))
    }
    
    drawArc(
        brush = backgroundBrush,
        startAngle = 180f,
        sweepAngle = 180f,
        useCenter = false,
        topLeft = Offset(center.x - radius, center.y - radius),
        size = Size(radius * 2, radius * 2),
        style = Stroke(width = strokeWidth, cap = StrokeCap.Round)
    )
    
    if (!isStale && value > minValue) {
        // Calculate value position
        val normalizedValue = ((value - minValue) / (maxValue - minValue)).coerceIn(0f, 1f)
        val sweepAngle = normalizedValue * 180f
        
        // Determine color based on thresholds and freshness
        val gaugeColor = when {
            criticalThreshold != null && value >= criticalThreshold -> Color.Red
            warningThreshold != null && value >= warningThreshold -> Color.Yellow
            else -> if (isFresh) Color.Green else Color.Blue
        }
        
        val alpha = if (isFresh) 1f else 0.6f
        
        // Value arc
        drawArc(
            color = gaugeColor.copy(alpha = alpha),
            startAngle = 180f,
            sweepAngle = sweepAngle,
            useCenter = false,
            topLeft = Offset(center.x - radius, center.y - radius),
            size = Size(radius * 2, radius * 2),
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round)
        )
        
        // Value indicator dot
        val angle = Math.toRadians((180 + sweepAngle).toDouble())
        val dotX = center.x + radius * cos(angle).toFloat()
        val dotY = center.y + radius * sin(angle).toFloat()
        
        drawCircle(
            color = gaugeColor.copy(alpha = alpha),
            radius = strokeWidth / 2,
            center = Offset(dotX, dotY)
        )
    }
    
    // Minimal scale markers for compact gauge
    drawCompactScaleMarkers(center, radius, strokeWidth)
}

/**
 * Draw minimal scale markers for compact gauge
 */
private fun DrawScope.drawCompactScaleMarkers(
    center: Offset,
    radius: Float,
    strokeWidth: Float
) {
    val markerCount = 3 // Fewer markers for compact gauge
    val markerLength = 1.5.dp.toPx() // Very small markers
    val markerRadius = radius + strokeWidth / 2 + markerLength
    
    for (i in 0..markerCount) {
        val angle = Math.toRadians((180 + (i * 180f / markerCount)).toDouble())
        val startX = center.x + radius * cos(angle).toFloat()
        val startY = center.y + radius * sin(angle).toFloat()
        val endX = center.x + markerRadius * cos(angle).toFloat()
        val endY = center.y + markerRadius * sin(angle).toFloat()
        
        drawLine(
            color = Color.Gray.copy(alpha = 0.4f),
            start = Offset(startX, startY),
            end = Offset(endX, endY),
            strokeWidth = 0.5.dp.toPx(), // Very thin markers
            cap = StrokeCap.Round
        )
    }
}
