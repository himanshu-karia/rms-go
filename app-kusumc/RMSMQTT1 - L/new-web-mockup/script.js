// PMKUSUM IoT Monitor - Web Mockup JavaScript with Real MQTT Connection

// Application State
let appState = {
    currentScreen: 'home',
    isDrawerOpen: false,
    mqttConnected: false,
    simulationRunning: false,
    simulationInterval: 5,
    packetCounter: 0,
    clientId: '',
    topicPrefix: '869630050762180', // Default DEMO_IMEI
    mqttClient: null,
    data: {
        heartbeat: [],
        pump: [],
        daq: [],
        ondemand: []
    },
    timers: {
        simulation: null,
        gaugeAnimation: null,
        reconnection: null
    },
    connectionParams: null,
    reconnectAttempts: 0,
    maxReconnectAttempts: 10,
    reconnectDelayMs: 5000
};

// Graphing state
const graphState = {
    charts: {},
    mode: 'live', // 'live' | 'historical'
    replayTimer: null,
    replayQueue: [],
    range: { preset: 'today', start: null, end: null },
};

// Sample Data Templates (same as Android app)
const sampleData = {
    heartbeat: {
        VD: "0",
        TIMESTAMP: "",
        DATE: "",
        IMEI: "869630050762180",
        ICCID: "89014103211118510720",
        DEVICENO: "001",
        CRSSI: "-65",
        NWBAND: "8",
        LAT: "28.6139",
        LONG: "77.2090",
        CELLID: "12345",
        BTVOLT: "12.5",
        FLASH: "1",
        RFCARD: "1",
        SDCARD: "1",
        GPS: "1",
        TEMP: "25",
        POTP: "12345678",
        COTP: "87654321"
    },
    pump: {
        VD: "1",
        TIMESTAMP: "",
        DATE: "",
        IMEI: "869630050762180",
        PDKWH1: "15.2",
        PTOTKWH1: "1234.5",
        POPDWD1: "2500",
        POPTOTWD1: "125000",
        PDHR1: "6.5",
        PTOTHR1: "8760",
        POPKW1: "2.5",
        MAXINDEX: "95",
        INDEX: "3",
        LOAD: "0",
        STINTERVAL: "2",
        POTP: "12345678",
        COTP: "87654321",
        PMAXFREQ1: "60",
        PFREQLSP1: "45",
        PFREQHSP1: "55",
        PCNTRMODE1: "1",
        PRUNST1: "1",
        POPFREQ1: "50",
        POPVOLT1: "380",
        POPCUR1: "5.2",
        POPFLW1: "150"
    },
    daq: {
        VD: "12",
        TIMESTAMP: "",
        MAXINDEX: "98",
        INDEX: "5",
        LOAD: "0",
        STINTERVAL: "2",
        MSGID: "123",
        DATE: "",
        IMEI: "869630050762180",
        POTP: "12345678",
        COTP: "87654321",
        AI11: "3.2",
        AI21: "1.8",
        AI31: "4.1",
        AI41: "2.7",
        DI11: "1",
        DI21: "0",
        DI31: "1",
        DI41: "0",
        DO11: "1",
        DO21: "0",
        DO31: "0",
        DO41: "0"
    }
};

// Parameter Mappings (complete set - all parameters from packets)
const parameterMappings = {
    heartbeat: {
        VD: "Version Data",
        TIMESTAMP: "Timestamp",
        DATE: "Date",
        IMEI: "Device IMEI",
        ICCID: "SIM Card ICCID",
        DEVICENO: "Device Number",
        CRSSI: "Signal Strength (dBm)",
        NWBAND: "Network Band",
        LAT: "Latitude",
        LONG: "Longitude",
        CELLID: "Cell Tower ID",
    VBATT: "Battery Voltage (V)",
        BTVOLT: "Battery Voltage (V)",
        FLASH: "Flash Memory Status",
        RFCARD: "RF Card Status",
        SDCARD: "SD Card Status",
        GPS: "GPS Status",
        TEMP: "Temperature (°C)",
        POTP: "Previous OTP",
        COTP: "Current OTP"
    },
    pump: {
        VD: "Version Data",
        TIMESTAMP: "Timestamp",
        DATE: "Date",
        IMEI: "Device IMEI",
        PDKWH1: "Daily Energy (kWh)",
        PTOTKWH1: "Total Energy (kWh)",
        POPDWD1: "Daily Water (L)",
        POPTOTWD1: "Total Water (L)",
        PDHR1: "Daily Hours",
        PTOTHR1: "Total Hours",
        POPKW1: "Power (kW)",
        MAXINDEX: "Max Index",
        INDEX: "Current Index",
        LOAD: "Load Status",
        STINTERVAL: "Status Interval",
        POTP: "Previous OTP",
        COTP: "Current OTP",
        PMAXFREQ1: "Max Frequency (Hz)",
        PFREQLSP1: "Frequency Low SP (Hz)",
        PFREQHSP1: "Frequency High SP (Hz)",
        PCNTRMODE1: "Control Mode",
        PRUNST1: "Pump Run Status",
        POPFREQ1: "Operating Frequency (Hz)",
        POPVOLT1: "Operating Voltage (V)",
        POPCUR1: "Operating Current (A)",
        POPFLW1: "Operating Flow Rate (L/min)"
    },
    daq: {
        VD: "Version Data",
        TIMESTAMP: "Timestamp",
        MAXINDEX: "Max Index",
        INDEX: "Current Index",
        LOAD: "Load Status",
        STINTERVAL: "Status Interval",
        MSGID: "Message ID",
        DATE: "Date",
        IMEI: "Device IMEI",
        POTP: "Previous OTP",
        COTP: "Current OTP",
        AI11: "Analog Input 1 (V)",
        AI21: "Analog Input 2 (V)",
        AI31: "Analog Input 3 (V)",
        AI41: "Analog Input 4 (V)",
        DI11: "Digital Input 1",
        DI21: "Digital Input 2",
        DI31: "Digital Input 3",
        DI41: "Digital Input 4",
        DO11: "Digital Output 1",
        DO21: "Digital Output 2",
        DO31: "Digital Output 3",
        DO41: "Digital Output 4"
    }
};

// Initialize App
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

function initializeApp() {
    // Small delay to ensure DOM is fully ready
    setTimeout(() => {
        generateClientId();
        setupEventListeners();
        updateConnectionStatus();
        updateDataCounts();
        
        // Check MQTT.js library availability
        checkMqttLibrary();
        
        // Show home screen initially
        showScreen('home');
        
        // Start gauge animations after a brief delay to ensure canvases are ready
        setTimeout(() => {
            startGaugeAnimations();
        }, 100);
        
        console.log('PMKUSUM IoT Monitor initialized with real MQTT support');
    }, 50);
}

function checkMqttLibrary() {
    const statusElement = document.getElementById('mqtt-library-status');
    
    if (typeof mqtt === 'undefined') {
        console.error('MQTT.js library not available');
        showToast('MQTT library loading... Please wait and try again.', 'warning');
        
        if (statusElement) {
            statusElement.textContent = 'Loading...';
            statusElement.style.color = 'orange';
        }
        
        // Try to detect when it loads
        let attempts = 0;
        const checkInterval = setInterval(() => {
            attempts++;
            if (typeof mqtt !== 'undefined') {
                console.log('MQTT.js library loaded successfully');
                showToast('MQTT library loaded successfully', 'success');
                if (statusElement) {
                    statusElement.textContent = `Loaded (v${mqtt.VERSION || 'unknown'})`;
                    statusElement.style.color = 'green';
                }
                clearInterval(checkInterval);
            } else if (attempts > 10) { // 5 seconds
                console.error('MQTT.js library failed to load after 5 seconds');
                showToast('MQTT library failed to load. Please refresh the page.', 'error');
                if (statusElement) {
                    statusElement.textContent = 'Failed to load';
                    statusElement.style.color = 'red';
                }
                clearInterval(checkInterval);
            }
        }, 500);
    } else {
        console.log('MQTT.js library is available:', mqtt.VERSION || 'version unknown');
        showToast('MQTT library ready', 'success');
        if (statusElement) {
            statusElement.textContent = `Ready (v${mqtt.VERSION || 'unknown'})`;
            statusElement.style.color = 'green';
        }
    }
}

function setupEventListeners() {
    // Navigation drawer items
    document.querySelectorAll('.drawer-item').forEach(item => {
        item.addEventListener('click', function(e) {
            e.preventDefault();
            const screen = this.dataset.screen;
            showScreen(screen);
            closeDrawer();
            
            // Update active state
            document.querySelectorAll('.drawer-item').forEach(i => i.classList.remove('active'));
            this.classList.add('active');
        });
    });

    // Tab navigation
    document.querySelectorAll('.tab-button').forEach(button => {
        button.addEventListener('click', function() {
            const tab = this.dataset.tab;
            switchTab(tab);
            
            // Update active state
            const container = this.closest('.tab-navigation');
            container.querySelectorAll('.tab-button').forEach(b => b.classList.remove('active'));
            this.classList.add('active');
        });
    });

    // Dashboard tabs (handle both dashboard and raw-data screens)
    document.addEventListener('click', function(e) {
        if (e.target.matches('.tab-button[data-tab]')) {
            const tab = e.target.dataset.tab;
            const screen = appState.currentScreen;
            
            if (screen === 'dashboard') {
                switchDashboardTab(tab);
            } else if (screen === 'raw-data') {
                switchRawDataTab(tab);
            }
        }
    });

    // Topic prefix change listener
    document.getElementById('topic-prefix').addEventListener('change', function() {
        appState.topicPrefix = this.value;
        console.log('Topic prefix updated to:', appState.topicPrefix);
    });
}

// Navigation Functions
function toggleDrawer() {
    if (appState.isDrawerOpen) {
        closeDrawer();
    } else {
        openDrawer();
    }
}

function openDrawer() {
    appState.isDrawerOpen = true;
    document.getElementById('drawer-overlay').classList.add('active');
    document.getElementById('navigation-drawer').classList.add('active');
}

