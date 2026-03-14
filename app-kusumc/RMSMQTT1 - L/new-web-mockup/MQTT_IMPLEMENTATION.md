# PMKUSUM IoT Monitor - Real MQTT Implementation Guide

## 🚀 Real MQTT Integration Complete

The web mockup now includes **full real MQTT connectivity** using MQTT.js library, providing 100% functional parity with the Android application.

## 🔧 MQTT Configuration

### Connection Settings
```javascript
// Default Configuration
MQTT Broker URL: "localhost" 
WebSocket Port: 8080
Username: (optional)
Password: (optional)  
Client ID: NEReceiver554_<timestamp>
Topic Prefix: 869630050762180 (configurable)
```

### Supported Transports
- **WebSocket**: `ws://broker:port` (primary method)
- **Secure WebSocket**: `wss://broker:port` (for HTTPS sites)
- **Auto-detection**: Tries WebSocket ports 8080, 8081, 9001
- **Fallback**: Standard MQTT port + 7000 for WebSocket gateway

## 📡 Topic Structure

### Subscribed Topics
```
{topicPrefix}/heartbeat   - Device status and GPS data (VD="0")
{topicPrefix}/pump        - Pump operational data (VD="1")  
{topicPrefix}/data        - Pump data mirror (same as /pump)
{topicPrefix}/daq         - Data acquisition parameters (VD="12")
{topicPrefix}/ondemand    - System requests/responses (VD="254")
```

### Published Topics  
The web app publishes to the same topics for:
- **Data Simulation**: Sends test data during simulation mode
- **OnDemand Requests**: VD="254" system data requests
- **Pump Commands**: Control messages (future enhancement)

## 🔄 Real-Time Features

### Live MQTT Connection
- **Automatic Connection**: Connects to real MQTT broker on settings apply
- **Real Message Handling**: Processes actual MQTT messages from devices
- **Live Subscriptions**: Subscribes to all device topics with prefix
- **Connection Status**: Real-time indicator of broker connectivity

### Message Processing
- **JSON Parsing**: All messages parsed as JSON with error handling
- **Data Validation**: Validates message format before processing  
- **Timestamp Tracking**: Each message tagged with receive time
- **Memory Management**: Keeps last 100 messages per topic

### Automatic Reconnection
- **Connection Monitoring**: Detects disconnections automatically
- **Exponential Backoff**: Increasing delay between reconnect attempts
- **Max Attempts**: Configurable limit (default: 10 attempts)
- **Status Updates**: User notifications for connection state changes

## 💻 Implementation Details

### MQTT.js Integration
```html
<script src="https://unpkg.com/mqtt/dist/mqtt.min.js"></script>
```

### Connection Code
```javascript
const options = {
    clientId: clientId,
    clean: true,
    connectTimeout: 10000,
    reconnectPeriod: 0, // Manual reconnection
    username: username || undefined,
    password: password || undefined
};

appState.mqttClient = mqtt.connect(brokerUrl, options);
```

### Message Handling
```javascript
appState.mqttClient.on('message', (topic, message) => {
    try {
        const data = JSON.parse(message.toString());
        const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
        
        // Store data by topic type
        if (topic.endsWith('/heartbeat')) {
            appState.data.heartbeat.push({ timestamp, data });
        } else if (topic.endsWith('/pump') || topic.endsWith('/data')) {
            appState.data.pump.push({ timestamp, data });
        } // ... handle other topics
        
        // Update all displays
        updateDataCounts();
        if (appState.currentScreen === 'dashboard') updateDashboardData();
        if (appState.currentScreen === 'raw-data') updateRawDataDisplay();
        
    } catch (error) {
        console.error('JSON parse error:', error);
        showToast(`JSON parsing error: ${topic}`, 'error');
    }
});
```

### Data Simulation via MQTT
```javascript
function publishHeartbeatData() {
    const heartbeatData = {
        ...sampleData.heartbeat,
        TIMESTAMP: new Date().toISOString().replace('T', ' ').substring(0, 19),
        CRSSI: String(-70 + Math.random() * 10),
        BTVOLT: String(12 + Math.random() * 1).substring(0, 4),
        TEMP: String(20 + Math.random() * 15).substring(0, 4)
    };
    
    const topic = `${appState.topicPrefix}/heartbeat`;
    if (appState.mqttClient) {
        appState.mqttClient.publish(topic, JSON.stringify(heartbeatData));
    }
}
```

## 🛠️ MQTT Broker Setup

### For Development/Testing

#### Mosquitto with WebSocket
```bash
# Install Mosquitto
sudo apt-get install mosquitto mosquitto-clients

# Enable WebSocket in /etc/mosquitto/mosquitto.conf
listener 1883
listener 8080
protocol websockets

# Start broker
sudo systemctl start mosquitto
```

#### HiveMQ Community Edition
```bash
# Download and run HiveMQ CE with WebSocket support
# Default WebSocket port: 8000
```

