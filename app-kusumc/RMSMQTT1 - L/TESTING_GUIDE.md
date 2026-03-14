# PMKUSUM IoT App - Testing Guide & User Manual

## 🧪 **Testing Checkpoints**

### **CHECKPOINT 1: Project Build** ⚡
**What to do:**
```bash
cd "C:\Users\Autogrid\AndroidStudioProjects\RMSMQTT1"
.\gradlew clean
.\gradlew assembleDebug
```

**Success Indicators:**
- ✅ No compilation errors
- ✅ All dependencies resolved
- ✅ APK generated successfully
- ✅ Kotlin compilation completed without type errors
- ✅ All Compose imports resolved correctly

**Failure Signs:**
- ❌ Gradle sync errors
- ❌ Missing dependencies
- ❌ Kotlin compilation issues
- ❌ Missing import statements

**Fix if fails:** Check Android Studio sync, update SDK, verify internet connection, check import statements

**Recent Fixes Applied:**
- ✅ Removed conflicting XML layouts (content_main.xml, app_bar_main.xml, etc.)
- ✅ Fixed missing `Alignment` import in SettingsScreenWithExport.kt
- ✅ Fixed type mismatches in DemoDataGenerator.kt (Int vs Double parameters)
- ✅ Cleaned up MainActivity to use Compose exclusively
- ✅ Added packaging configuration to exclude duplicate META-INF files from Netty libraries
- ✅ Added exclusion for META-INF/io.netty.versions.properties file
- ✅ Suppressed compileSdk warning for Android Gradle Plugin compatibility

---

### **CHECKPOINT 2: App Launch** 📱
**What to do:**
1. Deploy to Android device/emulator
2. Launch app from home screen
3. Check if app opens without crashes

**Success Indicators:**
- ✅ App launches successfully
- ✅ Dark theme loads correctly
- ✅ Home screen displays with app logo
- ✅ Navigation drawer opens with menu button

**Failure Signs:**
- ❌ App crashes on startup
- ❌ White/blank screen
- ❌ Navigation drawer doesn't open

**Fix if fails:** Check device compatibility, permissions, logcat for errors

---

### **CHECKPOINT 3: Navigation & UI** 🧭
**What to do:**
1. Open navigation drawer (hamburger menu)
2. Navigate to each screen: Home → Dashboard → Raw Data → Settings
3. Check UI elements load properly

**Success Indicators:**
- ✅ All 4 screens load without errors
- ✅ Dark theme applied consistently
- ✅ Text is readable and properly aligned
- ✅ Bottom navigation works in Dashboard/Raw Data

**Failure Signs:**
- ❌ Screens crash when navigating
- ❌ UI elements overlap or misaligned
- ❌ White backgrounds instead of dark theme

**Fix if fails:** Check Compose theme configuration, screen layouts

---

### **CHECKPOINT 4: MQTT Service** 🔌
**What to do:**
1. Go to Settings screen
2. Verify default settings: `test.mosquitto.org:1883`
3. Tap "Connect" button
4. Check notification tray for service notification

**Success Indicators:**
- ✅ "MQTT Service" notification appears
- ✅ Connection status changes to "Connecting..." then "Active"
- ✅ Green pulse animation on connection indicator
- ✅ No error notifications

**Failure Signs:**
- ❌ Connection status stays "Inactive"
- ❌ Error notifications appear
- ❌ No service notification
- ❌ App crashes when connecting

**Fix if fails:** Check internet connection, firewall, DNS resolution

---

### **CHECKPOINT 5: Data Simulation** 🎯
**What to do:**
1. With MQTT connected (Status = "Active"), go to Settings
2. Scroll down to "Data Simulation" section (should appear after connection)
3. Set "Sending Interval" to 5 seconds
4. Tap "Simulate Data" button
5. Watch status change to "Stop Data Simulation"
6. Check notification for background activity

**Success Indicators:**
- ✅ "Data Simulation" card appears when connected
- ✅ Button changes to "Stop Data Simulation" when active
- ✅ "Publishing every 5s" status appears
- ✅ Packet count increases (30 packets per cycle: 10 heartbeat + 10 data + 10 daq)
- ✅ Sending interval field becomes disabled during simulation

**Failure Signs:**
- ❌ Data simulation card doesn't appear
- ❌ Button doesn't respond
- ❌ Packet count doesn't increase
- ❌ App crashes when starting simulation

**Fix if fails:** Check MQTT connection, service binding, coroutine handling

---

### **CHECKPOINT 6: Data Reception** 📊
**What to do:**
1. With MQTT connected, go to Dashboard
2. Navigate through all tabs: Heartbeat, Data, DAQ, On Demand
3. Go to Raw Data screen and check all tabs
4. Look for "No data received yet" messages

**Success Indicators:**
- ✅ Data tables appear (even if empty initially)
- ✅ "No data received yet" messages display properly
- ✅ No crashes when switching tabs
- ✅ Smooth scrolling in Raw Data

**Failure Signs:**
- ❌ Tabs crash when selected
- ❌ Blank screens instead of data tables
- ❌ JSON parsing errors in notifications