function closeDrawer() {
    appState.isDrawerOpen = false;
    document.getElementById('drawer-overlay').classList.remove('active');
    document.getElementById('navigation-drawer').classList.remove('active');
}

function showScreen(screenName) {
    try {
        // Hide all screens
        document.querySelectorAll('.screen').forEach(screen => {
            screen.classList.add('hidden');
        });
        
        // Show target screen
        const targetScreen = document.getElementById(screenName + '-screen');
        if (targetScreen) {
            targetScreen.classList.remove('hidden');
            appState.currentScreen = screenName;
            
            // Special handling for different screens
            if (screenName === 'dashboard') {
                switchDashboardTab('heartbeat');
                updateDashboardData();
            } else if (screenName === 'raw-data') {
                switchRawDataTab('raw-heartbeat');
                updateRawDataDisplay();
            } else if (screenName === 'settings') {
                updateDataCounts();
            } else if (screenName === 'ux-dashboard') {
                // Delay initialization to ensure canvas elements are ready
                setTimeout(() => {
                    initializeUxDashboard(); // Initialize with Android-like setup
                    updateUxDashboard();
                }, 100);
            } else if (screenName === 'ux-graphs') {
                // Initialize graphs screen
                setTimeout(() => {
                    initializeUxGraphs();
                }, 50);
            }
        } else {
            console.error(`Screen not found: ${screenName}`);
        }
    } catch (error) {
        console.error('Error showing screen:', error);
    }
}

function switchTab(tabName) {
    const container = event.target.closest('.main-content');
    container.querySelectorAll('.tab-pane').forEach(pane => {
        pane.classList.remove('active');
    });
    
    const targetPane = container.querySelector(`#${tabName}-tab`);
    if (targetPane) {
        targetPane.classList.add('active');
    }
}

function switchDashboardTab(tabName) {
    document.querySelectorAll('#dashboard-screen .tab-pane').forEach(pane => {
        pane.classList.remove('active');
    });
    
    const targetPane = document.getElementById(tabName + '-tab');
    if (targetPane) {
        targetPane.classList.add('active');
        
        document.querySelectorAll('#dashboard-screen .tab-button').forEach(btn => {
            btn.classList.remove('active');
        });
        document.querySelector(`#dashboard-screen .tab-button[data-tab="${tabName}"]`).classList.add('active');
        
        updateTabData(tabName);
    }
}

function switchRawDataTab(tabName) {
    document.querySelectorAll('#raw-data-screen .tab-pane').forEach(pane => {
        pane.classList.remove('active');
    });
    
    const targetPane = document.getElementById(tabName + '-tab');
    if (targetPane) {
        targetPane.classList.add('active');
        
        document.querySelectorAll('#raw-data-screen .tab-button').forEach(btn => {
            btn.classList.remove('active');
        });
        document.querySelector(`#raw-data-screen .tab-button[data-tab="${tabName}"]`).classList.add('active');
        
        updateRawDataTab(tabName);
    }
}

// MQTT Connection Functions (Real Implementation)
function generateClientId() {
    const timestamp = Date.now();
    const clientId = `NEReceiver554_${timestamp}`;
    appState.clientId = clientId;
    document.getElementById('mqtt-client-id').value = clientId;
}

// Quick setup functions
function setupHiveMQ() {
    document.getElementById('mqtt-url').value = 'broker.hivemq.com';
    document.getElementById('mqtt-port').value = '8884';
    document.getElementById('mqtt-username').value = '';
    document.getElementById('mqtt-password').value = '';
    showToast('Configured for HiveMQ public broker (WSS)', 'info');
}

function setupMosquitto() {
    document.getElementById('mqtt-url').value = 'test.mosquitto.org';
    document.getElementById('mqtt-port').value = '8081';
    document.getElementById('mqtt-username').value = '';
    document.getElementById('mqtt-password').value = '';
    showToast('Configured for Mosquitto test broker (WSS) — uses /mqtt path', 'info');
}

function setupLocal() {
    document.getElementById('mqtt-url').value = 'localhost';
    document.getElementById('mqtt-port').value = '8080';
    document.getElementById('mqtt-username').value = '';
    document.getElementById('mqtt-password').value = '';
    showToast('Configured for local broker (ensure WebSocket support)', 'info');
}

function setupEMQX() {
    document.getElementById('mqtt-url').value = 'broker.emqx.io';
    document.getElementById('mqtt-port').value = '8083';
    document.getElementById('mqtt-username').value = '';
    document.getElementById('mqtt-password').value = '';
    showToast('Configured for EMQX public broker (WS)', 'info');
}

function toggleConnection() {
    console.log('toggleConnection called, current state:', appState.mqttConnected);
    if (appState.mqttConnected) {
        disconnect();
    } else {
        connect();
    }
}

function connect() {
    console.log('Connect function called');
    
    const url = document.getElementById('mqtt-url').value;
    const port = document.getElementById('mqtt-port').value;
    const username = document.getElementById('mqtt-username').value;
    const password = document.getElementById('mqtt-password').value;
    const clientId = document.getElementById('mqtt-client-id').value;
    
    console.log('Connection parameters:', { url, port, username, clientId });
    
    appState.topicPrefix = document.getElementById('topic-prefix').value;
    
    if (!url || !port) {
        console.error('Missing URL or port');
        showToast('Please enter broker URL and port', 'error');
        return;
    }
    
    // Check if MQTT.js is loaded
    if (typeof mqtt === 'undefined') {
        console.error('MQTT.js library not loaded');
        showToast('MQTT library not loaded. Please refresh the page.', 'error');
        return;
    }
    
    // Store connection parameters for reconnection (like Android app)
    appState.connectionParams = {
        url: url,
        port: parseInt(port),
        username: username || null,
        password: password || null,
        clientId: clientId
    };
    
    showToast('Connecting to MQTT broker...', 'info');
    
    try {
        // Determine broker URL with better WebSocket detection
        let brokerUrl;
        let host = url.trim();
        // Auto-correct common hostname issue: use test.mosquitto.org instead of mosquitto.org
        if (host.includes('mosquitto.org') && !host.includes('test.mosquitto.org')) {
            showToast('Switching to test.mosquitto.org (public Mosquitto broker)', 'warning');
            host = 'test.mosquitto.org';
        }
        const portNum = parseInt(port);
        
        // Known WebSocket ports for popular brokers
        if (host.includes('broker.hivemq.com')) {
            brokerUrl = `wss://broker.hivemq.com:8884/mqtt`;
        } else if (host.includes('test.mosquitto.org')) {
            // Mosquitto public broker requires /mqtt path on WebSocket endpoints
            brokerUrl = `wss://test.mosquitto.org:8081/mqtt`;
        } else if (host.includes('broker.emqx.io') || host.includes('emqx')) {
            // EMQX public broker requires /mqtt path
            if (portNum === 8084) {
                brokerUrl = `wss://${host}:8084/mqtt`;
            } else {
                brokerUrl = `ws://${host}:8083/mqtt`;
            }
        } else if (host === 'localhost' || host === '127.0.0.1') {
            // Local broker - try WebSocket
            brokerUrl = `ws://${host}:${port}`;
        } else {
            // Generic broker detection
            if (portNum === 8000) {
                brokerUrl = `ws://${host}:8000/mqtt`;
            } else if (portNum === 8080 || portNum === 8081 || portNum === 9001) {
                brokerUrl = `ws://${host}:${port}`;
            } else if (portNum === 8883 || portNum === 8884) {
                brokerUrl = `wss://${host}:${port}/mqtt`;
            } else {
                // Default: try WebSocket on specified port
                brokerUrl = `ws://${host}:${port}`;
            }
        }
        
        console.log(`Attempting to connect to: ${brokerUrl}`);
        
        const options = {
            clientId: clientId,
            clean: true,
            connectTimeout: 10000,
            reconnectPeriod: 0, // We'll handle reconnection manually like Android app
            keepalive: 30,
            protocolVersion: 4
        };
        
        if (username) options.username = username;
        if (password) options.password = password;
        
        console.log('Connection options:', options);
        
        appState.mqttClient = mqtt.connect(brokerUrl, options);
        
        console.log('MQTT client created:', appState.mqttClient);
        
        appState.mqttClient.on('connect', () => {
            console.log('Connected to MQTT broker');
            appState.mqttConnected = true;
            appState.reconnectAttempts = 0; // Reset on successful connection
            updateConnectionStatus();
            subscribeToTopics();
            showToast('Connected to MQTT broker successfully', 'success');
            
            // Show data simulation card
            document.getElementById('data-simulation-card').style.display = 'block';
        });
        
        appState.mqttClient.on('message', (topic, message) => {
            handleMqttMessage(topic, message.toString());
        });
        
        appState.mqttClient.on('error', (error) => {
            console.error('MQTT Connection error:', error);
            appState.mqttConnected = false;
            updateConnectionStatus();
            
            // Provide specific error guidance
            let errorMessage = `Connection error: ${error.message}`;
            if (error.message.includes('WebSocket connection failed')) {
                errorMessage += '. Try a different broker or check WebSocket support.';
            } else if (error.message.includes('ENOTFOUND')) {
                errorMessage += '. Check the broker URL spelling.';
            } else if (error.message.includes('timeout')) {
                errorMessage += '. Broker may be down or port blocked.';
            }
            
            showToast(errorMessage, 'error');
            attemptReconnection();
        });
        
        appState.mqttClient.on('close', () => {
            console.log('MQTT Connection closed');
            if (appState.mqttConnected) {
                appState.mqttConnected = false;
                updateConnectionStatus();
                showToast('Connection lost. Attempting to reconnect...', 'warning');
                attemptReconnection();
            }
        });
        
        appState.mqttClient.on('offline', () => {
            console.log('MQTT Client offline');
            appState.mqttConnected = false;
            updateConnectionStatus();
        });
        
    // Add connection timeout with retry logic
        setTimeout(() => {
            if (!appState.mqttConnected) {
                console.log('Connection timeout after 10 seconds');
                
                // Try alternative connection if using HiveMQ
        if (host.includes('broker.hivemq.com') && port == '8884') {
                    showToast('Trying alternative HiveMQ endpoint...', 'warning');
                    if (appState.mqttClient) {
                        appState.mqttClient.end(true);
                    }
                    
                    // Try the older WebSocket endpoint
                    const altBrokerUrl = `ws://broker.hivemq.com:8000/mqtt`;
                    console.log(`Attempting alternative connection to: ${altBrokerUrl}`);
                    
                    appState.mqttClient = mqtt.connect(altBrokerUrl, options);
                    
                    // Set up event handlers for alternative connection
                    appState.mqttClient.on('connect', () => {
                        console.log('Connected to alternative MQTT broker endpoint');
                        appState.mqttConnected = true;
                        appState.reconnectAttempts = 0;
                        updateConnectionStatus();
                        subscribeToTopics();
                        showToast('Connected via alternative endpoint', 'success');
                        document.getElementById('data-simulation-card').style.display = 'block';
                    });
                    
                    appState.mqttClient.on('error', () => {
                        showToast('All connection attempts failed. Please try a different broker.', 'error');
                        if (appState.mqttClient) {
                            appState.mqttClient.end(true);
                            appState.mqttClient = null;
                        }
                    });
                    
                } else if (host.includes('test.mosquitto.org') && port == '8081') {
                    // Mosquitto fallback: try ws (non-TLS) 8080 with /mqtt
                    showToast('Trying Mosquitto fallback (ws://:8080/mqtt)...', 'warning');
                    if (appState.mqttClient) {
                        appState.mqttClient.end(true);
                    }

                    const altBrokerUrl = `ws://test.mosquitto.org:8080/mqtt`;
                    console.log(`Attempting alternative connection to: ${altBrokerUrl}`);

                    appState.mqttClient = mqtt.connect(altBrokerUrl, options);

                    appState.mqttClient.on('connect', () => {
                        console.log('Connected to Mosquitto via alternative ws endpoint');
                        appState.mqttConnected = true;
                        appState.reconnectAttempts = 0;
                        updateConnectionStatus();
                        subscribeToTopics();
                        showToast('Connected via Mosquitto ws fallback', 'success');
                        document.getElementById('data-simulation-card').style.display = 'block';
                    });

                    appState.mqttClient.on('error', () => {
                        showToast('All Mosquitto connection attempts failed. Try a different broker.', 'error');
                        if (appState.mqttClient) {
                            appState.mqttClient.end(true);
                            appState.mqttClient = null;
                        }
                    });
                } else {
                    showToast('Connection timeout. Please check broker settings.', 'error');
                    if (appState.mqttClient) {
                        appState.mqttClient.end(true);
                        appState.mqttClient = null;
                    }
                }
            }
        }, 10000);
        
    } catch (error) {
        console.error('Failed to create MQTT client:', error);
        showToast(`Failed to connect: ${error.message}`, 'error');
    }
}

