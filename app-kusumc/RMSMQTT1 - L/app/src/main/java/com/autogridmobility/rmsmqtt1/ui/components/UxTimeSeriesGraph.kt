package com.autogridmobility.rmsmqtt1.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTransformGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.*
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.IntOffset
import com.autogridmobility.rmsmqtt1.viewmodel.DataPoint
import kotlin.math.max
import kotlin.math.abs
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import android.graphics.Paint as AndroidPaint
import android.graphics.Typeface as AndroidTypeface

/**
 * Simple zoomable/pannable time-series graph for Compose using Canvas.
 * - Input: list of DataPoint(timestampMillis, value)
 * - Supports pinch zoom and pan gestures
 */
@Composable
fun UxTimeSeriesGraph(
    points: List<DataPoint>,
    modifier: Modifier = Modifier,
    label: String = "Frequency (Hz)",
    lineColor: Color = Color(0xFF0288D1),
    height: androidx.compose.ui.unit.Dp = 180.dp,
    showAxisTicks: Boolean = true,
    showYAxisTicks: Boolean = true,
    showTooltip: Boolean = true,
    maxDisplayPoints: Int = 1000,
    paused: Boolean = false,
    onTogglePause: (() -> Unit)? = null,
    adaptiveColor: Boolean = true,
    stale: Boolean = false
) {
    // Local state for transform
    var scale by remember { mutableStateOf(1f) }
    var translateX by remember { mutableStateOf(0f) }
    var activePointer by remember { mutableStateOf<Offset?>(null) }

    // Pre-process (downsample if needed)
    val baseColor = if (adaptiveColor) parameterColor(label) else lineColor
    val workingPoints = if (paused) remember { points } else points
    val sortedPoints = remember(workingPoints) { workingPoints.sortedBy { it.t } }
    val displayPoints = remember(sortedPoints) {
        if (sortedPoints.size <= maxDisplayPoints) sortedPoints else {
            val stride = kotlin.math.ceil(sortedPoints.size / maxDisplayPoints.toFloat()).toInt().coerceAtLeast(1)
            val reduced = ArrayList<DataPoint>(maxDisplayPoints + 2)
            for (i in sortedPoints.indices step stride) reduced.add(sortedPoints[i])
            if (reduced.last().t != sortedPoints.last().t) reduced.add(sortedPoints.last())
            reduced
        }
    }

    // Tooltip selection state
    var selectedPoint by remember { mutableStateOf<DataPoint?>(null) }
    var selectedOffsetX by remember { mutableStateOf<Float?>(null) }

    val timeFormatter = remember { DateTimeFormatter.ofPattern("HH:mm:ss") }

    Card(
        modifier = modifier,
        colors = CardDefaults.cardColors(containerColor = androidx.compose.material3.MaterialTheme.colorScheme.surface)
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween, modifier = Modifier.fillMaxWidth()) {
                Text(text = label, fontWeight = FontWeight.SemiBold)
                if (onTogglePause != null) {
                    IconButton(onClick = { onTogglePause() }) {
                        Icon(if (paused) Icons.Filled.PlayArrow else Icons.Filled.Pause, contentDescription = if (paused) "Resume" else "Pause")
                    }
                }
            }
            Spacer(modifier = Modifier.height(8.dp))

            Box(modifier = Modifier
                .fillMaxWidth()
                .height(height)
                .background(androidx.compose.material3.MaterialTheme.colorScheme.background)
            ) {
                Canvas(modifier = Modifier
                    .fillMaxSize()
                    .pointerInput(Unit) {
                        detectTransformGestures { centroid, pan, zoom, rotation ->
                            // Apply zoom at centroid
                            val oldScale = scale
                            scale = (scale * zoom).coerceIn(0.5f, 5f)
                            // Adjust translate to zoom around centroid
                            translateX += pan.x
                        }
                    }
                    .pointerInput(Unit) {
                        detectDragGestures { change, dragAmount ->
                            change.consume()
                            translateX += dragAmount.x
                        }
                    }
                    .pointerInput(showTooltip) {
                        if (showTooltip) {
                            detectTapGestures { offset ->
                                // selection handled inside draw with accurate mapping; store raw x
                                selectedOffsetX = offset.x
                                selectedPoint = mapXToNearestPoint(offset.x, size.width.toFloat(), displayPoints, translateX, scale)
                            }
                        }
                    }
                    .pointerInput(showTooltip) {
                        if (showTooltip) {
                            detectDragGestures(
                                onDragStart = { offset ->
                                    selectedOffsetX = offset.x
                                    selectedPoint = mapXToNearestPoint(offset.x, size.width.toFloat(), displayPoints, translateX, scale)
                                },
                                onDrag = { change, _ ->
                                    selectedOffsetX = change.position.x
                                    selectedPoint = mapXToNearestPoint(change.position.x, size.width.toFloat(), displayPoints, translateX, scale)
                                }
                            )
                        }
                    }
                ) {
                    val w = size.width
                    val h = size.height
                    if (displayPoints.isEmpty()) return@Canvas

                    val minT = displayPoints.first().t
                    val maxT = displayPoints.last().t
                    val tRange = max(1L, (maxT - minT))

                    val minV = displayPoints.minOf { it.v }
                    val maxV = displayPoints.maxOf { it.v }
                    val vRange = max(1f, maxV - minV)

                    val axisHeightPx = if (showAxisTicks) 20.dp.toPx() else 0f
                    val yAxisWidthPx = if (showYAxisTicks) 36.dp.toPx() else 0f
                    val contentHeight = h - axisHeightPx
                    val contentWidth = w - yAxisWidthPx

                    // Map time to x with translate & scale
                    fun tx(t: Long): Float {
                        val normalized = (t - minT).toFloat() / tRange
                        return yAxisWidthPx + (normalized * contentWidth * scale) + translateX
                    }

                    fun ty(v: Float): Float {
                        val normalized = (v - minV) / vRange
                        return contentHeight - normalized * contentHeight
                    }

                    // Draw grid lines
                    val gridPaintColor = androidx.compose.ui.graphics.Color.Gray.copy(alpha = 0.12f)
                    for (i in 0..4) {
                        val y = i.toFloat() * h / 4f
                        drawLine(gridPaintColor, Offset(0f, y), Offset(w, y))
                    }

                    // Build path
                    val path = Path()
                    displayPoints.forEachIndexed { index, p ->
                        val x = tx(p.t)
                        val y = ty(p.v)
                        if (index == 0) path.moveTo(x, y) else path.lineTo(x, y)
                    }

                    // Draw path with stroke
                    drawPath(path = path, color = baseColor, style = Stroke(width = 2.dp.toPx()))

                    // Axis ticks & labels
                    if (showAxisTicks) {
                        val tickCount = 4
                        val paint = AndroidPaint().apply {
                            color = android.graphics.Color.GRAY
                            textSize = 10.dp.toPx()
                            isAntiAlias = true
                            typeface = AndroidTypeface.MONOSPACE
                        }
                        val baselineY = contentHeight
                        // baseline
                        drawLine(Color.Gray.copy(alpha = 0.4f), Offset(yAxisWidthPx, baselineY), Offset(w, baselineY))
            drawIntoCanvas { cnv ->
                            for (i in 0..tickCount) {
                                val frac = i / tickCount.toFloat()
                                val tTick = (minT + (tRange * frac)).toLong()
                                val x = tx(tTick)
                                // tick mark
                                drawLine(Color.Gray.copy(alpha = 0.5f), Offset(x, baselineY), Offset(x, baselineY + 4f))
                                val instant = Instant.ofEpochMilli(tTick)
                                val label = timeFormatter.format(instant.atZone(ZoneId.systemDefault()))
                                val textWidth = paint.measureText(label)
                                val txPos = (x - textWidth / 2f).coerceIn(0f, w - textWidth)
                                cnv.nativeCanvas.drawText(label, txPos, baselineY + paint.textSize + 4f, paint)
                            }
                        }
                    }

                    // Y-axis ticks & labels
                    if (showYAxisTicks) {
                        val yTickCount = 4
                        val paintY = AndroidPaint().apply {
                            color = android.graphics.Color.GRAY
                            textSize = 10.dp.toPx()
                            isAntiAlias = true
                            typeface = AndroidTypeface.MONOSPACE
                        }
                        for (i in 0..yTickCount) {
                            val frac = i / yTickCount.toFloat()
                            val v = minV + (vRange * (1 - frac))
                            val y = ty(v)
                            drawLine(Color.Gray.copy(alpha = 0.3f), Offset(yAxisWidthPx - 4f, y), Offset(yAxisWidthPx, y))
                            drawIntoCanvas { cnv ->
                                val label = String.format("%.2f", v)
                                val textWidth = paintY.measureText(label)
                                cnv.nativeCanvas.drawText(label, (yAxisWidthPx - 6f - textWidth), y + (paintY.textSize / 3f), paintY)
                            }
                        }
                    }

                    // Tooltip marker
                    if (showTooltip && selectedPoint != null && selectedOffsetX != null) {
                        val sp = selectedPoint!!
                        val sx = tx(sp.t)
                        val sy = ty(sp.v)
                        // vertical line
                        drawLine(baseColor.copy(alpha = 0.4f), Offset(sx, 0f), Offset(sx, contentHeight), strokeWidth = 1.dp.toPx())
                        // point marker
                        drawCircle(baseColor, radius = 5.dp.toPx(), center = Offset(sx, sy))
                    }

                    // Stale overlay
                    if (stale) {
                        drawRect(Color.Black.copy(alpha = 0.25f))
                        drawIntoCanvas { cnv ->
                            val paint = AndroidPaint().apply {
                                color = android.graphics.Color.WHITE
                                textSize = 18.dp.toPx()
                                isAntiAlias = true
                                typeface = AndroidTypeface.DEFAULT_BOLD
                            }
                            val text = "STALE"
                            val tw = paint.measureText(text)
                            cnv.nativeCanvas.drawText(text, (w - tw) / 2f, h / 2f, paint)
                        }
                    }
                }

                // Overlay tooltip box (Compose) after Canvas if selection exists
                if (showTooltip && selectedPoint != null && selectedOffsetX != null) {
                    val sp = selectedPoint!!
                    val instant = Instant.ofEpochMilli(sp.t)
                    val timeStr = timeFormatter.format(instant.atZone(ZoneId.systemDefault()))
                    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.TopStart) {
                        val xOffset = selectedOffsetX!!.coerceAtLeast(0f)
                        Card(
                            modifier = Modifier
                                .padding(4.dp)
                                .offset { IntOffset(xOffset.toInt().coerceAtLeast(0), 0) }
                        ) {
                            Column(Modifier.padding(6.dp)) {
                                Text(timeStr, style = androidx.compose.material3.MaterialTheme.typography.labelSmall)
                                Text(sp.v.toString(), style = androidx.compose.material3.MaterialTheme.typography.bodySmall, fontWeight = FontWeight.Bold)
                            }
                        }
                    }
                }
            }
        }
    }
}

