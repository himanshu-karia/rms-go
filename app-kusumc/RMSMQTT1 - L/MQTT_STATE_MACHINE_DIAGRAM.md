# MQTT Application State Machine Diagram

## 🔄 Complete State Machine for MQTT Connection & Data Simulation

### 📊 **MQTT Connection State Machine**

```
┌─────────────────┐
│   APPLICATION   │
│     START       │
└─────────┬───────┘
          │
          ▼
   ┌─────────────┐
   │DISCONNECTED │ ◄─────────────────────────────┐
   │             │                               │
   │Button:      │                               │
   │"Connect"    │                               │
   │Enabled:true │                               │
   │Notify:"Disc"│                               │
   └──────┬──────┘                               │
          │                                      │
          │ USER_CONNECT                         │
          │ (Network Available)                  │
          ▼                                      │
   ┌─────────────┐                               │
   │ CONNECTING  │                               │
   │             │                               │
   │Button:      │                               │
   │"Connecting" │                               │
   │Enabled:false│                               │
   │Notify:"Conn"│                               │
   └──────┬──────┘                               │
          │                                      │
          │ CONNECTION_SUCCESS                   │
          ▼                                      │
   ┌─────────────┐                               │
   │  CONNECTED  │                               │
   │             │                               │
   │Button:      │                               │
   │"Disconnect" │                               │
   │Enabled:true │ ──┐                           │
   │Notify:"Conn"│   │ AUTO_SUBSCRIBE            │
   │Auto-Sub:ON  │   │                           │
   └──────┬──────┘   ▼                           │
          │     ┌─────────────┐                  │
          │     │SUBSCRIBED TO│                  │
          │     │heartbeat    │                  │
          │     │data         │                  │
          │     │daq          │                  │
          │     └─────────────┘                  │
          │                                      │
          │ USER_DISCONNECT                      │
          ▼                                      │
   ┌─────────────┐                               │
   │DISCONNECTING│                               │
   │             │                               │
   │Button:      │                               │
   │"Disconnect" │                               │
   │Enabled:false│                               │
   │Notify:"Disc"│                               │
   └──────┬──────┘                               │
          │                                      │
          │ DISCONNECTION_COMPLETE               │
          └──────────────────────────────────────┘
```

### ⚠️ **Error & Network Loss Handling**

```
   ┌─────────────┐
   │ CONNECTED   │
   └──────┬──────┘
          │
          │ NETWORK_LOST / CONNECTION_ERROR
          ▼
   ┌─────────────┐      ┌─────────────┐
   │NETWORK_LOST │ ──── │    ERROR    │
   │             │      │             │
   │Button:      │      │Button:      │
   │"Retry"      │      │"Retry"      │
   │Enabled:true │      │Enabled:true │
   │Notify:"Net" │      │Notify:"Err" │
   └──────┬──────┘      └──────┬──────┘
          │                    │
          │ NETWORK_RESTORED   │ AUTO_RECONNECT
          │ + AUTO_RECONNECT   │ (up to 5 attempts)
          ▼                    ▼
   ┌─────────────┐      ┌─────────────┐
   │ CONNECTING  │      │ CONNECTING  │
   │(Auto-Retry) │      │(Auto-Retry) │
   └─────────────┘      └─────────────┘
```

### 🔄 **Data Simulation State Machine**

```
┌─────────────────┐
│  SIMULATION     │
│    STOPPED      │
│                 │
│Button:          │
│"Start Sim"      │
│Enabled: depends │
│on MQTT state    │
└─────────┬───────┘
          │
          │ USER_START_SIMULATION
          │ (if MQTT CONNECTED)
          ▼
   ┌─────────────┐
   │ SIMULATION  │
   │   RUNNING   │ ──┐
   │             │   │ PUBLISH_HEARTBEAT
   │Button:      │   │ (every interval)
   │"Stop Sim"   │   │
   │Enabled:true │   ▼
   │Packets: N   │ ┌─────────────┐
   └──────┬──────┘ │ PUBLISHING  │
          │        │ heartbeat/  │
          │        │ data/daq    │
          │        └─────────────┘
          │
          │ USER_STOP_SIMULATION
          ▼
   ┌─────────────┐
   │ SIMULATION  │
   │   STOPPED   │
   │             │
   │Resources    │
   │Released     │
   └─────────────┘
```

## 🎯 **Button State Matrix**

### **Connect/Disconnect Button States**

| MQTT State     | Button Text    | Enabled | Color         |
|----------------|----------------|---------|---------------|
| DISCONNECTED   | "Connect"      | ✅ True | DataBlue      |
| CONNECTING     | "Connecting"   | ❌ False| DataBlue      |
| CONNECTED      | "Disconnect"   | ✅ True | SuccessGreen  |
| DISCONNECTING  | "Disconnecting"| ❌ False| DataBlue      |
| ERROR          | "Retry"        | ✅ True | WarningYellow |
| NETWORK_LOST   | "Retry"        | ✅ True | WarningYellow |

### **Simulation Button States**

| MQTT State + Simulation | Button Text      | Enabled | Color      |
|------------------------|------------------|---------|------------|
| DISCONNECTED + Stopped| "Start Simulation"| ❌ False| Gray       |
| CONNECTED + Stopped    | "Start Simulation"| ✅ True | DataBlue   |
| CONNECTED + Running    | "Stop Simulation" | ✅ True | WarningRed |
| ERROR + Running        | "Stop Simulation" | ✅ True | WarningRed |