function disconnect() {
    if (appState.mqttClient) {
        appState.mqttClient.end(true);
        appState.mqttClient = null;
    }
    
    appState.mqttConnected = false;
    appState.connectionParams = null;
    
    // Stop simulation if running
    if (appState.simulationRunning) {
        stopSimulation();
    }
    
    // Clear reconnection timer
    if (appState.timers.reconnection) {
        clearTimeout(appState.timers.reconnection);
        appState.timers.reconnection = null;
    }
    
    updateConnectionStatus();
    showToast('Disconnected from MQTT broker', 'info');
    
    // Hide data simulation card
    document.getElementById('data-simulation-card').style.display = 'none';
}

// Subscribe to topics (exactly like Android app)
function subscribeToTopics() {
    const topics = [
        `${appState.topicPrefix}/heartbeat`,
        `${appState.topicPrefix}/pump`,
        `${appState.topicPrefix}/data`, // Added subscription to /data
        `${appState.topicPrefix}/daq`,
        `${appState.topicPrefix}/ondemand`
    ];
    
    console.log('Subscribing to topics with prefix:', appState.topicPrefix);
    
    topics.forEach(topic => {
        appState.mqttClient.subscribe(topic, (error) => {
            if (error) {
                console.error(`Failed to subscribe to ${topic}:`, error);
                showToast(`Subscription error: ${topic}`, 'error');
            } else {
                console.log(`Subscribed to ${topic}`);
            }
        });
    });
    
    showToast(`Subscribed to topics with prefix: ${appState.topicPrefix}`, 'success');
}

// Handle incoming MQTT messages (exactly like Android app)
function handleMqttMessage(topic, payload) {
    try {
        console.log(`Received on ${topic}:`, payload);
        
        const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
        
        if (topic.endsWith('/heartbeat')) {
            appState.data.heartbeat.push({
                timestamp: timestamp,
                data: JSON.parse(payload)
            });
            
            // Keep only last 100 entries
            if (appState.data.heartbeat.length > 100) {
                appState.data.heartbeat = appState.data.heartbeat.slice(-100);
            }
            
        } else if (topic.endsWith('/pump') || topic.endsWith('/data')) {
            appState.data.pump.push({
                timestamp: timestamp,
                data: JSON.parse(payload)
            });
            
            if (appState.data.pump.length > 100) {
                appState.data.pump = appState.data.pump.slice(-100);
            }
            
        } else if (topic.endsWith('/daq')) {
            appState.data.daq.push({
                timestamp: timestamp,
                data: JSON.parse(payload)
            });
            
            if (appState.data.daq.length > 100) {
                appState.data.daq = appState.data.daq.slice(-100);
            }
            
        } else if (topic.endsWith('/ondemand')) {
            appState.data.ondemand.push({
                timestamp: timestamp,
                data: JSON.parse(payload)
            });
        }
        
        // Update displays if currently viewing
        updateDataCounts();
        if (appState.currentScreen === 'dashboard') {
            updateDashboardData();
        }
        if (appState.currentScreen === 'raw-data') {
            updateRawDataDisplay();
        }
        if (appState.currentScreen === 'ux-dashboard') {
            updateUxDashboard();
        }
        if (appState.currentScreen === 'ux-graphs' && graphState.mode === 'live') {
            updateGraphsFromPayload(topic, payload, timestamp);
        }
        
    } catch (error) {
        console.error(`Error parsing message from ${topic}:`, error);
        showToast(`JSON parsing error: ${topic}`, 'error');
    }
}

// ========== UX-Graphs: init, update, and history loading ==========
function initializeUxGraphs() {
    // Setup toolbar interactions
    const modeInputs = document.querySelectorAll('input[name="graph-mode"]');
    modeInputs.forEach(input => {
        input.addEventListener('change', () => {
            graphState.mode = input.value;
            const hist = document.getElementById('historical-controls');
            if (hist) hist.style.display = (graphState.mode === 'historical') ? 'flex' : 'none';
        });
    });

    const loadBtn = document.getElementById('load-history');
    const stopBtn = document.getElementById('stop-replay');
    if (loadBtn) loadBtn.addEventListener('click', handleHistoryLoad);
    if (stopBtn) stopBtn.addEventListener('click', stopReplay);

    // Build charts idempotently (create any missing charts)
    if (!graphState.charts.voltage) {
        graphState.charts.voltage = buildMultiLineChart('graph-voltage', [
            { label: 'POPV1 (AC Out)', color: '#42a5f5' },
            { label: 'PDC1V1 (DC In)', color: '#66bb6a' },
            { label: 'PDCVOC1 (DC OC)', color: '#ffa726' }
        ]);
    }
    if (!graphState.charts.dcCurrent) {
        graphState.charts.dcCurrent = buildLineChart('graph-dc-current', 'PDC1I1 (DC Current)');
    }
    if (!graphState.charts.rssi) {
        graphState.charts.rssi = buildLineChart('graph-rssi', 'RSSI (dBm)');
    }
    if (!graphState.charts.status) {
        graphState.charts.status = buildMultiStepChart('graph-status', [
            { label: 'PRUNST1 (Run)', color: '#ab47bc' },
            { label: 'PCNTRMODE1 (Mode)', color: '#26a69a' }
        ]);
    }
    if (!graphState.charts.ai11) graphState.charts.ai11 = buildLineChart('graph-ai11', 'AI11');
    if (!graphState.charts.ai21) graphState.charts.ai21 = buildLineChart('graph-ai21', 'AI21');
    if (!graphState.charts.ai31) graphState.charts.ai31 = buildLineChart('graph-ai31', 'AI31');
    if (!graphState.charts.ai41) graphState.charts.ai41 = buildLineChart('graph-ai41', 'AI41');
    if (!graphState.charts.ops1) {
        graphState.charts.ops1 = buildMultiLineChart('graph-ops1', [
            { label: 'Frequency (Hz)', color: '#90caf9' },
            { label: 'Power (kW)', color: '#4dd0e1' }
        ]);
    }
    if (!graphState.charts.ops2) {
        graphState.charts.ops2 = buildMultiLineChart('graph-ops2', [
            { label: 'Flow (L/min)', color: '#a5d6a7' },
            { label: 'Current (A)', color: '#ffcc80' }
        ]);
    }
    if (!graphState.charts.battTemp) {
        graphState.charts.battTemp = buildMultiLineChart('graph-batt-temp', [
            { label: 'VBATT (V)', color: '#42a5f5' },
            { label: 'TEMP (°C)', color: '#ef5350' },
        ]);
    }
    if (!graphState.charts.di11) graphState.charts.di11 = buildStepChart('graph-di11', 'DI11');
    if (!graphState.charts.di21) graphState.charts.di21 = buildStepChart('graph-di21', 'DI21');
    if (!graphState.charts.di31) graphState.charts.di31 = buildStepChart('graph-di31', 'DI31');
    if (!graphState.charts.di41) graphState.charts.di41 = buildStepChart('graph-di41', 'DI41');
    if (!graphState.charts.do11) graphState.charts.do11 = buildStepChart('graph-do11', 'DO11');
    if (!graphState.charts.do21) graphState.charts.do21 = buildStepChart('graph-do21', 'DO21');
    if (!graphState.charts.do31) graphState.charts.do31 = buildStepChart('graph-do31', 'DO31');
    if (!graphState.charts.do41) graphState.charts.do41 = buildStepChart('graph-do41', 'DO41');
}

