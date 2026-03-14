# PMKUSUM IoT Monitor - Web Mockup

## Overview
This is a web-based mockup that replicates all the functions and user flow of the PMKUSUM IoT Android app. It provides a complete testing environment for the system without requiring an Android device.

## Features Replicated

### 🏠 **Home Screen**
- ✅ App logo and version information
- ✅ Real-time MQTT connection status with pulse animation
- ✅ Clean, centered layout with app information

### 📊 **UX Dashboard Screen**
- ✅ Advanced real-time monitoring with intelligent data correlation
- ✅ Semi-circular animated gauges for key parameters
- ✅ Communication Hub (RSSI, Device IMEI)
- ✅ Power & Battery System (Battery Voltage, Temperature gauges)
- ✅ Pump Operations (6 main gauges: Frequency, Power, Flow Rate, Current)
- ✅ Energy Monitoring (Daily/Total Energy, Water tracking)
- ✅ Digital I/O Matrix (4 Analog Inputs + Digital Input/Output indicators)

### 📊 **Dashboard Screen (4 Tabs)**
- ✅ **Heartbeat Tab**: Live device status, GPS location, battery voltage, network connectivity
- ✅ **Data Tab**: Real-time pump performance metrics, energy generation, water discharge
- ✅ **DAQ Tab**: Data acquisition system readings (analog/digital inputs/outputs)
- ✅ **On Demand Tab**: Pump control with ON/OFF commands and simulated responses

### 📁 **Raw Data Screen**
- ✅ View raw JSON messages for all data types
- ✅ Historical data with timestamps
- ✅ Tabbed interface (Heartbeat, Data, DAQ)
- ✅ Newest-first chronological display

### ⚙️ **Settings Screen**
- ✅ MQTT broker configuration (pre-configured for test.mosquitto.org)
- ✅ Connection management with status indicators
- ✅ Data simulation controls for testing
- ✅ CSV data export functionality
- ✅ Automatic client ID generation

## Technical Implementation

### **Technologies Used**
- **HTML5**: Structure and semantic markup
- **CSS3**: Dark theme styling with Material Design 3 principles
- **Vanilla JavaScript**: All functionality without external dependencies
- **Canvas API**: Animated semi-circular gauges
- **Local Storage**: State persistence (can be added)

### **Data Simulation**
- ✅ Generates realistic sample data for all packet types
- ✅ Configurable sending intervals (1-60 seconds)
- ✅ Batch publishing (10 packets per type per cycle, like Android app)
- ✅ Background data generation when connected
- ✅ Real-time packet counter

### **MQTT Simulation**
- ✅ Connection/disconnection states
- ✅ Broker configuration (URL, port, credentials)
- ✅ Auto-generated client IDs
- ✅ Connection status indicators throughout UI

### **Data Management**
- ✅ Stores last 100 entries per data type
- ✅ Real-time updates across all screens
- ✅ JSON data structures matching Android app
- ✅ Parameter mappings for display names

### **User Interface**
- ✅ **Dark Theme**: Consistent with Android app
- ✅ **Navigation Drawer**: Slide-out menu with same items
- ✅ **Tabbed Navigation**: Bottom tabs for Dashboard and Raw Data
- ✅ **Responsive Design**: Works on desktop and mobile
- ✅ **Toast Notifications**: Status messages and feedback
- ✅ **Animated Elements**: Pulse indicators, gauge animations

## Usage Instructions

### **Getting Started**
1. **Open `index.html`** in any modern web browser
2. **Navigate using the hamburger menu** (top-left)
3. **Go to Settings** to configure MQTT connection
4. **Click "Connect"** to simulate MQTT broker connection
5. **Start Data Simulation** to generate test data
6. **Explore all screens** to see live data updates

### **Testing Workflow**
1. **Home Screen**: Check connection status indicator
2. **Settings**: 
   - Configure MQTT settings
   - Connect to broker
   - Start data simulation with desired interval
3. **UX Dashboard**: View advanced gauges and parameter groupings
4. **Dashboard**: Navigate through all 4 tabs to see data tables
5. **Raw Data**: View JSON messages in chronological order
6. **Pump Control**: Test ON/OFF commands in Dashboard → On Demand tab
7. **Data Export**: Export collected data as CSV files

### **Data Simulation Testing**
- Set interval to **5 seconds** for realistic testing
- Each cycle publishes **30 packets** (10 heartbeat + 10 pump + 10 DAQ)
- Watch packet counter increase
- Data appears across all screens simultaneously
- Stop/start simulation to test state changes