**Fix if fails:** Check JSON data structures, MQTT subscriptions

---

### **CHECKPOINT 6: Data Reception** 📊
**What to do:**
1. With data simulation running, go to Dashboard
2. Navigate through all tabs: Heartbeat, Data, DAQ, On Demand
3. Go to Raw Data screen and check all tabs
4. Look for actual data appearing in tables

**Success Indicators:**
- ✅ Data tables populate with real values
- ✅ Timestamps update regularly
- ✅ Parameter names and units display correctly
- ✅ Raw Data shows JSON messages with timestamps
- ✅ No "No data received yet" messages

**Failure Signs:**
- ❌ Tables remain empty despite simulation running
- ❌ Tabs crash when selected
- ❌ JSON parsing errors in notifications
- ❌ Data doesn't update over time

**Fix if fails:** Check MQTT subscriptions, JSON parsing, data flow

---

### **CHECKPOINT 7: Pump Control** 🔧
**What to do:**
1. Go to Dashboard → On Demand tab
2. Tap "Turn ON" button
3. Check "Latest Command Status" section
4. Tap "Turn OFF" button
5. Verify status updates

**Success Indicators:**
- ✅ Buttons respond to taps
- ✅ Simulated response appears in status table
- ✅ Status shows "Pump ON/OFF" correctly
- ✅ Timestamp updates with each command

**Failure Signs:**
- ❌ Buttons don't respond
- ❌ No status updates
- ❌ App crashes when sending commands

**Fix if fails:** Check MQTT publish functionality, command generation

---

### **CHECKPOINT 7: Pump Control** 🔧
**What to do:**
1. Go to Dashboard → On Demand tab
2. Tap "Turn ON" button
3. Check "Latest Command Status" section
4. Tap "Turn OFF" button
5. Verify status updates

**Success Indicators:**
- ✅ Buttons respond to taps
- ✅ Simulated response appears in status table
- ✅ Status shows "Pump ON/OFF" correctly
- ✅ Timestamp updates with each command

**Failure Signs:**
- ❌ Buttons don't respond
- ❌ No status updates
- ❌ App crashes when sending commands

**Fix if fails:** Check MQTT publish functionality, command generation

---

### **CHECKPOINT 8: CSV Export** 📁
**What to do:**
1. After collecting some data via simulation, go to Settings
2. Scroll to "Data Export" section
3. Check data counts (should show numbers > 0)
4. Tap "Export All Data"
5. Choose sharing method when prompted

**Success Indicators:**
- ✅ Data counts show collected data (Heartbeat: X, Pump: Y, DAQ: Z)
- ✅ Export button works and opens sharing dialog
- ✅ CSV files are generated and shared successfully
- ✅ No crashes during export process

**Failure Signs:**
- ❌ Data counts remain at 0 despite simulation
- ❌ Export button doesn't work
- ❌ File permission errors
- ❌ App crashes when exporting

**Fix if fails:** Check data storage, file permissions, CSV generation

---

### **CHECKPOINT 9: Simulation Control** ⏯️
**What to do:**
1. While simulation is running, go to Settings
2. Change the sending interval to 10 seconds
3. Verify field is disabled during simulation
4. Tap "Stop Data Simulation"
5. Verify field becomes editable
6. Change interval to 3 seconds and restart simulation

**Success Indicators:**
- ✅ Interval field disabled during simulation
- ✅ Stop button works correctly
- ✅ Field becomes editable after stopping
- ✅ New interval takes effect when restarted
- ✅ Packet counts reset when simulation restarts

**Failure Signs:**
- ❌ Can edit interval during simulation
- ❌ Stop button doesn't work
- ❌ New interval not applied
- ❌ Simulation state gets stuck

**Fix if fails:** Check UI state management, simulation service control

---

## 📖 **USER MANUAL**

### **Getting Started**

#### **1. Initial Setup**
1. **Install the app** on your Android device (API 24+)
2. **Grant permissions** when prompted:
   - Internet access (automatic)
   - Notifications (tap "Allow")
   - File storage (when exporting data)

#### **2. First Connection**
1. **Open the app** → You'll see the Home screen
2. **Tap the menu** (☰) in the top-left corner
3. **Navigate to Settings**
4. **Default settings are pre-configured:**
   - URL: `test.mosquitto.org`
   - Port: `1883`
   - Username: (empty)
   - Password: (empty)
   - Client ID: Auto-generated
5. **Tap "Connect"** → Status should change to "Active"

### **Main Features**

#### **🏠 Home Screen**
- **App information** and version number
- **Connection status** with color indicator:
  - 🟢 Green (pulsing) = Connected
  - 🔴 Red = Disconnected
  - 🟡 Yellow = Connecting/Disconnecting

#### **📊 Dashboard Screen**
Navigate between 4 tabs using bottom navigation:

**Heartbeat Tab:**
- Device status information
- GPS location and signal strength
- Battery voltage and temperature
- Network connectivity status

**Data Tab:**
- Pump performance metrics
- Energy generation (daily/total)
- Water discharge measurements
- Operating voltages and currents