function buildLineChart(canvasId, label) {
    const ctx = document.getElementById(canvasId);
    if (!ctx || !window.Chart) return null;
    return new Chart(ctx, {
        type: 'line',
        data: { labels: [], datasets: [{ label, data: [], borderColor: '#90caf9', tension: 0.2, pointRadius: 0 }] },
        options: baseChartOptions()
    });
}

function buildMultiLineChart(canvasId, series) {
    const ctx = document.getElementById(canvasId);
    if (!ctx || !window.Chart) return null;
    return new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: series.map(s => ({ label: s.label, data: [], borderColor: s.color, tension: 0.2, pointRadius: 0 }))
        },
        options: baseChartOptions()
    });
}

function buildStepChart(canvasId, label) {
    const ctx = document.getElementById(canvasId);
    if (!ctx || !window.Chart) return null;
    return new Chart(ctx, {
        type: 'line',
        data: { labels: [], datasets: [{ label, data: [], borderColor: '#ffcc80', stepped: true, pointRadius: 0 }] },
        options: baseChartOptions()
    });
}

function buildMultiStepChart(canvasId, series) {
    const ctx = document.getElementById(canvasId);
    if (!ctx || !window.Chart) return null;
    return new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: series.map(s => ({ label: s.label, data: [], borderColor: s.color, stepped: true, pointRadius: 0 }))
        },
        options: baseChartOptions()
    });
}

function baseChartOptions() {
    return {
        responsive: true,
        animation: false,
        scales: {
            x: { ticks: { maxRotation: 0 }, type: 'category' },
            y: { beginAtZero: true }
        },
        plugins: { legend: { display: true } },
        interaction: { intersect: false, mode: 'nearest' },
        maintainAspectRatio: false
    };
}

function pushChartPoint(chart, ts, value) {
    if (!chart) return;
    chart.data.labels.push(formatTs(ts));
    chart.data.datasets[0].data.push(value);
    trimChart(chart);
    chart.update('none');
}

function pushMultiChartPoint(chart, ts, values) {
    if (!chart) return;
    chart.data.labels.push(formatTs(ts));
    chart.data.datasets.forEach((ds, i) => ds.data.push(values[i]));
    trimChart(chart);
    chart.update('none');
}

function formatTs(ts) {
    try {
        const d = new Date(ts);
        if (!isNaN(d)) return d.toLocaleTimeString();
    } catch {}
    return String(ts).slice(0, 19);
}

function trimChart(chart, maxPoints = 600) { // ~10min at 1s
    if (chart.data.labels.length > maxPoints) {
        chart.data.labels.splice(0, chart.data.labels.length - maxPoints);
        chart.data.datasets.forEach(ds => ds.data.splice(0, ds.data.length - maxPoints));
    }
}

function updateGraphsFromPayload(topic, payload, timestamp) {
    try {
        const data = JSON.parse(payload);
        const ts = timestamp || new Date().toISOString();
        // Voltage series (support POPV1 alias POPVOLT1); allow zero values
        if ('POPV1' in data || 'POPVOLT1' in data || 'PDC1V1' in data || 'PDCVOC1' in data) {
            const popv1 = num(data.POPV1 ?? data.POPVOLT1);
            const pdc1v1 = num(data.PDC1V1);
            const pdcvoc1 = num(data.PDCVOC1);
            pushMultiChartPoint(graphState.charts.voltage, ts, [popv1, pdc1v1, pdcvoc1]);
        }
        // DC Current (allow zero values)
        if ('PDC1I1' in data) {
            pushChartPoint(graphState.charts.dcCurrent, ts, num(data.PDC1I1));
        }
        // RSSI
        if (data.CRSSI || data.RSSI) {
            pushChartPoint(graphState.charts.rssi, ts, num(data.CRSSI || data.RSSI));
        }
        // Status metrics
        if ('PRUNST1' in data || 'PCNTRMODE1' in data) {
            pushMultiChartPoint(graphState.charts.status, ts, [num(data.PRUNST1), num(data.PCNTRMODE1)]);
        }
        // AI mini-graphs
        ['11','21','31','41'].forEach(suf => {
            if ((`AI${suf}`) in data) {
                pushChartPoint(graphState.charts[`ai${suf}`], ts, num(data[`AI${suf}`]));
            }
        });
        // Energy/counter summary (text only)
        if (data.PDKWH1 !== undefined) document.getElementById('summary-pdkwh1').textContent = data.PDKWH1;
        if (data.PTOTKWH1 !== undefined) document.getElementById('summary-ptotkwh1').textContent = data.PTOTKWH1;
        if (data.POPDWD1 !== undefined) document.getElementById('summary-popdwd1').textContent = data.POPDWD1;
        if (data.POPTOTWD1 !== undefined) document.getElementById('summary-poptotwd1').textContent = data.POPTOTWD1;
        if (data.PDHR1 !== undefined) document.getElementById('summary-pdhr1').textContent = data.PDHR1;
        if (data.PTOTHR1 !== undefined) document.getElementById('summary-ptothr1').textContent = data.PTOTHR1;
        // Existing graphs
        if (topic.endsWith('/pump') || topic.endsWith('/data')) {
            const freq = num(data.POPFREQ1);
            const pwr = num(data.POPKW1);
            const flw = num(data.POPFLW1);
            const cur = num(data.POPI1 || data.POPCUR1);
            pushMultiChartPoint(graphState.charts.ops1, ts, [freq, pwr]);
            pushMultiChartPoint(graphState.charts.ops2, ts, [flw, cur]);
        }
        if (topic.endsWith('/heartbeat')) {
            const vb = num(data.VBATT ?? data.BTVOLT);
            const tp = num(data.TEMP);
            if (!isNaN(vb) || !isNaN(tp)) pushMultiChartPoint(graphState.charts.battTemp, ts, [vb || null, tp || null]);
        }
        if (topic.endsWith('/daq')) {
            ['11','21','31','41'].forEach(suf => {
                const di = num(data[`DI${suf}`]);
                const doval = num(data[`DO${suf}`]);
                if (di !== null) pushChartPoint(graphState.charts[`di${suf}`], ts, di ? 1 : 0);
                if (doval !== null) pushChartPoint(graphState.charts[`do${suf}`], ts, doval ? 1 : 0);
            });
        }
    } catch (e) {
        console.error('Graph update parse error:', e);
    }
}

// Historical loading
async function handleHistoryLoad() {
    const fileInput = document.getElementById('history-file');
    const modeSel = document.getElementById('history-mode');
    if (!fileInput || !fileInput.files || fileInput.files.length === 0) {
        showToast('Select a CSV/JSON/NDJSON file', 'warning');
        return;
    }
    const file = fileInput.files[0];
    const text = await file.text();
    let packets = [];
    try {
        if (file.name.endsWith('.csv')) {
            packets = parseCsvToPackets(text);
        } else if (file.name.endsWith('.ndjson')) {
            packets = text.split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line));
        } else {
            const parsed = JSON.parse(text);
            packets = Array.isArray(parsed) ? parsed : (parsed.packets || []);
        }
    } catch (e) {
        console.error('History parse error:', e);
        showToast('Failed to parse file', 'error');
        return;
    }
    if (packets.length === 0) {
        showToast('No packets found in file', 'warning');
        return;
    }

    const mode = modeSel ? modeSel.value : 'load-all';
    if (mode === 'load-all') {
        loadAllToCharts(packets);
    } else {
        startReplay(packets);
    }
}

function parseCsvToPackets(csv) {
    const lines = csv.split(/\r?\n/).filter(Boolean);
    const header = lines.shift().split(',').map(h => h.trim());
    return lines.map(line => {
        const parts = line.split(',');
        const obj = {};
        header.forEach((h, i) => obj[h] = parts[i]);
        // Infer topic suffix from present keys
    let topic = 'pump';
        if ('VBATT' in obj || 'TEMP' in obj) topic = 'heartbeat';
        if ('AI11' in obj || 'DI11' in obj) topic = 'daq';
    return { topic, data: obj, ts: obj.TIMESTAMP || obj.DATE || new Date().toISOString() };
    });
}

function loadAllToCharts(packets) {
    // Clear current data
    Object.values(graphState.charts).forEach(ch => { if (ch) { ch.data.labels = []; ch.data.datasets.forEach(d=>d.data=[]); ch.update('none'); } });
    const filtered = filterPacketsByRange(packets);
    filtered.forEach(p => {
        const payload = JSON.stringify(p.data || p);
        updateGraphsFromPayload(`/${p.topic}`, payload, p.ts);
    });
}

function startReplay(packets) {
    stopReplay();
    graphState.replayQueue = filterPacketsByRange(packets);
    const tick = () => {
        const p = graphState.replayQueue.shift();
        if (!p) { stopReplay(); return; }
        const payload = JSON.stringify(p.data || p);
        updateGraphsFromPayload(`/${p.topic}`, payload, p.ts);
        graphState.replayTimer = setTimeout(tick, 500); // 2 packets/sec
    };
    tick();
}

function stopReplay() {
    if (graphState.replayTimer) {
        clearTimeout(graphState.replayTimer);
        graphState.replayTimer = null;
    }
    graphState.replayQueue = [];
}

// Date range filtering (client-side for file-based data)
function filterPacketsByRange(packets) {
    const { preset, start, end } = graphState.range;
    let s = start ? new Date(start) : null;
    let e = end ? new Date(end) : null;
    if (preset && (!s || !e)) {
        const now = new Date();
        if (preset === 'today') {
            s = new Date(now.getFullYear(), now.getMonth(), now.getDate());
            e = now;
        } else if (preset === 'yesterday') {
            const y = new Date(now.getFullYear(), now.getMonth(), now.getDate()-1);
            s = y; e = new Date(now.getFullYear(), now.getMonth(), now.getDate());
        } else if (preset === 'last7') {
            e = now; s = new Date(now.getTime() - 7*24*3600*1000);
        } else if (preset === 'last30') {
            e = now; s = new Date(now.getTime() - 30*24*3600*1000);
        }
    }
    if (!s || !e) return packets;
    const sT = s.getTime();
    const eT = e.getTime();
    return packets.filter(p => {
        const t = new Date(p.ts).getTime();
        return !isNaN(t) && t >= sT && t <= eT;
    });
}

