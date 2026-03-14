# MQTT Connection State Machine Design

## 🎯 **State Machine Overview**

### **Primary States**
```
DISCONNECTED → CONNECTING → CONNECTED → DISCONNECTING → DISCONNECTED
     ↑                                       ↓
     ←───────── ERROR/NETWORK_LOST ──────────
```

### **Detailed State Definitions**

#### **1. DISCONNECTED**
- **Condition**: No MQTT connection, may/may not have network
- **Network Status**: Any (Available/Unavailable)
- **MQTT Client**: Null or disconnected
- **UI Display**: "Inactive" / Red indicator
- **Button**: "Connect" (enabled only if network available)
- **Notification**: "MQTT Service - Disconnected"
- **Auto Actions**: Monitor network, attempt connection when network becomes available

#### **2. CONNECTING** 
- **Condition**: Connection attempt in progress
- **Network Status**: Must be Available & Validated
- **MQTT Client**: Connection attempt initiated
- **UI Display**: "Connecting..." / Yellow indicator (pulsing)
- **Button**: "Connecting..." (disabled)
- **Notification**: "MQTT Service - Connecting to [broker]"
- **Auto Actions**: Timeout after 30 seconds → ERROR state

#### **3. CONNECTED**
- **Condition**: Successful MQTT connection established
- **Network Status**: Available & Validated
- **MQTT Client**: Connected and active
- **UI Display**: "Active" / Green indicator
- **Button**: "Disconnect" (enabled)
- **Notification**: "MQTT Service - Connected to [broker]"
- **Auto Actions**: Monitor connection health, detect disconnections

#### **4. DISCONNECTING**
- **Condition**: Intentional disconnection in progress
- **Network Status**: Any
- **MQTT Client**: Disconnection initiated
- **UI Display**: "Disconnecting..." / Yellow indicator
- **Button**: "Disconnecting..." (disabled)
- **Notification**: "MQTT Service - Disconnecting..."
- **Auto Actions**: Force disconnect after 10 seconds if hanging

#### **5. ERROR** 
- **Condition**: Connection failed or unexpected disconnection
- **Network Status**: Any
- **MQTT Client**: Disconnected/Failed
- **UI Display**: "Error" / Red indicator (with error message)
- **Button**: "Retry" (enabled only if network available)
- **Notification**: "MQTT Service - Connection Error: [reason]"
- **Auto Actions**: Auto-retry with exponential backoff if network available

#### **6. NETWORK_LOST**
- **Condition**: Network became unavailable while connected
- **Network Status**: Unavailable
- **MQTT Client**: Disconnected (due to network)
- **UI Display**: "No Network" / Gray indicator
- **Button**: "Connect" (disabled - no network)
- **Notification**: "MQTT Service - Waiting for network"
- **Auto Actions**: Monitor network, auto-connect when network returns

### **State Transitions**

#### **Valid Transitions:**
```
DISCONNECTED → CONNECTING (user click connect + network available)
CONNECTING → CONNECTED (connection successful)
CONNECTING → ERROR (connection failed/timeout)
CONNECTED → DISCONNECTING (user click disconnect)
CONNECTED → NETWORK_LOST (network becomes unavailable)
CONNECTED → ERROR (unexpected disconnection)
DISCONNECTING → DISCONNECTED (disconnection complete)
ERROR → CONNECTING (retry attempt)
NETWORK_LOST → CONNECTING (network restored + auto-reconnect)
ERROR → DISCONNECTED (give up retrying)
```

#### **Invalid Transitions:**
```
CONNECTING → DISCONNECTED (must go through ERROR or CONNECTED)
DISCONNECTING → CONNECTED (must complete disconnection first)
Any state → Any state without proper trigger
```

### **External Triggers**

#### **Network Events:**
- `NetworkAvailable` → May trigger DISCONNECTED → CONNECTING
- `NetworkLost` → Forces any state → NETWORK_LOST
- `NetworkValidated` → May trigger auto-reconnection

#### **User Actions:**
- `Connect Button` → DISCONNECTED → CONNECTING
- `Disconnect Button` → CONNECTED → DISCONNECTING
- `Retry Button` → ERROR → CONNECTING

#### **MQTT Events:**
- `Connection Success` → CONNECTING → CONNECTED
- `Connection Failed` → CONNECTING → ERROR
- `Unexpected Disconnection` → CONNECTED → ERROR
- `Graceful Disconnection` → DISCONNECTING → DISCONNECTED

#### **System Events:**
- `App Start/Boot` → Initialize to DISCONNECTED
- `Service Restart` → Restore last known state or DISCONNECTED
- `Timeout` → CONNECTING → ERROR

### **State Validation Rules**

#### **Consistency Checks:**
1. **Network + MQTT Alignment**: 
   - CONNECTED state MUST have network available
   - NETWORK_LOST state MUST have network unavailable
   - CONNECTING state MUST have validated network

2. **UI Consistency**:
   - Button text MUST match current state
   - Status indicator MUST reflect current state
   - Notification MUST show current state

3. **Auto-Action Rules**:
   - Only attempt connection in DISCONNECTED/ERROR with network
   - Only monitor connection health in CONNECTED state
   - Only retry from ERROR state with exponential backoff

### **Error Handling**

#### **State Corruption Recovery:**
- Periodic state validation (every 5 seconds)
- Force state sync if inconsistencies detected
- Reset to DISCONNECTED if state cannot be determined

#### **Timeout Handling:**
- CONNECTING timeout (30s) → ERROR
- DISCONNECTING timeout (10s) → Force DISCONNECTED
- Network validation timeout (15s) → Treat as unavailable

### **State Persistence**
- Save current state to SharedPreferences
- Restore state on service restart
- Clear state on app uninstall

### **Logging Strategy**
```
[TIMESTAMP] STATE_CHANGE: OLD_STATE → NEW_STATE (TRIGGER: trigger_name)
[TIMESTAMP] STATE_VALIDATION: Current=STATE, Network=status, MQTT=status
[TIMESTAMP] STATE_ERROR: Expected=STATE, Actual=STATE, Reason=reason
```

This state machine ensures **deterministic behavior** and **consistent UI state** across all components.