#### EMQX Broker
```bash
# Docker command for EMQX with WebSocket
docker run -d --name emqx \
  -p 1883:1883 \
  -p 8083:8083 \
  -p 8084:8084 \
  -p 18083:18083 \
  emqx/emqx:latest
```

### Production Setup
- Use **secure WebSocket** (WSS) for HTTPS sites
- Configure **authentication** with username/password
- Set up **SSL/TLS certificates** for secure connections
- Enable **logging** for debugging and monitoring

## 🧪 Testing Real MQTT

### Test with Real Device
1. **Configure Device**: Set device to publish to your MQTT broker
2. **Set Topic Prefix**: Use device IMEI as topic prefix in web app
3. **Connect**: Establish connection to same MQTT broker  
4. **Monitor**: View real device data in all app screens
5. **Verify**: Check console logs for successful message reception

### Test with MQTT Client
```bash
# Publish test heartbeat data
mosquitto_pub -h localhost -p 1883 \
  -t "869630050762180/heartbeat" \
  -m '{"VD":"0","TIMESTAMP":"2024-01-01 12:00:00","IMEI":"869630050762180","CRSSI":"-65","LAT":"28.6139","LONG":"77.2090","BTVOLT":"12.5","TEMP":"25","FLASH":"1","RFCARD":"1","SDCARD":"1","GPS":"1"}'

# Subscribe to all topics  
mosquitto_sub -h localhost -p 1883 -t "869630050762180/+"
```

### Test with Web Simulation
1. **Connect** to MQTT broker in Settings
2. **Start Simulation** with desired interval
3. **Monitor Traffic** using MQTT client or broker logs
4. **Verify Publishing** by subscribing to topics externally
5. **Check Reception** by viewing data in Dashboard/Raw Data screens

## 🔍 Troubleshooting

### Connection Issues
**WebSocket Connection Failed**
- Verify broker supports WebSocket connections
- Check if WebSocket port (8080/8081/9001) is accessible
- Try different WebSocket ports or TCP port + 7000

**Authentication Errors**
- Verify username/password if broker requires authentication
- Check broker logs for authentication failures
- Ensure client ID is unique and allowed

**CORS/Security Issues**  
- Modern browsers may block unsecured WebSocket connections
- Use HTTPS + WSS for production environments
- Check browser console for security errors

### Data Issues
**No Data Received**
- Verify topic prefix matches publishing device/client
- Check broker logs to confirm messages are being published
- Ensure JSON format is valid in published messages

**JSON Parse Errors**
- Validate message payload format in broker logs
- Check for special characters or encoding issues  
- Verify data types match expected format

**Message Loss**
- Check QoS levels (web app uses QoS 0 by default)
- Monitor broker message queue and memory usage
- Verify network stability between client and broker

## 📊 Performance Considerations

### Memory Usage
- **Message Limit**: Only keeps last 100 messages per topic
- **Automatic Cleanup**: Removes old messages when limit exceeded
- **Efficient Storage**: Messages stored as lightweight objects

### Network Optimization  
- **QoS 0**: Used for real-time data to minimize overhead
- **Clean Session**: Reduces broker memory usage
- **Selective Updates**: Only updates changed UI elements

### Browser Compatibility
- **WebSocket Support**: All modern browsers supported
- **MQTT.js Compatibility**: Works in all major browsers
- **Performance**: Optimized for minimal resource usage

## 🚀 Advanced Features

### Custom Topic Configuration
```javascript
// Configurable topic prefix in Settings
appState.topicPrefix = document.getElementById('topic-prefix').value;

// Dynamic topic subscription
const topics = [
    `${appState.topicPrefix}/heartbeat`,
    `${appState.topicPrefix}/pump`,
    `${appState.topicPrefix}/data`,
    `${appState.topicPrefix}/daq`, 
    `${appState.topicPrefix}/ondemand`
];
```

### OnDemand Data Requests
```javascript
function sendOnDemandRequest() {
    const onDemandData = {
        VD: "254",
        TIMESTAMP: new Date().toISOString().replace('T', ' ').substring(0, 19),
        IMEI: appState.topicPrefix,
        REQTYPE: "1"
    };
    
    const topic = `${appState.topicPrefix}/ondemand`;
    appState.mqttClient.publish(topic, JSON.stringify(onDemandData));
}
```

### Background Data Generation
- **Automatic**: Generates heartbeat data every 30 seconds when connected
- **Realistic**: Uses random variations in sensor values
- **Non-intrusive**: Only when not actively simulating

## 🎯 Conclusion

The web mockup now provides **complete real MQTT functionality**:

✅ **Real broker connection** via WebSocket transport  
✅ **Live message processing** from actual MQTT topics
✅ **Full topic subscription** matching Android app exactly
✅ **Data publishing capability** for simulation and commands  
✅ **Automatic reconnection** with proper error handling
✅ **Production-ready** implementation with security considerations

This enables the web application to work with **real PMKUSUM IoT devices** and serves as a fully functional **alternative interface** to the Android application.