// Wire preset/custom range controls
document.addEventListener('change', (e) => {
    if (e.target && e.target.id === 'preset-range') {
        graphState.range.preset = e.target.value;
    }
    if (e.target && (e.target.id === 'start-time' || e.target.id === 'end-time')) {
        graphState.range.preset = 'custom';
        graphState.range[e.target.id === 'start-time' ? 'start' : 'end'] = e.target.value;
    }
});

document.addEventListener('click', (e) => {
    if (e.target && e.target.id === 'apply-range') {
        showToast('Range applied for historical data', 'info');
    }
});

function num(v) {
    const n = Number(v);
    return isNaN(n) ? null : n;
}

// Reconnection logic (like Android app)
function attemptReconnection() {
    if (!appState.connectionParams || appState.reconnectAttempts >= appState.maxReconnectAttempts) {
        if (appState.reconnectAttempts >= appState.maxReconnectAttempts) {
            showToast('Maximum reconnection attempts reached', 'error');
        }
        return;
    }
    
    appState.reconnectAttempts++;
    console.log(`Reconnection attempt ${appState.reconnectAttempts}/${appState.maxReconnectAttempts}`);
    
    appState.timers.reconnection = setTimeout(() => {
        console.log('Attempting to reconnect...');
        connect();
    }, appState.reconnectDelayMs);
}

function updateConnectionStatus() {
    const connectButton = document.getElementById('connect-button');
    const statusIndicator = document.getElementById('mqtt-status-indicator');
    const statusText = document.getElementById('mqtt-status-text');
    const connectionDot = document.getElementById('connection-dot');
    const connectionText = document.getElementById('connection-text');
    
    if (appState.mqttConnected) {
        if (connectButton) {
            connectButton.textContent = 'Disconnect';
            connectButton.classList.add('connected');
        }
        if (statusIndicator) {
            statusIndicator.classList.add('connected');
        }
        if (statusText) {
            statusText.textContent = 'Connected';
        }
        if (connectionDot) {
            connectionDot.classList.add('connected');
        }
        if (connectionText) {
            connectionText.textContent = 'Connected';
        }
    } else {
        if (connectButton) {
            connectButton.textContent = 'Connect';
            connectButton.classList.remove('connected');
        }
        if (statusIndicator) {
            statusIndicator.classList.remove('connected');
        }
        if (statusText) {
            statusText.textContent = 'Disconnected';
        }
        if (connectionDot) {
            connectionDot.classList.remove('connected');
        }
        if (connectionText) {
            connectionText.textContent = 'Disconnected';
        }
    }
}

// Data Simulation Functions
function toggleSimulation() {
    if (appState.simulationRunning) {
        stopSimulation();
    } else {
        startSimulation();
    }
}

function startSimulation() {
    if (!appState.mqttConnected) {
        showToast('Please connect to MQTT broker first', 'error');
        return;
    }
    
    const interval = parseInt(document.getElementById('simulation-interval').value);
    appState.simulationInterval = interval;
    appState.simulationRunning = true;
    
    updateSimulationStatus();
    
    // Start simulation timer
    appState.timers.simulation = setInterval(() => {
        publishSimulationBatch();
    }, interval * 1000);
    
    // Publish first batch immediately
    publishSimulationBatch();
    
    showToast(`Data simulation started (${interval}s interval)`, 'success');
}

function stopSimulation() {
    appState.simulationRunning = false;
    
    if (appState.timers.simulation) {
        clearInterval(appState.timers.simulation);
        appState.timers.simulation = null;
    }
    
    updateSimulationStatus();
    showToast('Data simulation stopped', 'info');
}

function updateSimulationStatus() {
    const simulateButton = document.getElementById('simulate-button');
    const statusText = document.getElementById('simulation-status-text');
    const intervalInput = document.getElementById('simulation-interval');
    
    if (appState.simulationRunning) {
        simulateButton.textContent = 'Stop Data Simulation';
        simulateButton.classList.add('running');
        statusText.textContent = `Publishing every ${appState.simulationInterval}s`;
        intervalInput.disabled = true;
    } else {
        simulateButton.textContent = 'Simulate Data';
        simulateButton.classList.remove('running');
        statusText.textContent = 'Stopped';
        intervalInput.disabled = false;
    }
    
    document.getElementById('packet-counter').textContent = appState.packetCounter;
}

function publishSimulationBatch() {
    const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
    const date = new Date().toISOString().substring(2, 4) + new Date().toISOString().substring(5, 7);
    
    // Publish heartbeat data
    const heartbeatData = { 
        ...sampleData.heartbeat,
        TIMESTAMP: timestamp,
        DATE: date,
        CRSSI: (Math.random() * 30 - 80).toFixed(0), // -50 to -80 dBm
        BTVOLT: (12 + Math.random() * 2).toFixed(1), // 12-14V
        TEMP: (20 + Math.random() * 15).toFixed(0) // 20-35°C
    };
    
    // Publish to MQTT
    const heartbeatTopic = `${appState.topicPrefix}/heartbeat`;
    if (appState.mqttClient) {
        appState.mqttClient.publish(heartbeatTopic, JSON.stringify(heartbeatData));
        console.log(`Published heartbeat to ${heartbeatTopic}:`, heartbeatData);
    }
    appState.packetCounter++;
    
    // Generate pump data (30% chance of publishing)
    if (Math.random() < 0.3) {
        const isRunning = Math.random() > 0.3; // 70% chance running
        const pumpData = {
            ...sampleData.pump,
            TIMESTAMP: timestamp,
            DATE: date,
            PDKWH1: isRunning ? (10 + Math.random() * 20).toFixed(1) : "0",
            POPKW1: isRunning ? (2 + Math.random() * 3).toFixed(1) : "0",
            POPFREQ1: isRunning ? (48 + Math.random() * 4).toFixed(0) : "0",
            POPVOLT1: isRunning ? (370 + Math.random() * 20).toFixed(0) : "0",
            // DC side simulated values
            PDC1V1: isRunning ? (300 + Math.random() * 80).toFixed(0) : "0",
            PDCVOC1: isRunning ? (360 + Math.random() * 90).toFixed(0) : "0",
            PDC1I1: isRunning ? (1 + Math.random() * 6).toFixed(1) : "0",
            POPCUR1: isRunning ? (4 + Math.random() * 2).toFixed(1) : "0",
            POPFLW1: isRunning ? (100 + Math.random() * 100).toFixed(0) : "0",
            PRUNST1: isRunning ? "1" : "0"
        };
        
        // Publish to both /pump and /data topics
        const pumpTopics = [`${appState.topicPrefix}/pump`, `${appState.topicPrefix}/data`];
        pumpTopics.forEach(topic => {
            if (appState.mqttClient) {
                appState.mqttClient.publish(topic, JSON.stringify(pumpData));
                console.log(`Published pump data to ${topic}:`, pumpData);
            }
        });
        appState.packetCounter++;
    }
    
    // Generate DAQ data (20% chance of publishing)
    if (Math.random() < 0.2) {
        const daqData = {
            ...sampleData.daq,
            TIMESTAMP: timestamp,
            DATE: date,
            AI11: (Math.random() * 5).toFixed(1),
            AI21: (Math.random() * 5).toFixed(1),
            AI31: (Math.random() * 5).toFixed(1),
            AI41: (Math.random() * 5).toFixed(1),
            DI11: Math.random() > 0.5 ? "1" : "0",
            DI21: Math.random() > 0.5 ? "1" : "0",
            DI31: Math.random() > 0.5 ? "1" : "0",
            DI41: Math.random() > 0.5 ? "1" : "0"
        };
        
        const daqTopic = `${appState.topicPrefix}/daq`;
        if (appState.mqttClient) {
            appState.mqttClient.publish(daqTopic, JSON.stringify(daqData));
            console.log(`Published DAQ data to ${daqTopic}:`, daqData);
        }
        appState.packetCounter++;
    }
    
    // Update displays
    updateSimulationStatus();
    updateDataCounts();
    
    if (appState.currentScreen === 'dashboard') {
        updateDashboardData();
    }
    if (appState.currentScreen === 'raw-data') {
        updateRawDataDisplay();
    }
    if (appState.currentScreen === 'ux-dashboard') {
        updateUxDashboard();
    }
}

// Send OnDemand Request (like Android app)
function sendOnDemandRequest() {
    if (!appState.mqttConnected) {
        showToast('Please connect to MQTT broker first', 'error');
        return;
    }
    
    const onDemandData = {
        VD: "254",
        TIMESTAMP: new Date().toISOString().replace('T', ' ').substring(0, 19),
        IMEI: appState.topicPrefix,
        REQTYPE: "1"
    };
    
    const topic = `${appState.topicPrefix}/ondemand`;
    const payload = JSON.stringify(onDemandData);
    
    if (appState.mqttClient) {
        appState.mqttClient.publish(topic, payload);
        showToast('OnDemand request sent', 'success');
        console.log(`Published OnDemand request to ${topic}:`, onDemandData);
    }
}

// Background data generation (slower pace when not simulating)
function startBackgroundDataGeneration() {
    // Generate occasional data when connected but not actively simulating
    setInterval(() => {
        if (appState.mqttConnected && !appState.simulationRunning && Math.random() > 0.7) {
            const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
            const date = new Date().toISOString().substring(2, 4) + new Date().toISOString().substring(5, 7);
            
            // Generate heartbeat data
            const heartbeatData = { 
                ...sampleData.heartbeat,
                TIMESTAMP: timestamp,
                DATE: date,
                CRSSI: (Math.random() * 30 - 80).toFixed(0),
                BTVOLT: (12 + Math.random() * 2).toFixed(1),
                TEMP: (20 + Math.random() * 15).toFixed(0)
            };
            
            const topic = `${appState.topicPrefix}/heartbeat`;
            if (appState.mqttClient) {
                appState.mqttClient.publish(topic, JSON.stringify(heartbeatData));
                console.log(`Background heartbeat published to ${topic}`);
            }
        }
    }, 30000); // Every 30 seconds
}