**DAQ Tab:**
- Data acquisition readings
- Analog input values (AI1-AI4)
- Digital input/output status (DI1-DI4, DO1-DO4)

**On Demand Tab:**
- **Pump Control Buttons:**
  - Green "Turn ON" button
  - Red "Turn OFF" button
- **Command Status:** Shows simulated device response
- **Disclaimer:** Notes that responses are simulated

#### **📁 Raw Data Screen**
View historical JSON messages:
- **Heartbeat Tab:** Raw heartbeat data with timestamps
- **Data Tab:** Raw pump data messages
- **DAQ Tab:** Raw DAQ system data
- Data displayed in chronological order (newest first)

#### **⚙️ Settings Screen**
**MQTT Configuration:**
- Modify broker URL and port if needed
- Add username/password for secured brokers
- View auto-generated client ID

**Data Simulation:**
- **Sending Interval:** Set how often to publish demo data (in seconds)
- **Simulate Data Button:** Start/stop publishing demo data
- **Packet Counter:** Shows total packets published during current session
- **Interval Control:** Cannot edit interval while simulation is running

**Data Export:**
- View data record counts
- Export all data as CSV files
- Share exported files via email/cloud storage

### **Advanced Usage**

#### **Data Simulation (Testing)**
1. **Connect to MQTT broker** first (Status must be "Active")
2. **Go to Settings → Data Simulation section**
3. **Set Sending Interval** (default: 5 seconds)
4. **Tap "Simulate Data"** to start publishing demo data
5. **Monitor Progress:**
   - Status shows "Publishing every Xs"
   - Packet counter increases
   - Data appears in Dashboard and Raw Data screens
6. **Change Frequency:**
   - Tap "Stop Data Simulation"
   - Change sending interval
   - Tap "Simulate Data" again with new frequency
7. **Data Published:** 10 packets each to heartbeat, data, and daq topics per cycle

#### **Pump Control**
1. **Go to Dashboard → On Demand tab**
2. **Send Commands:**
   - Tap "Turn ON" to start pump
   - Tap "Turn OFF" to stop pump
3. **Monitor Response:**
   - Check "Latest Command Status" section
   - Timestamp shows when command was sent
   - Status indicates pump state (Running/Stopped)

#### **Data Export**
1. **Collect data** by keeping the app connected
2. **Go to Settings → Data Export**
3. **Check record counts** (Heartbeat, Pump, DAQ data)
4. **Tap "Export All Data"** when ready
5. **Choose sharing method** (Email, Google Drive, etc.)
6. **CSV files include:**
   - Timestamp and JSON data for each record
   - Separate files for each data type

### **Troubleshooting**

#### **Connection Issues**
- **Check internet connection** on your device
- **Verify broker settings** in Settings screen
- **Try disconnecting and reconnecting**
- **Check notification tray** for error messages

#### **No Data Appearing**
- **Ensure MQTT connection is active** (green status)
- **Data may take time** to appear from real devices
- **Check Raw Data screen** for incoming messages
- **Connection status** should show "Active"

#### **App Performance**
- **Keep app in foreground** for best performance
- **Background service** maintains connection when minimized
- **Battery optimization** may affect background operation
- **Close and reopen** if UI becomes unresponsive

### **Data Understanding**

#### **Heartbeat Data (VD=0)**
- **Device health** and connectivity status
- **Location information** (GPS coordinates)
- **Network signal strength** and connection quality
- **Battery and temperature** monitoring

#### **Pump Data (VD=1)**
- **Energy production** (daily and cumulative kWh)
- **Water discharge** (daily and total liters)
- **Electrical parameters** (voltage, current, frequency)
- **Pump operational status** (running/stopped)

#### **DAQ Data (VD=12)**
- **Analog inputs** for sensor readings
- **Digital inputs** for switch/status monitoring
- **Digital outputs** for control signals
- **System timing** and indexing information

### **Important Notes**

#### **Demo Mode**
- This is a **demonstration version**
- **Simulated responses** for pump commands
- **Real MQTT data** can be received from actual devices
- **CSV export** works with all collected data

#### **Security**
- **Default broker** (test.mosquitto.org) is public
- **No encryption** on demo broker
- **Production use** requires secured MQTT broker
- **Client ID** is unique to prevent conflicts

#### **Battery Usage**
- **Foreground service** maintains connection
- **Optimized for efficiency** but uses background power
- **Close app** completely to stop background service
- **Connection status** visible in notification tray

---

## 🎯 **Success Criteria Summary**

### **Fully Working App Should:**
1. ✅ **Build and launch** without errors
2. ✅ **Connect to MQTT broker** successfully
3. ✅ **Display all UI screens** properly
4. ✅ **Handle navigation** smoothly
5. ✅ **Send pump commands** and show responses
6. ✅ **Export data as CSV** files
7. ✅ **Maintain connection** in background
8. ✅ **Show appropriate status** indicators

### **Ready for Production When:**
- All checkpoints pass ✅
- Real device data flows correctly ✅
- CSV export contains actual data ✅
- No crashes during normal operation ✅
- MQTT reconnection works automatically ✅

This comprehensive testing approach ensures each component works correctly before moving to the next level!