// Inline function (not composable) used in gesture handlers
private fun mapXToNearestPoint(
    tapX: Float,
    canvasWidth: Float,
    displayPoints: List<DataPoint>,
    translateX: Float,
    scale: Float
): DataPoint? {
    if (displayPoints.isEmpty()) return null
    val minT = displayPoints.first().t
    val maxT = displayPoints.last().t
    val tRange = max(1L, (maxT - minT))
    // Reverse transform: (tapX - translateX) / (contentWidth * scale)
    val normalized = ((tapX - translateX) / (canvasWidth * scale)).coerceIn(0f, 1f)
    val targetT = (minT + tRange * normalized).toLong()
    var nearest: DataPoint? = null
    var bestDist = Long.MAX_VALUE
    for (p in displayPoints) {
        val d = abs(p.t - targetT)
        if (d < bestDist) { bestDist = d; nearest = p }
    }
    return nearest
}

// Multi-series overlay graph (two series max for now)
@Composable
fun UxMultiSeriesGraph(
    seriesA: List<DataPoint>,
    seriesB: List<DataPoint>,
    labelA: String,
    labelB: String,
    modifier: Modifier = Modifier,
    height: androidx.compose.ui.unit.Dp = 220.dp,
    showAxisTicks: Boolean = true,
    showYAxisTicks: Boolean = true,
    showTooltip: Boolean = true
) {
    val allPoints = remember(seriesA, seriesB) { (seriesA + seriesB).sortedBy { it.t } }
    var scale by remember { mutableStateOf(1f) }
    var translateX by remember { mutableStateOf(0f) }
    var selectedPointA by remember { mutableStateOf<DataPoint?>(null) }
    var selectedPointB by remember { mutableStateOf<DataPoint?>(null) }
    var selectedTime by remember { mutableStateOf<Long?>(null) }
    var selectedOffsetX by remember { mutableStateOf<Float?>(null) }
    val timeFormatter = remember { DateTimeFormatter.ofPattern("HH:mm:ss") }
    val colorA = parameterColor(labelA)
    val colorB = parameterColor(labelB)

    Card(modifier = modifier, colors = CardDefaults.cardColors(containerColor = androidx.compose.material3.MaterialTheme.colorScheme.surface)) {
        Column(Modifier.padding(12.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text("$labelA + $labelB", fontWeight = FontWeight.SemiBold)
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    LegendSwatch(colorA, labelA)
                    LegendSwatch(colorB, labelB)
                }
            }

            Spacer(Modifier.height(8.dp))

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(height)
                    .background(androidx.compose.material3.MaterialTheme.colorScheme.background)
            ) {
                if (allPoints.isNotEmpty()) {
                    Canvas(
                        modifier = Modifier
                            .fillMaxSize()
                            .pointerInput(Unit) {
                                detectTransformGestures { _, pan, zoom, _ ->
                                    scale = (scale * zoom).coerceIn(0.5f, 5f)
                                    translateX += pan.x
                                }
                            }
                            .pointerInput(showTooltip) {
                                if (showTooltip) {
                                    detectTapGestures { offset ->
                                        selectedOffsetX = offset.x
                                        val nearest = multiBinarySearch(offset.x, size.width.toFloat(), allPoints, translateX, scale)
                                        selectedTime = nearest?.t
                                        selectedPointA = nearestByTime(seriesA, nearest?.t ?: 0L)
                                        selectedPointB = nearestByTime(seriesB, nearest?.t ?: 0L)
                                    }
                                }
                            }
                            .pointerInput(showTooltip) {
                                if (showTooltip) {
                                    detectDragGestures(
                                        onDragStart = { offset ->
                                            selectedOffsetX = offset.x
                                            val nearest = multiBinarySearch(offset.x, size.width.toFloat(), allPoints, translateX, scale)
                                            selectedTime = nearest?.t
                                            selectedPointA = nearestByTime(seriesA, nearest?.t ?: 0L)
                                            selectedPointB = nearestByTime(seriesB, nearest?.t ?: 0L)
                                        },
                                        onDrag = { change, _ ->
                                            selectedOffsetX = change.position.x
                                            val nearest = multiBinarySearch(change.position.x, size.width.toFloat(), allPoints, translateX, scale)
                                            selectedTime = nearest?.t
                                            selectedPointA = nearestByTime(seriesA, nearest?.t ?: 0L)
                                            selectedPointB = nearestByTime(seriesB, nearest?.t ?: 0L)
                                        }
                                    )
                                }
                            }
                    ) {
                        val w = size.width
                        val h = size.height
                        val minT = allPoints.first().t
                        val maxT = allPoints.last().t
                        val tRange = max(1L, (maxT - minT))
                        val minV = allPoints.minOf { it.v }
                        val maxV = allPoints.maxOf { it.v }
                        val vRange = max(1f, maxV - minV)
                        val axisHeightPx = if (showAxisTicks) 20.dp.toPx() else 0f
                        val yAxisWidthPx = if (showYAxisTicks) 36.dp.toPx() else 0f
                        val contentHeight = h - axisHeightPx
                        val contentWidth = w - yAxisWidthPx

                        fun tx(t: Long): Float {
                            val normalized = (t - minT).toFloat() / tRange
                            return yAxisWidthPx + (normalized * contentWidth * scale) + translateX
                        }

                        fun ty(v: Float): Float {
                            val normalized = (v - minV) / vRange
                            return contentHeight - normalized * contentHeight
                        }

                        fun drawSeries(points: List<DataPoint>, color: Color) {
                            if (points.isEmpty()) return
                            val path = Path()
                            points.forEachIndexed { i, p ->
                                val x = tx(p.t)
                                val y = ty(p.v)
                                if (i == 0) path.moveTo(x, y) else path.lineTo(x, y)
                            }
                            drawPath(path, color, style = Stroke(width = 2.dp.toPx()))
                        }

                        drawSeries(seriesA, colorA)
                        drawSeries(seriesB, colorB.copy(alpha = 0.85f))

                        if (showAxisTicks) {
                            val baselineY = contentHeight
                            drawLine(Color.Gray.copy(alpha = 0.4f), Offset(yAxisWidthPx, baselineY), Offset(w, baselineY))
                        }

                        if (showTooltip && selectedTime != null && selectedOffsetX != null) {
                            val sx = tx(selectedTime!!)
                            drawLine(Color.White.copy(alpha = 0.5f), Offset(sx, 0f), Offset(sx, contentHeight))
                            selectedPointA?.let { sa ->
                                drawCircle(colorA, radius = 5.dp.toPx(), center = Offset(sx, ty(sa.v)))
                            }
                            selectedPointB?.let { sb ->
                                drawCircle(colorB, radius = 5.dp.toPx(), center = Offset(sx, ty(sb.v)))
                            }
                        }
                    }
                }

                if (showTooltip && selectedTime != null && selectedOffsetX != null) {
                    val instant = Instant.ofEpochMilli(selectedTime!!)
                    val timeStr = timeFormatter.format(instant.atZone(ZoneId.systemDefault()))
                    val valA = selectedPointA?.v
                    val valB = selectedPointB?.v
                    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.TopStart) {
                        Card(
                            Modifier
                                .padding(4.dp)
                                .offset { IntOffset(selectedOffsetX!!.toInt().coerceAtLeast(0), 0) }
                        ) {
                            Column(Modifier.padding(6.dp)) {
                                Text(timeStr, style = androidx.compose.material3.MaterialTheme.typography.labelSmall)
                                if (valA != null) Text("$labelA: ${"%.2f".format(valA)}", color = colorA, fontWeight = FontWeight.Bold)
                                if (valB != null) Text("$labelB: ${"%.2f".format(valB)}", color = colorB, fontWeight = FontWeight.Bold)
                            }
                        }
                    }
                }
            }
        }
    }
}