function stopBackgroundDataGeneration() {
    // Background generation stops automatically when mqttConnected is false
}

// Pump Control Functions
function sendPumpCommand(action) {
    if (!appState.mqttConnected) {
        showToast('Please connect to MQTT broker first', 'error');
        return;
    }
    
    const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
    const command = {
        msgid: Math.floor(Math.random() * 90000 + 10000).toString(),
        COTP: "12356",
        POTP: "58986",
        timestamp: timestamp,
        type: "ondemand",
        cmd: "write",
        DO1: action === 'ON' ? 1 : 0
    };
    
    // Simulate response
    const response = {
        timestamp: timestamp,
        status: `Pump ${action}`,
        DO1: action === 'ON' ? 1 : 0,
        PRUNST1: action === 'ON' ? "1" : "0"
    };
    
    appState.data.ondemand.push({
        timestamp: timestamp,
        command: command,
        response: response
    });
    
    updateCommandStatus();
    showToast(`Pump ${action} command sent`, 'success');
}

function updateCommandStatus() {
    const statusTable = document.getElementById('command-status-table');
    
    if (appState.data.ondemand.length === 0) {
        statusTable.innerHTML = '<div class="no-data">No commands sent yet</div>';
        return;
    }
    
    const latest = appState.data.ondemand[appState.data.ondemand.length - 1];
    
    statusTable.innerHTML = `
        <div class="status-row">
            <div class="data-row">
                <div class="data-label">Timestamp</div>
                <div class="data-value">${latest.timestamp}</div>
            </div>
            <div class="data-row">
                <div class="data-label">Command</div>
                <div class="data-value">${latest.response.status}</div>
            </div>
        </div>
        <div class="status-row">
            <div class="data-row">
                <div class="data-label">DO1 Status</div>
                <div class="data-value">${latest.response.DO1 === 1 ? 'ON' : 'OFF'}</div>
            </div>
            <div class="data-row">
                <div class="data-label">Pump Status</div>
                <div class="data-value">${latest.response.PRUNST1 === '1' ? 'Running' : 'Stopped'}</div>
            </div>
        </div>
    `;
}

// Data Display Functions
function updateDashboardData() {
    updateTabData('heartbeat');
    updateTabData('data');
    updateTabData('daq');
    updateCommandStatus();
}

function updateTabData(tabName) {
    let dataType, tableId, mappings;
    
    switch(tabName) {
        case 'heartbeat':
            dataType = 'heartbeat';
            tableId = 'heartbeat-table';
            mappings = parameterMappings.heartbeat;
            break;
        case 'data':
            dataType = 'pump';
            tableId = 'data-table';
            mappings = parameterMappings.pump;
            break;
        case 'daq':
            dataType = 'daq';
            tableId = 'daq-table';
            mappings = parameterMappings.daq;
            break;
        default:
            return;
    }
    
    const table = document.getElementById(tableId);
    const data = appState.data[dataType];
    
    if (data.length === 0) {
        table.innerHTML = `<div class="no-data">No ${tabName} data received yet</div>`;
        return;
    }
    
    const latest = data[data.length - 1].data;
    let html = '';
    
    // Group parameters by category for better organization
    const parameterGroups = getParameterGroups(tabName);
    
    parameterGroups.forEach(group => {
        html += `<div class="parameter-group">`;
        if (group.title) {
            html += `<div class="group-title">${group.title}</div>`;
        }
        
        group.parameters.forEach(key => {
            if (latest[key] !== undefined && mappings[key]) {
                const value = formatParameterValue(key, latest[key]);
                html += `
                    <div class="data-row">
                        <div class="data-label">${mappings[key]}</div>
                        <div class="data-value">${value}</div>
                    </div>
                `;
            }
        });
        
        html += `</div>`;
    });
    
    // Add any remaining parameters that weren't grouped
    const groupedKeys = parameterGroups.flatMap(g => g.parameters);
    Object.keys(mappings).forEach(key => {
        if (latest[key] !== undefined && !groupedKeys.includes(key)) {
            const value = formatParameterValue(key, latest[key]);
            html += `
                <div class="data-row">
                    <div class="data-label">${mappings[key]}</div>
                    <div class="data-value">${value}</div>
                </div>
            `;
        }
    });
    
    table.innerHTML = html;
    table.classList.add('populated');
}

function getParameterGroups(tabName) {
    switch(tabName) {
        case 'heartbeat':
            return [
                {
                    title: "Device Information",
                    parameters: ['IMEI', 'ICCID', 'DEVICENO', 'VD']
                },
                {
                    title: "Network & Location",
                    parameters: ['CRSSI', 'NWBAND', 'CELLID', 'LAT', 'LONG', 'GPS']
                },
                {
                    title: "System Status",
                    parameters: ['BTVOLT', 'TEMP', 'FLASH', 'RFCARD', 'SDCARD']
                },
                {
                    title: "Timing & Security",
                    parameters: ['TIMESTAMP', 'DATE', 'POTP', 'COTP']
                }
            ];
        case 'data':
            return [
                {
                    title: "Device Information",
                    parameters: ['IMEI', 'VD', 'TIMESTAMP', 'DATE']
                },
                {
                    title: "Energy Metrics",
                    parameters: ['PDKWH1', 'PTOTKWH1', 'POPKW1']
                },
                {
                    title: "Water Metrics",
                    parameters: ['POPDWD1', 'POPTOTWD1', 'POPFLW1']
                },
                {
                    title: "Operating Parameters",
                    parameters: ['POPFREQ1', 'POPVOLT1', 'POPCUR1', 'PRUNST1']
                },
                {
                    title: "Configuration",
                    parameters: ['PMAXFREQ1', 'PFREQLSP1', 'PFREQHSP1', 'PCNTRMODE1']
                },
                {
                    title: "Runtime Info",
                    parameters: ['PDHR1', 'PTOTHR1', 'MAXINDEX', 'INDEX', 'LOAD', 'STINTERVAL']
                },
                {
                    title: "Security",
                    parameters: ['POTP', 'COTP']
                }
            ];
        case 'daq':
            return [
                {
                    title: "System Information",
                    parameters: ['IMEI', 'VD', 'MSGID', 'TIMESTAMP', 'DATE']
                },
                {
                    title: "Analog Inputs",
                    parameters: ['AI11', 'AI21', 'AI31', 'AI41']
                },
                {
                    title: "Digital Inputs",
                    parameters: ['DI11', 'DI21', 'DI31', 'DI41']
                },
                {
                    title: "Digital Outputs",
                    parameters: ['DO11', 'DO21', 'DO31', 'DO41']
                },
                {
                    title: "Control Parameters",
                    parameters: ['MAXINDEX', 'INDEX', 'LOAD', 'STINTERVAL']
                },
                {
                    title: "Security",
                    parameters: ['POTP', 'COTP']
                }
            ];
        default:
            return [];
    }
}

function formatParameterValue(key, value) {
    // Format special values for better display
    switch(key) {
        case 'TIMESTAMP':
            return new Date(value).toLocaleString();
        case 'PRUNST1':
            return value === '1' ? 'Running' : 'Stopped';
        case 'FLASH':
        case 'RFCARD':
        case 'SDCARD':
        case 'GPS':
            return value === '1' ? 'OK' : 'Error';
        case 'DI11':
        case 'DI21':
        case 'DI31':
        case 'DI41':
        case 'DO11':
        case 'DO21':
        case 'DO31':
        case 'DO41':
            return value === '1' ? 'High' : 'Low';
        case 'LAT':
        case 'LONG':
            return parseFloat(value).toFixed(4) + '°';
        case 'VBATT':
        case 'BTVOLT':
        case 'POPKW1':
        case 'PDKWH1':
        case 'PTOTKWH1':
        case 'POPCUR1':
        case 'AI11':
        case 'AI21':
        case 'AI31':
        case 'AI41':
            return parseFloat(value).toFixed(2);
        case 'CRSSI':
            return value + ' dBm';
        case 'TEMP':
            return value + '°C';
        case 'POPFREQ1':
        case 'PMAXFREQ1':
        case 'PFREQLSP1':
        case 'PFREQHSP1':
            return value + ' Hz';
        case 'POPVOLT1':
            return value + ' V';
        case 'POPFLW1':
            return value + ' L/min';
        case 'POPDWD1':
        case 'POPTOTWD1':
            return value + ' L';
        default:
            return value;
    }
}

function updateRawDataDisplay() {
    updateRawDataTab('raw-heartbeat');
    updateRawDataTab('raw-data');
    updateRawDataTab('raw-daq');
}

function updateRawDataTab(tabName) {
    let dataType, listId;
    
    switch(tabName) {
        case 'raw-heartbeat':
            dataType = 'heartbeat';
            listId = 'raw-heartbeat-list';
            break;
        case 'raw-data':
            dataType = 'pump';
            listId = 'raw-data-list';
            break;
        case 'raw-daq':
            dataType = 'daq';
            listId = 'raw-daq-list';
            break;
        default:
            return;
    }
    
    const list = document.getElementById(listId);
    const data = appState.data[dataType];
    
    if (data.length === 0) {
        list.innerHTML = `<div class="no-data">No raw ${dataType} data received yet</div>`;
        return;
    }
    
    // Show last 20 entries in reverse order (newest first)
    const recentData = data.slice(-20).reverse();
    
    let html = '';
    recentData.forEach(item => {
        html += `
            <div class="raw-data-item">
                <div class="raw-timestamp">${item.timestamp}</div>
                <div class="raw-json">${JSON.stringify(item.data, null, 2)}</div>
            </div>
        `;
    });
    
    list.innerHTML = html;
}