### **Reconnect Button States**

| MQTT State     | Reconnect Button | Enabled | Visible |
|----------------|------------------|---------|---------|
| DISCONNECTED   | "Reconnect"      | ✅ True | ✅ Yes  |
| CONNECTING     | "Reconnect"      | ❌ False| ❌ No   |
| CONNECTED      | "Reconnect"      | ❌ False| ❌ No   |
| DISCONNECTING  | "Reconnect"      | ❌ False| ❌ No   |
| ERROR          | "Reconnect"      | ✅ True | ✅ Yes  |
| NETWORK_LOST   | "Reconnect"      | ✅ True | ✅ Yes  |

## 📱 **UI Status Display Matrix**

### **Notification Status**

| MQTT State     | Notification Text           | Priority |
|----------------|-----------------------------|----------|
| DISCONNECTED   | "MQTT Service - Disconnected"| LOW     |
| CONNECTING     | "MQTT Service - Connecting..."| LOW     |
| CONNECTED      | "MQTT Service - Connected to server:port"| LOW |
| DISCONNECTING  | "MQTT Service - Disconnecting..."| LOW  |
| ERROR          | "MQTT Service - Error: message"| HIGH    |
| NETWORK_LOST   | "MQTT Service - Network unavailable"| MEDIUM |

### **Settings Screen Status**

| MQTT State     | Status Text      | Color         | Indicator |
|----------------|------------------|---------------|-----------|
| DISCONNECTED   | "Disconnected"   | Red           | ●         |
| CONNECTING     | "Connecting..."  | Yellow        | ◐         |
| CONNECTED      | "Connected"      | Green         | ●         |
| DISCONNECTING  | "Disconnecting"  | Yellow        | ◐         |
| ERROR          | "Error"          | Red           | ⚠         |
| NETWORK_LOST   | "Network Lost"   | Orange        | ⚠         |

### **Main Screen Status**

| Component      | DISCONNECTED | CONNECTING | CONNECTED | ERROR |
|----------------|--------------|------------|-----------|--------|
| Connection Indicator| Red ●    | Yellow ◐   | Green ●   | Red ⚠  |
| Data Reception | No Data      | No Data    | Live Data | No Data|
| Timestamp      | Last Known   | Last Known | Real-time | Last Known|

## 🔄 **Complete State Transition Flow**

### **Successful Connection Flow**
```
DISCONNECTED
    │ User taps "Connect"
    │ (Network validation passed)
    ▼
CONNECTING
    │ DNS Resolution
    │ TCP Connection
    │ MQTT Handshake
    ▼
CONNECTED
    │ Auto-subscribe to:
    │ - 869630050762180/heartbeat
    │ - 869630050762180/data  
    │ - 869630050762180/daq
    ▼
SUBSCRIBED & READY
    │ Can start simulation
    │ Can publish messages
    │ Can receive messages
```

### **Disconnection Flow**
```
CONNECTED
    │ User taps "Disconnect"
    ▼
DISCONNECTING
    │ Stop simulation (if running)
    │ Unsubscribe from topics
    │ MQTT Disconnect
    │ Cleanup resources
    ▼
DISCONNECTED
    │ Ready for new connection
```

### **Error Recovery Flow**
```
CONNECTED
    │ Network Lost / Server Error
    ▼
ERROR/NETWORK_LOST
    │ Auto-reconnection attempts (up to 5)
    │ Exponential backoff (5s delay)
    ▼
CONNECTING (Auto-retry)
    │ Success? → CONNECTED
    │ Failed? → ERROR (max attempts reached)
```

### **Simulation Control Flow**
```
MQTT CONNECTED
    │ User taps "Start Simulation"
    ▼
SIMULATION RUNNING
    │ Timer starts (configurable interval)
    │ Publishes heartbeat/data/daq
    │ Increments packet counter
    │ 
    │ User taps "Stop Simulation"
    ▼
SIMULATION STOPPED
    │ Timer cancelled
    │ Resources released
    │ Counter preserved
```

## 🔧 **Implementation Details**

### **State Manager Classes**
- `MqttConnectionState` enum - 6 states
- `StateTrigger` enum - 13 transition triggers
- `MqttStateManager` - Central state controller
- `ButtonConfig` data class - UI button configuration

### **Network Monitoring**
- `NetworkMonitor` - Real-time network state
- Automatic state transitions on network changes
- Background validation every 10 seconds

### **Auto-Reconnection Logic**
- Maximum 5 attempts with exponential backoff
- Triggered by network restoration
- Preserves original connection parameters
- Manual reconnection resets attempt counter

### **Subscription Management**
- Auto-subscribe after successful connection
- Hardcoded topics with 869630050762180 prefix
- Automatic re-subscription on reconnection
- Topic consistency across publish/subscribe

### **Resource Management**
- Simulation timer cleanup on stop
- MQTT client cleanup on disconnect
- Memory-efficient data structure updates
- Background coroutine lifecycle management

---

**📝 Note**: This state machine ensures consistent behavior across all UI components, proper resource management, and reliable connection handling with comprehensive error recovery mechanisms.
