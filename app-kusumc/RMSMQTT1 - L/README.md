# PMKUSUM IoT Demo Android App

A modern Android application built with Kotlin and Jetpack Compose for monitoring and controlling IoT devices in the PMKUSUM solar pump project. The app provides real-time data visualization, MQTT communication, and data export capabilities.

## Features

### 🏠 **Home Screen**
- App logo and version information
- Real-time MQTT connection status with pulse animation
- Clean, centered layout with app information

### 📊 **Dashboard Screen**
- **Heartbeat Tab**: Live device status and location data
- **Data Tab**: Real-time pump performance metrics
- **DAQ Tab**: Data acquisition system readings
- **On Demand Tab**: Pump control with ON/OFF commands and simulated responses

### 📁 **Raw Data Screen**
- View raw JSON messages for all data types
- Historical data with timestamps
- Tabbed interface for different data streams

### ⚙️ **Settings Screen**
- MQTT broker configuration (test.mosquitto.org pre-configured)
- Connection management with status indicators
- CSV data export functionality
- Automatic client ID generation for uniqueness

## Technical Architecture

### **Technologies Used**
- **Language**: Kotlin
- **UI Framework**: Jetpack Compose with Material Design 3
- **MQTT Client**: HiveMQ MQTT Client with custom IPv4 DNS resolution
- **JSON Parsing**: kotlinx.serialization
- **Architecture**: MVVM with ViewModels and StateFlow
- **Background Processing**: Foreground Service for persistent MQTT connection

### **App Structure**
```
├── data/                    # Data classes and parameter mappings
│   ├── HeartbeatData.kt    # Heartbeat JSON structure
│   ├── PumpData.kt         # Pump data JSON structure
│   ├── DaqData.kt          # DAQ data JSON structure
│   ├── OnDemandData.kt     # Command/response structures
│   └── ParameterMappings.kt # Field name to display name mappings
├── service/
│   └── MqttService.kt      # Background MQTT service
├── viewmodel/              # ViewModels for each screen
├── ui/
│   ├── screens/            # Compose screens
│   ├── components/         # Reusable UI components
│   ├── theme/              # Dark theme configuration
│   └── navigation/         # Navigation structure
└── utils/
    ├── CsvExportUtil.kt    # CSV export functionality
    └── DemoDataGenerator.kt # Demo data for testing
```

## MQTT Configuration

### **Topics**
- **Heartbeat**: `{IMEI}/heartbeat` (VD=0)
- **Pump Data**: `{IMEI}/data` (VD=1)
- **DAQ Data**: `{IMEI}/daq` (VD=12)
- **Commands**: `{IMEI}/ondemand` (Publish)

### **Demo IMEI**: `869630050762180`

### **Default Broker Settings**
- **URL**: test.mosquitto.org
- **Port**: 1883
- **Username**: (empty)
- **Password**: (empty)
- **Client ID**: Auto-generated as `NEReceiver554_{timestamp}`

## Data Export

The app supports exporting collected data as CSV files:
- **Heartbeat Data**: Device status, location, connectivity
- **Pump Data**: Performance metrics, energy generation
- **DAQ Data**: Analog/digital inputs and outputs
- **Time Range**: All data or filtered by date range (feature ready)

## Setup Instructions

### **Prerequisites**
- Android Studio Arctic Fox or later
- Android SDK 24 (Android 7.0) or higher
- Kotlin 2.0.21+

### **Installation**
1. Clone the repository
2. Open in Android Studio
3. Sync Gradle dependencies
4. Build and run on device/emulator

### **Permissions**
The app requires the following permissions:
- `INTERNET` - MQTT communication
- `ACCESS_NETWORK_STATE` - Network status monitoring
- `WAKE_LOCK` - Maintain connection while screen off
- `FOREGROUND_SERVICE` - Background MQTT service
- `POST_NOTIFICATIONS` - Error notifications

## Usage

### **Getting Started**
1. Open the app
2. Navigate to Settings
3. Configure MQTT broker (default settings work with test.mosquitto.org)
4. Tap "Connect" to establish MQTT connection
5. Navigate to Dashboard to view live data
6. Use Raw Data screen to see historical messages
7. Export data as CSV from Settings screen

### **Testing with Demo Data**
The app includes a demo data generator for testing without a real MQTT broker. This can be activated in development builds.

### **Pump Control**
1. Go to Dashboard → On Demand tab
2. Use "Turn ON" or "Turn OFF" buttons
3. View simulated response in the "Latest Command Status" section

## JSON Data Structures

### **Heartbeat (VD=0)**
Device status, location, connectivity information

### **Pump Data (VD=1)**
Energy generation, water discharge, pump status

### **DAQ Data (VD=12)**
Analog inputs, digital I/O, system parameters

### **On Demand Commands**
Pump control commands with timestamp and authentication

## Error Handling

- Network connection errors shown in notifications
- JSON parsing errors logged and displayed
- MQTT connection status continuously monitored
- Graceful handling of malformed data

## Performance Optimizations

- Background service for persistent MQTT connection
- Efficient JSON parsing with kotlinx.serialization
- StateFlow for reactive UI updates
- Custom IPv4 DNS resolution for better connectivity
- Memory-efficient data storage

## Future Enhancements

- Date range filtering for data export
- Real-time charts and graphs
- Push notifications for critical alerts
- Multi-device support
- Offline data caching
- Advanced pump scheduling

## License

This project is developed for the PMKUSUM IoT demonstration and is intended for educational and testing purposes.

## Support

For technical support or questions about the PMKUSUM IoT project, please contact the development team.

---

**Version**: 1.0.0 (Demo)  
**Last Updated**: July 2025  
**Minimum Android Version**: 7.0 (API 24)  
**Target Android Version**: 15 (API 36)