function updateDataCounts() {
    document.getElementById('heartbeat-count').textContent = appState.data.heartbeat.length;
    document.getElementById('pump-count').textContent = appState.data.pump.length;
    document.getElementById('daq-count').textContent = appState.data.daq.length;
}

// UX Dashboard Functions
function updateUxDashboard() {
    updateUxParameters();
    updateGauges();
    updateIoMatrix();
    updateEnergyCards();
}

function updateUxParameters() {
    // Update basic parameters
    if (appState.data.heartbeat.length > 0) {
        const latest = appState.data.heartbeat[appState.data.heartbeat.length - 1].data;
        document.getElementById('ux-rssi').textContent = `${latest.CRSSI} dBm`;
        document.getElementById('ux-imei').textContent = latest.IMEI;
    }
}

// Enhanced UX Dashboard Functions (mirroring Android app sophistication)
function updateUxDashboard() {
    updateUxParameters();
    updateAdvancedGauges();
    updateIoMatrix();
    updateEnergyCards();
    updateConnectionStatus();
}

function updateUxParameters() {
    // Update basic parameters with freshness indicators
    if (appState.data.heartbeat.length > 0) {
        const latest = appState.data.heartbeat[appState.data.heartbeat.length - 1];
        const data = latest.data;
        const isFresh = (Date.now() - new Date(latest.timestamp).getTime()) < 30000; // 30 seconds freshness
        
        document.getElementById('ux-rssi').textContent = `${data.CRSSI} dBm`;
        document.getElementById('ux-imei').textContent = data.IMEI;
        
        // Update freshness indicators
        updateValueFreshness('ux-rssi', isFresh);
        updateValueFreshness('ux-imei', isFresh);
    }
}

// Enhanced Gauge System with Android-like Smooth Animations
const gaugeAnimations = new Map(); // Store animation states for each gauge

function updateAdvancedGauges() {
    const currentTime = Date.now();
    
    // Update gauges with enhanced animation system
    if (appState.data.heartbeat.length > 0) {
        const heartbeat = appState.data.heartbeat[appState.data.heartbeat.length - 1];
        const data = heartbeat.data;
        const isFresh = (currentTime - new Date(heartbeat.timestamp).getTime()) < 30000;
        
    const vbattVal = parseFloat(data.VBATT ?? data.BTVOLT);
    animateGaugeValue('battery-gauge', vbattVal, 10, 15, 'V', isFresh, { warning: 11.5, critical: 11.0 });
        animateGaugeValue('temperature-gauge', parseFloat(data.TEMP), 0, 50, '°C', isFresh, { warning: 40, critical: 45 });
        
        if (!isNaN(vbattVal)) {
            updateGaugeValue('battery-value', `${vbattVal.toFixed(2)}V`, isFresh);
        }
        updateGaugeValue('temperature-value', `${data.TEMP}°C`, isFresh);
    }
    
    if (appState.data.pump.length > 0) {
        const pump = appState.data.pump[appState.data.pump.length - 1];
        const data = pump.data;
        const isFresh = (currentTime - new Date(pump.timestamp).getTime()) < 30000;
        
        animateGaugeValue('frequency-gauge', parseFloat(data.POPFREQ1), 0, 60, 'Hz', isFresh, { warning: 55, critical: 58 });
        animateGaugeValue('power-gauge', parseFloat(data.POPKW1), 0, 5, 'kW', isFresh, { warning: 4, critical: 4.5 });
        animateGaugeValue('flow-gauge', parseFloat(data.POPFLW1), 0, 200, 'L/min', isFresh, null);
        animateGaugeValue('current-gauge', parseFloat(data.POPCUR1), 0, 10, 'A', isFresh, { warning: 8, critical: 9 });
        
        updateGaugeValue('frequency-value', `${data.POPFREQ1} Hz`, isFresh);
        updateGaugeValue('power-value', `${data.POPKW1} kW`, isFresh);
        updateGaugeValue('flow-value', `${data.POPFLW1} L/min`, isFresh);
        updateGaugeValue('current-value', `${data.POPCUR1} A`, isFresh);
        
        // Update pump status with smart color coding
        const pumpStatus = data.PRUNST1 === '1' ? 'Running' : 'Stopped';
        const pumpElement = document.getElementById('pump-status');
        if (pumpElement) {
            pumpElement.textContent = pumpStatus;
            pumpElement.className = `status-indicator ${data.PRUNST1 === '1' ? 'running' : 'stopped'} ${isFresh ? 'fresh' : 'stale'}`;
        }
    }
}

// Advanced Animation Function (mirroring Android's animateFloatAsState)
function animateGaugeValue(canvasId, targetValue, min, max, unit, isFresh, thresholds) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    
    // Initialize or get existing animation state
    if (!gaugeAnimations.has(canvasId)) {
        gaugeAnimations.set(canvasId, {
            currentValue: targetValue,
            targetValue: targetValue,
            animationStartTime: Date.now(),
            duration: 1000, // 1 second like Android app
            isAnimating: false
        });
    }
    
    const animation = gaugeAnimations.get(canvasId);
    
    // Start new animation if target changed
    if (Math.abs(animation.targetValue - targetValue) > 0.01) {
        animation.currentValue = animation.currentValue || targetValue;
        animation.targetValue = targetValue;
        animation.animationStartTime = Date.now();
        animation.isAnimating = true;
        
        // Start animation loop
        animateGauge(canvasId, min, max, unit, isFresh, thresholds);
    }
}

// Smooth Animation Loop (EaseInOutCubic like Android)
function animateGauge(canvasId, min, max, unit, isFresh, thresholds) {
    const animation = gaugeAnimations.get(canvasId);
    if (!animation || !animation.isAnimating) return;
    
    const elapsed = Date.now() - animation.animationStartTime;
    const progress = Math.min(elapsed / animation.duration, 1);
    
    // EaseInOutCubic easing function (matching Android)
    const easeInOutCubic = (t) => {
        return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
    };
    
    const easedProgress = easeInOutCubic(progress);
    const currentValue = animation.currentValue + (animation.targetValue - animation.currentValue) * easedProgress;
    
    // Update gauge visual
    drawAdvancedGauge(canvasId, currentValue, min, max, unit, isFresh, thresholds);
    
    // Continue animation or finish
    if (progress < 1) {
        requestAnimationFrame(() => animateGauge(canvasId, min, max, unit, isFresh, thresholds));
    } else {
        animation.isAnimating = false;
        animation.currentValue = animation.targetValue;
    }
}

// Advanced Gauge Drawing (mirroring Android's visual sophistication)
function drawAdvancedGauge(canvasId, value, min, max, unit, isFresh, thresholds) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    
    const ctx = canvas.getContext('2d');
    const centerX = canvas.width / 2;
    const centerY = canvas.height / 2;
    const radius = Math.min(centerX, centerY) - 12;
    
    // Clear canvas
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    
    // Calculate normalized value and angle
    const normalizedValue = Math.max(0, Math.min(1, (value - min) / (max - min)));
    const angle = -Math.PI + (normalizedValue * Math.PI);
    
    // Determine colors based on freshness and thresholds (like Android app)
    let gaugeColor = '#2196F3'; // Default blue
    let alpha = isFresh ? 1.0 : 0.6; // Freshness alpha like Android
    
    if (thresholds) {
        if (thresholds.critical && value >= thresholds.critical) {
            gaugeColor = '#f44336'; // Red for critical
        } else if (thresholds.warning && value >= thresholds.warning) {
            gaugeColor = '#ff9800'; // Orange for warning
        } else {
            gaugeColor = isFresh ? '#4caf50' : '#2196F3'; // Green when fresh and normal
        }
    }
    
    // Background arc (stale appearance)
    ctx.beginPath();
    ctx.arc(centerX, centerY, radius, -Math.PI, 0);
    ctx.strokeStyle = isFresh ? 'rgba(51, 51, 51, 0.2)' : 'rgba(51, 51, 51, 0.3)';
    ctx.lineWidth = 6; // Thinner like Android improvements
    ctx.lineCap = 'round';
    ctx.stroke();
    
    // Value arc with color and alpha
    if (normalizedValue > 0) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, radius, -Math.PI, angle);
        ctx.strokeStyle = `${gaugeColor}${Math.floor(alpha * 255).toString(16).padStart(2, '0')}`;
        ctx.lineWidth = 6;
        ctx.lineCap = 'round';
        ctx.stroke();
    }
    
    // Value indicator dot (like Android)
    const dotX = centerX + radius * Math.cos(angle);
    const dotY = centerY + radius * Math.sin(angle);
    
    ctx.beginPath();
    ctx.arc(dotX, dotY, 3, 0, 2 * Math.PI);
    ctx.fillStyle = `${gaugeColor}${Math.floor(alpha * 255).toString(16).padStart(2, '0')}`;
    ctx.fill();
    
    // Draw minimal scale markers (like Android compact gauges)
    drawScaleMarkers(ctx, centerX, centerY, radius, isFresh);
}

// Scale Markers (matching Android's minimal approach)
function drawScaleMarkers(ctx, centerX, centerY, radius, isFresh) {
    const markerCount = 3; // Minimal markers like Android compact gauges
    const markerLength = 2;
    const markerRadius = radius + 4;
    
    ctx.strokeStyle = isFresh ? 'rgba(128, 128, 128, 0.4)' : 'rgba(128, 128, 128, 0.2)';
    ctx.lineWidth = 0.5;
    
    for (let i = 0; i <= markerCount; i++) {
        const angle = Math.PI + (i * Math.PI / markerCount);
        const startX = centerX + radius * Math.cos(angle);
        const startY = centerY + radius * Math.sin(angle);
        const endX = centerX + markerRadius * Math.cos(angle);
        const endY = centerY + markerRadius * Math.sin(angle);
        
        ctx.beginPath();
        ctx.moveTo(startX, startY);
        ctx.lineTo(endX, endY);
        ctx.stroke();
    }
}

// Value Display Update with Freshness (like Android UxValue)
function updateGaugeValue(elementId, text, isFresh) {
    const element = document.getElementById(elementId);
    if (element) {
        element.textContent = text;
        element.className = `gauge-value ${isFresh ? 'fresh' : 'stale'}`;
    }
}

