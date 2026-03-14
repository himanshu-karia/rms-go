package primary

import (
	"fmt"
	"ingestion-go/internal/core/services"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MqttHandler struct {
	ingest *services.IngestionService
	client mqtt.Client
}

func NewMqttHandler(ingest *services.IngestionService) *MqttHandler {
	return &MqttHandler{ingest: ingest}
}

// SetupClient instantiates the MQTT client with robust retry logic
func (h *MqttHandler) SetupClient() mqtt.Client {
	// 1. Config
	broker := os.Getenv("MQTT_HOST")
	port := os.Getenv("MQTT_PORT")
	user := os.Getenv("SERVICE_MQTT_USERNAME")
	pass := os.Getenv("SERVICE_MQTT_PASSWORD")
	if broker == "" {
		broker = "localhost" // Default for local dev
	}
	if port == "" {
		port = "1883"
	}

	// 2. Options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID("go_ingestion_engine")
	if user != "" {
		opts.SetUsername(user)
	}
	if pass != "" {
		opts.SetPassword(pass)
	}

	// --- ROBUSTNESS UPGRADES START ---

	// A. Auto Reconnect: Retries if connection drops AFTER successful connect
	opts.SetAutoReconnect(true)

	// B. Connect Retry: Retries if broker is down on INITIAL boot (The "Crash Loop" fix)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second) // Wait 5s between attempts

	// C. Keep Alive: Ping broker every 60s to prevent silent timeouts
	opts.SetKeepAlive(60 * time.Second)

	// --- ROBUSTNESS UPGRADES END ---

	// D. Handlers
	opts.SetOnConnectHandler(h.onConnect)
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("[MQTT] ⚠️ Connection Lost: %v. Retrying in background...", err)
	})

	h.client = mqtt.NewClient(opts)
	return h.client
}

func (h *MqttHandler) Start() {
	if h.client == nil {
		h.SetupClient()
	}

	log.Println("[MQTT] Attempting to connect...")

	// 3. Connect
	// With SetConnectRetry(true), this call initiates the connection loop.
	// We wait on the token to see if the CONFIGURATION is valid.
	// If the network is down, Paho will keep retrying in the background.
	token := h.client.Connect()

	// Wait up to 5 seconds for immediate connection, otherwise let it retry in background
	// This prevents the "Startup Hang" if EMQX is down.
	if token.WaitTimeout(5 * time.Second) {
		if token.Error() != nil {
			// This usually implies a configuration error (e.g., malformed URL)
			// rather than a network error (which is handled by retry).
			log.Fatalf("[MQTT] Critical Config Error: %v", token.Error())
		}
	} else {
		log.Println("[MQTT] Broker not yet reachable. Retrying in background...")
	}
}

func (h *MqttHandler) onConnect(client mqtt.Client) {
	log.Println("[MQTT] Connected! Subscribing...")
	// KUSUMC/RMS telemetry contract is per-device: <imei>/{heartbeat,data,daq,ondemand,errors}
	// Keep channels/* subscriptions for compatibility when a bridge is in place.
	// Compat subscriptions are disabled by default. Enable them only when needed by setting
	// MQTT_COMPAT_TOPICS_ENABLED=true.
	compatEnabled := false
	if v := strings.TrimSpace(os.Getenv("MQTT_COMPAT_TOPICS_ENABLED")); v != "" {
		compatEnabled = strings.EqualFold(v, "true") || strings.EqualFold(v, "1") || strings.EqualFold(v, "yes")
	}
	topics := map[string]byte{
		"+/heartbeat": 1,
		"+/data":      1,
		"+/daq":       1,
		"+/ondemand":  1,
		"+/errors":    1,
	}

	if compatEnabled {
		topics["channels/+/messages/+"] = 1
		topics["channels/+/messages/+/+"] = 1
		topics["devices/+/telemetry"] = 1
		topics["devices/+/telemetry/+"] = 1
		topics["devices/+/errors"] = 1
		topics["devices/+/errors/+"] = 1
		topics["channels/+/commands/+/resp"] = 1
		topics["channels/+/commands/+/ack"] = 1
	}

	if token := client.SubscribeMultiple(topics, h.handleMessage); token.Wait() && token.Error() != nil {
		log.Printf("[MQTT] Subscribe Error: %v", token.Error())
	} else {
		log.Println("[MQTT] Subscribed: +/heartbeat,+/data,+/daq,+/ondemand,+/errors")
		if compatEnabled {
			log.Println("[MQTT] Subscribed (compat): channels/+/messages/+[,/+], devices/+/telemetry[,/+], devices/+/errors[,/+], channels/+/commands/+/resp, channels/+/commands/+/ack")
		} else {
			log.Println("[MQTT] Compat subscriptions disabled (MQTT_COMPAT_TOPICS_ENABLED=false)")
		}
	}
}

func (h *MqttHandler) handleMessage(client mqtt.Client, msg mqtt.Message) {
	// log.Printf("[MQTT] RX: %s", msg.Topic())
	// Delegate to Ingestion Service
	// We pass topic as protocol metadata or just "mqtt"
	err := h.ingest.ProcessPacket(msg.Topic(), msg.Payload(), "")
	if err != nil {
		log.Printf("[MQTT] Process Error: %v", err)
	}
}
