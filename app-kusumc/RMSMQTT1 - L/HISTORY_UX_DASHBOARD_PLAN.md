# UX Dashboard History Replay Planning

## Overview
This document outlines the plan for implementing a **History UX Dashboard** that can replay historical data stored in the database, providing users with the ability to visualize past system performance using the same UX Dashboard interface.

## Architecture Plan

### **1. Data Storage Layer**
- **Current**: Raw MQTT packets stored as JSON strings with timestamps
- **Enhancement Needed**: Structured data storage with indexed timestamps for efficient querying
- **Database Schema**:
  ```sql
  CREATE TABLE historical_packets (
      id INTEGER PRIMARY KEY,
      imei TEXT NOT NULL,
      packet_type TEXT NOT NULL, -- 'heartbeat', 'pump', 'daq'
      timestamp DATETIME NOT NULL,
      json_data TEXT NOT NULL,
      INDEX(imei, timestamp),
      INDEX(packet_type, timestamp)
  );
  ```

### **2. History Data Service**
**File**: `HistoryDataService.kt`
- **Purpose**: Fetch historical packets from database with time range queries
- **Key Methods**:
  - `getHistoricalData(imei: String, startTime: DateTime, endTime: DateTime): List<TimestampedPacket>`
  - `getDataForTimeWindow(imei: String, targetTime: DateTime, windowSeconds: Int): SynchronizedData`
  - `getAvailableDateRanges(imei: String): List<DateRange>`

### **3. History UX ViewModel**
**File**: `HistoryUxViewModel.kt`
- **Purpose**: Replay historical data at controlled speeds (1 packet/second)
- **Key Features**:
  - **Data Source**: Database instead of live MQTT
  - **Replay Control**: Play, Pause, Stop, Speed Control (1x, 2x, 5x, 10x)
  - **Time Navigation**: Jump to specific time, Date picker
  - **Progress Tracking**: Current position in timeline

**Replay Logic**:
```kotlin
class HistoryReplayController {
    private val replaySpeed = 1.0 // packets per second
    
    fun startReplay(dateRange: DateRange) {
        // Fetch all packets for date range
        // Create timeline of synchronized data points
        // Start coroutine to emit data at replaySpeed
    }
    
    fun setReplaySpeed(multiplier: Float) {
        // Adjust emission interval: baseInterval / multiplier
    }
}
```

### **4. UI Component Reuse**
**Strategy**: 99% code reuse from UX Dashboard
- **Shared Components**: 
  - All gauge components
  - Status icon components  
  - Layout components
- **Differences**:
  - Additional time controls (play/pause/speed)
  - Timeline scrubber
  - Date range picker
  - "REPLAY MODE" indicator

### **5. Navigation Structure**
```
Side Menu:
├── Home
├── UX-Dash (Live)
├── History-UX (Replay) ← New item
├── Dashboard (Text)
├── Settings & Export
```

## Implementation Phases

### **Phase 1: Data Storage Enhancement**
- **Database Migration**: Add structured tables for efficient historical queries
- **Data Retention Policy**: Configure retention period (30 days, 90 days, etc.)
- **Indexing Strategy**: Optimize for time-range queries

### **Phase 2: History Service Layer**
- **HistoryDataService**: Database query service
- **Data Synchronization**: Apply same ±12 second sync logic to historical data
- **Caching Strategy**: Cache frequently accessed time periods

### **Phase 3: Replay Engine**
- **HistoryUxViewModel**: Replay controller with timing precision
- **Timeline Management**: Calculate synchronized data points from historical packets
- **Memory Optimization**: Stream data for long time periods

### **Phase 4: UI Implementation**
- **Copy UX Dashboard**: Clone existing UX Dashboard screen
- **Add Replay Controls**: Play/pause/speed/timeline controls
- **Time Navigation**: Date picker and timeline scrubber
- **Visual Indicators**: Clear distinction between live and replay modes

## Technical Specifications

### **Replay Performance Calculations**
- **Example Daily Data**: 16 packets/hour × 8 hours = 128 packets/day
- **Replay Duration**: 128 seconds at 1 packet/second = ~2 minutes
- **Memory Usage**: ~128 KB for one day's synchronized data points
- **Database Query**: Single query to fetch day's data, then in-memory processing

### **Replay Speed Options**
- **1x**: Real-time replay (1 packet/second)
- **2x**: 2 packets/second (64 seconds for full day)
- **5x**: 5 packets/second (25.6 seconds for full day)
- **10x**: 10 packets/second (12.8 seconds for full day)

### **Data Synchronization in Replay**
```kotlin
data class HistoricalSyncPoint(
    val timestamp: LocalDateTime,
    val syncedData: UxDashboardData,
    val availablePacketTypes: Set<String> // track which packets contributed
)
```

## User Experience Design

### **History UX Dashboard Screen**
```
┌─────────────────────────────────────┐
│ 🔄 REPLAY MODE - 2024-08-04        │
├─────────────────────────────────────┤
│                                     │
│     [Same UX Dashboard Layout]      │
│     [All gauges and status icons]   │
│                                     │
├─────────────────────────────────────┤
│ ⏮️ ⏯️ ⏭️  [Timeline Scrubber]    │
│ Speed: [1x] [2x] [5x] [10x]         │
│ Time: 10:30:15 / 18:45:30           │
└─────────────────────────────────────┘
```

### **Timeline Controls**
- **Play/Pause Button**: Start/stop replay
- **Speed Selector**: 1x to 10x replay speed
- **Timeline Scrubber**: Jump to any time point
- **Time Display**: Current time / Total duration
- **Date Picker**: Select different days

## Future Enhancements

### **Advanced Features** (Later Versions)
1. **Export Replay**: Save replay as video or animated GIF
2. **Comparison Mode**: Compare two different time periods side-by-side
3. **Event Markers**: Highlight significant events (alarms, state changes)
4. **Data Analysis**: Show trends, averages, min/max over time periods
5. **Remote Sync**: Download historical data from cloud storage

### **Performance Optimizations**
1. **Background Preloading**: Pre-calculate sync points for faster replay
2. **Compression**: Compress historical data for storage efficiency
3. **Progressive Loading**: Load data as user navigates timeline
4. **Smart Caching**: Cache recently viewed time periods

## Implementation Timeline

**Estimated Development**: 2-3 weeks after UX Dashboard completion

**Dependencies**:
- ✅ UX Dashboard (must be completed first)
- ✅ Database migration capability
- ✅ Historical data collection (already working)

**Deliverables**:
1. Database schema migration
2. HistoryDataService implementation
3. HistoryUxViewModel with replay engine
4. History UX Dashboard screen
5. Navigation integration
6. Testing with real historical data

This planning ensures we can efficiently implement the History UX Dashboard by reusing the majority of the UX Dashboard code while adding powerful replay capabilities for historical data analysis.