### **Export Testing**
- Collect data by running simulation
- Go to Settings → Data Export
- Check record counts for each data type
- Click "Export All Data as CSV"
- Verify CSV files download with timestamp and JSON data

## Data Structures

### **Sample Data Generated**
```javascript
// Heartbeat (VD=0) - Device status and location
{
  "VD": "0",
  "TIMESTAMP": "2025-08-07 14:30:00",
  "IMEI": "869630050762180",
  "CRSSI": "-65",
  "LAT": "28.6139",
  "LONG": "77.2090",
  "BTVOLT": "12.5",
  "TEMP": "25"
  // ... additional fields
}

// Pump Data (VD=1) - Performance metrics
{
  "VD": "1",
  "TIMESTAMP": "2025-08-07 14:30:00",
  "IMEI": "869630050762180",
  "PDKWH1": "15.2",
  "POPKW1": "2.5",
  "POPFREQ1": "50",
  "POPFLW1": "150"
  // ... additional fields
}

// DAQ Data (VD=12) - I/O readings
{
  "VD": "12",
  "TIMESTAMP": "2025-08-07 14:30:00",
  "AI11": "3.2",
  "DI11": "1",
  "DO11": "1"
  // ... additional fields
}
```

### **Parameter Mappings**
All technical parameter names are mapped to user-friendly display names:
- `CRSSI` → "Signal Strength (dBm)"
- `BTVOLT` → "Battery Voltage (V)"
- `POPKW1` → "Power (kW)"
- `POPFLW1` → "Flow Rate (L/min)"

## Files Structure

```
web-mockup/
├── index.html          # Main HTML structure
├── styles.css          # Complete CSS styling
├── script.js           # All JavaScript functionality
└── README.md          # This documentation
```

## Browser Compatibility
- ✅ **Chrome/Edge**: Full functionality
- ✅ **Firefox**: Full functionality  
- ✅ **Safari**: Full functionality
- ✅ **Mobile Browsers**: Responsive design

## Testing Scenarios

### **Connection Flow Testing**
1. Start disconnected → Connect → See status change
2. Start simulation → Verify data flow
3. Disconnect → Verify simulation stops
4. Reconnect → Restart simulation

### **Navigation Testing**
1. Test all drawer navigation items
2. Test tab switching in Dashboard and Raw Data
3. Verify active states update correctly
4. Test responsive behavior on mobile

### **Data Flow Testing**
1. Generate data → Verify appears in all relevant screens
2. Test real-time updates during simulation
3. Verify data persistence during navigation
4. Test export functionality with various data amounts

### **UI/UX Testing**
1. Test dark theme consistency
2. Verify responsive layout on different screen sizes
3. Test touch interactions on mobile devices
4. Verify animations and transitions

## Comparison with Android App

| Feature | Android App | Web Mockup | Status |
|---------|-------------|------------|---------|
| Home Screen | ✅ | ✅ | ✅ Identical |
| UX Dashboard | ✅ | ✅ | ✅ Identical |
| Dashboard (4 tabs) | ✅ | ✅ | ✅ Identical |
| Raw Data (3 tabs) | ✅ | ✅ | ✅ Identical |
| Settings | ✅ | ✅ | ✅ Identical |
| MQTT Connection | ✅ | ✅ Simulated | ✅ Functionally Equivalent |
| Data Simulation | ✅ | ✅ | ✅ Identical |
| Pump Control | ✅ | ✅ | ✅ Identical |
| CSV Export | ✅ | ✅ | ✅ Identical |
| Dark Theme | ✅ | ✅ | ✅ Identical |
| Navigation | ✅ | ✅ | ✅ Identical |

## Future Enhancements
- **WebSocket Integration**: Real MQTT broker connection
- **Local Storage**: Persist data between sessions
- **PWA Support**: Install as web app
- **Real-time Charts**: Add graphing capabilities
- **Date Range Filtering**: Enhanced export options

## Notes
- This is a **complete functional replica** of the Android app
- **No server required** - runs entirely in browser
- **Realistic data simulation** matches Android app behavior
- **Identical user flows** and navigation patterns
- **Full export functionality** with CSV generation
- **Responsive design** works on all device sizes

Perfect for **system testing**, **user training**, and **demonstrations** without requiring Android devices or real MQTT infrastructure.