function updateValueFreshness(elementId, isFresh) {
    const element = document.getElementById(elementId);
    if (element) {
        element.className = `parameter-value ${isFresh ? 'fresh' : 'stale'}`;
    }
}

// Legacy function for backward compatibility
function updateGauges() {
    updateAdvancedGauges();
}

// Enhanced I/O Matrix with Freshness Indicators (Android-like)
function updateIoMatrix() {
    const currentTime = Date.now();
    
    if (appState.data.daq.length > 0) {
        const daq = appState.data.daq[appState.data.daq.length - 1];
        const data = daq.data;
        const isFresh = (currentTime - new Date(daq.timestamp).getTime()) < 30000;
        
        // Update analog inputs with freshness
        const analogInputs = ['AI11', 'AI21', 'AI31', 'AI41'];
        analogInputs.forEach((input, index) => {
            const element = document.getElementById(`ai${index + 1}`);
            if (element) {
                element.textContent = `${data[input]}V`;
                element.parentElement.className = `io-item analog ${isFresh ? 'fresh' : 'stale'}`;
            }
        });
        
        // Update digital I/O with enhanced status
        const digitalInputs = ['DI11', 'DI21', 'DI31', 'DI41'];
        const digitalOutputs = ['DO11', 'DO21', 'DO31', 'DO41'];
        
        document.querySelectorAll('.io-item.digital').forEach((item, index) => {
            if (index < 4) {
                // Digital inputs
                const value = data[digitalInputs[index]];
                const isActive = value === '1';
                item.className = `io-item digital ${isActive ? 'active' : 'inactive'} ${isFresh ? 'fresh' : 'stale'}`;
            } else {
                // Digital outputs
                const value = data[digitalOutputs[index - 4]];
                const isActive = value === '1';
                item.className = `io-item digital ${isActive ? 'active' : 'inactive'} ${isFresh ? 'fresh' : 'stale'}`;
            }
        });
    } else {
        // No data - mark all as stale
        document.querySelectorAll('.io-item').forEach(item => {
            item.classList.add('stale');
        });
    }
}

// Enhanced Energy Cards with Freshness (Android-style)
function updateEnergyCards() {
    const currentTime = Date.now();
    
    if (appState.data.pump.length > 0) {
        const pump = appState.data.pump[appState.data.pump.length - 1];
        const data = pump.data;
        const isFresh = (currentTime - new Date(pump.timestamp).getTime()) < 30000;
        
        const energyValues = [
            { id: 'daily-energy', value: `${data.PDKWH1} kWh` },
            { id: 'total-energy', value: `${data.PTOTKWH1} kWh` },
            { id: 'daily-water', value: `${data.POPDWD1} L` },
            { id: 'total-water', value: `${data.POPTOTWD1} L` }
        ];
        
        energyValues.forEach(({ id, value }) => {
            const element = document.getElementById(id);
            if (element) {
                element.textContent = value;
                element.className = `energy-value ${isFresh ? 'fresh' : 'stale'}`;
            }
        });
    } else {
        // No data - mark all as stale
        const energyIds = ['daily-energy', 'total-energy', 'daily-water', 'total-water'];
        energyIds.forEach(id => {
            const element = document.getElementById(id);
            if (element) {
                element.className = 'energy-value stale';
            }
        });
    }
}

// Connection Status Update (mirroring Android's communication hub)
function updateConnectionStatus() {
    const currentTime = Date.now();
    let connectionStatus = 'offline';
    let lastDataTime = 0;
    
    // Determine connection status based on recent data
    const allDataTimes = [
        ...appState.data.heartbeat.map(d => new Date(d.timestamp).getTime()),
        ...appState.data.pump.map(d => new Date(d.timestamp).getTime()),
        ...appState.data.daq.map(d => new Date(d.timestamp).getTime())
    ];
    
    if (allDataTimes.length > 0) {
        lastDataTime = Math.max(...allDataTimes);
        const timeSinceLastData = currentTime - lastDataTime;
        
        if (appState.mqttConnected) {
            if (timeSinceLastData < 30000) { // 30 seconds
                connectionStatus = 'online';
            } else if (timeSinceLastData < 60000) { // 1 minute
                connectionStatus = 'connecting';
            } else {
                connectionStatus = 'offline';
            }
        } else {
            connectionStatus = 'offline';
        }
    }
    
    // Update connection indicator
    const connectionElement = document.querySelector('.connection-status-text');
    const connectionIcon = document.querySelector('.connection-icon');
    
    if (connectionElement) {
        const statusText = {
            'online': 'Online',
            'connecting': 'Connecting',
            'offline': 'Offline'
        };
        connectionElement.textContent = statusText[connectionStatus] || 'Unknown';
    }
    
    if (connectionIcon) {
        connectionIcon.className = `connection-icon ${connectionStatus}`;
        const iconText = {
            'online': '●',
            'connecting': '◐', 
            'offline': '○'
        };
        connectionIcon.textContent = iconText[connectionStatus] || '?';
    }
}

// Enhanced Animation System (Android-like smooth updates)
function startGaugeAnimations() {
    // Enhanced update cycle with multiple intervals for different update rates
    
    // Fast updates for gauges and real-time data (like Android's fast refresh)
    setInterval(() => {
        if (appState.currentScreen === 'ux-dashboard') {
            updateAdvancedGauges();
            updateConnectionStatus();
            
            // Update last update time
            const lastUpdateElement = document.getElementById('last-update-time');
            if (lastUpdateElement) {
                lastUpdateElement.textContent = new Date().toLocaleTimeString();
            }
        }
    }, 1000); // 1 second updates like Android app
    
    // Medium updates for I/O and energy data
    setInterval(() => {
        if (appState.currentScreen === 'ux-dashboard') {
            updateIoMatrix();
            updateEnergyCards();
        }
    }, 2000); // 2 second updates
    
    // Slow updates for parameters and connection status
    setInterval(() => {
        if (appState.currentScreen === 'ux-dashboard') {
            updateUxParameters();
        }
    }, 5000); // 5 second updates
}

// Enhanced UX Dashboard initialization
function initializeUxDashboard() {
    // Initialize all gauges with default values
    const defaultGauges = [
        { id: 'battery-gauge', value: 12.5, min: 10, max: 15, unit: 'V', thresholds: { warning: 11.5, critical: 11.0 } },
        { id: 'temperature-gauge', value: 25, min: 0, max: 50, unit: '°C', thresholds: { warning: 40, critical: 45 } },
        { id: 'frequency-gauge', value: 50, min: 0, max: 60, unit: 'Hz', thresholds: { warning: 55, critical: 58 } },
        { id: 'power-gauge', value: 2.5, min: 0, max: 5, unit: 'kW', thresholds: { warning: 4, critical: 4.5 } },
        { id: 'flow-gauge', value: 150, min: 0, max: 200, unit: 'L/min', thresholds: null },
        { id: 'current-gauge', value: 5.2, min: 0, max: 10, unit: 'A', thresholds: { warning: 8, critical: 9 } }
    ];
    
    // Initialize all gauges with smooth animation
    defaultGauges.forEach(gauge => {
        try {
            const canvas = document.getElementById(gauge.id);
            if (canvas && canvas.getContext) {
                drawAdvancedGauge(gauge.id, gauge.value, gauge.min, gauge.max, gauge.unit, true, gauge.thresholds);
            } else {
                console.warn(`Gauge canvas not found or not ready: ${gauge.id}`);
            }
        } catch (error) {
            console.error(`Error initializing gauge ${gauge.id}:`, error);
        }
    });
    
    console.log('UX Dashboard initialized with Android-like smooth animations');
}

// Export Functions
function exportData() {
    if (appState.data.heartbeat.length === 0 && 
        appState.data.pump.length === 0 && 
        appState.data.daq.length === 0) {
        showToast('No data to export', 'warning');
        return;
    }
    
    // Create CSV content for each data type
    const csvFiles = [];
    
    if (appState.data.heartbeat.length > 0) {
        csvFiles.push({
            name: 'heartbeat_data.csv',
            content: generateCSV(appState.data.heartbeat, 'heartbeat')
        });
    }
    
    if (appState.data.pump.length > 0) {
        csvFiles.push({
            name: 'pump_data.csv',
            content: generateCSV(appState.data.pump, 'pump')
        });
    }
    
    if (appState.data.daq.length > 0) {
        csvFiles.push({
            name: 'daq_data.csv',
            content: generateCSV(appState.data.daq, 'daq')
        });
    }
    
    // Download files
    csvFiles.forEach(file => {
        downloadCSV(file.content, file.name);
    });
    
    showToast(`Exported ${csvFiles.length} CSV files`, 'success');
}

function generateCSV(data, type) {
    if (data.length === 0) return '';
    
    const headers = ['Timestamp', 'JSON Data'];
    let csv = headers.join(',') + '\n';
    
    data.forEach(item => {
        const row = [
            `"${item.timestamp}"`,
            `"${JSON.stringify(item.data).replace(/"/g, '""')}"`
        ];
        csv += row.join(',') + '\n';
    });
    
    return csv;
}

function downloadCSV(content, filename) {
    const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    
    link.setAttribute('href', url);
    link.setAttribute('download', filename);
    link.style.visibility = 'hidden';
    
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
}

// Toast Notification Functions
function showToast(message, type = 'info') {
    try {
        const container = document.getElementById('toast-container');
        if (!container) {
            console.warn('Toast container not found');
            return;
        }
        
        // Limit number of toasts to prevent overlap
        const existingToasts = container.querySelectorAll('.toast');
        if (existingToasts.length >= 3) {
            // Remove oldest toast
            existingToasts[0].remove();
        }
        
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.textContent = message;
        
        container.appendChild(toast);
        
        // Auto-remove toast after 3 seconds
        setTimeout(() => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        }, 3000);
        
        // Add click to dismiss
        toast.addEventListener('click', () => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        });
    } catch (error) {
        console.error('Error showing toast:', error);
    }
}