private fun multiBinarySearch(
    tapX: Float,
    width: Float,
    points: List<DataPoint>,
    translateX: Float,
    scale: Float
): DataPoint? {
    if (points.isEmpty()) return null
    val minT = points.first().t
    val maxT = points.last().t
    val tRange = max(1L, maxT - minT)
    val norm = ((tapX - translateX) / (width * scale)).coerceIn(0f, 1f)
    val targetT = (minT + tRange * norm).toLong()
    var lo = 0
    var hi = points.lastIndex
    while (lo <= hi) {
        val mid = (lo + hi) ushr 1
        val mt = points[mid].t
        if (mt < targetT) lo = mid + 1 else if (mt > targetT) hi = mid - 1 else return points[mid]
    }
    // choose nearest between hi and lo
    val candA = points.getOrNull(hi)
    val candB = points.getOrNull(lo)
    return when {
        candA == null -> candB
        candB == null -> candA
        else -> if (abs(candA.t - targetT) <= abs(candB.t - targetT)) candA else candB
    }
}

private fun nearestByTime(points: List<DataPoint>, t: Long): DataPoint? {
    if (points.isEmpty()) return null
    var lo = 0
    var hi = points.lastIndex
    while (lo <= hi) {
        val mid = (lo + hi) ushr 1
        val mt = points[mid].t
        if (mt < t) lo = mid + 1 else if (mt > t) hi = mid - 1 else return points[mid]
    }
    val candA = points.getOrNull(hi)
    val candB = points.getOrNull(lo)
    return when {
        candA == null -> candB
        candB == null -> candA
        else -> if (abs(candA.t - t) <= abs(candB.t - t)) candA else candB
    }
}

@Composable
private fun LegendSwatch(color: Color, label: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        Box(Modifier.size(12.dp).background(color))
        Text(label, style = androidx.compose.material3.MaterialTheme.typography.labelSmall)
    }
}

private fun parameterColor(label: String): Color = when {
    label.contains("Freq", true) -> Color(0xFF0288D1)
    label.contains("Power", true) -> Color(0xFFF57C00)
    label.contains("Flow", true) -> Color(0xFF00796B)
    label.contains("Current", true) -> Color(0xFF6A1B9A)
    label.contains("Volt", true) -> Color(0xFF512DA8)
    label.contains("Temp", true) -> Color(0xFFD32F2F)
    label.startsWith("AI", true) -> Color(0xFF455A64)
    else -> Color(0xFF00838F)
}
