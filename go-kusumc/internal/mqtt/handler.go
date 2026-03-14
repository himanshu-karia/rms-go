package mqtt

import (
	"encoding/json"
	"ingestion-go/internal/pipeline"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client     mqtt.Client
	WorkerPool *pipeline.WorkerPool
}

func NewClient(brokerURL string, wp *pipeline.WorkerPool) *Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("go-ingestion-service-v2")
	if user := os.Getenv("SERVICE_MQTT_USERNAME"); user != "" {
		opts.SetUsername(user)
	}
	if pass := os.Getenv("SERVICE_MQTT_PASSWORD"); pass != "" {
		opts.SetPassword(pass)
	}
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)

	// Define Handler
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		// Non-blocking push if possible, or blocking if full?
		// For high throughput, we want backpressure, so blocking is okay-ish, or drop.
		// Ideally we use a very large buffer.
		select {
		case wp.JobChan <- msg.Payload():
			// Success
		default:
			log.Println("⚠️ Worker Channel Full! Dropping packet.")
		}
	})

	opts.OnConnect = func(c mqtt.Client) {
		log.Println("✅ Connected to MQTT Broker")
		// Subscribe to same topic as Node.js for A/B testing
		// Using Shared Subscription would be better for load balancing, but for now simple Sub.
		token := c.Subscribe("telemetry/+", 1, nil)
		if token.Wait() && token.Error() != nil {
			log.Printf("❌ Subscribe Error: %v", token.Error())
		} else {
			log.Println("✅ Subscribed to telemetry/+")
		}
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("❌ MQTT Connection Lost: %v", err)
	}

	c := mqtt.NewClient(opts)
	return &Client{
		client:     c,
		WorkerPool: wp,
	}
}

func (c *Client) Connect() error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// Publish wraps the paho Publish method for the WorkerPool
func (c *Client) Publish(topic string, payload interface{}) {
	text, _ := json.Marshal(payload)
	token := c.client.Publish(topic, 0, false, text)
	// We don't wait for token to maximize throughput (Fire & Forget)
	if token.Error() != nil {
		log.Printf("❌ Publish Error: %v", token.Error())
	}
}
