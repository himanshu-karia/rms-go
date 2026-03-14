package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Minimal MQTT loopback helper: listens for commands and publishes a response with the same correlation_id.
// Env vars (flags override env):
//
//	MQTT_HOST (default: localhost)
//	MQTT_PORT (default: 1883)
//	MQTT_USERNAME / MQTT_PASSWORD (optional)
//	LOOP_PROJECT (required)
//	LOOP_IMEI (required)
//
// Response payload shape matches ingestion expectations: {"correlation_id":..., "status":"OK"}
func main() {
	host := getEnv("MQTT_HOST", "localhost")
	port := getEnv("MQTT_PORT", "1883")
	user := os.Getenv("MQTT_USERNAME")
	pass := os.Getenv("MQTT_PASSWORD")
	project := getEnv("LOOP_PROJECT", "")
	imei := getEnv("LOOP_IMEI", "")

	flag.StringVar(&host, "host", host, "mqtt host")
	flag.StringVar(&port, "port", port, "mqtt port")
	flag.StringVar(&project, "project", project, "project id")
	flag.StringVar(&imei, "imei", imei, "device imei")
	flag.Parse()

	if project == "" || imei == "" {
		log.Fatalf("project and imei required (set LOOP_PROJECT/LOOP_IMEI or flags)")
	}

	broker := fmt.Sprintf("tcp://%s:%s", host, port)
	client := newClient(broker, user, pass)

	topicCmd := fmt.Sprintf("%s/ondemand", imei)
	topicResp := topicCmd

	client.Subscribe(topicCmd, 1, func(_ mqtt.Client, msg mqtt.Message) {
		var m map[string]interface{}
		_ = json.Unmarshal(msg.Payload(), &m)
		corr, _ := m["correlation_id"].(string)
		cmdID, _ := m["command_id"].(string)
		log.Printf("recv cmd topic=%s command_id=%s correlation=%s", msg.Topic(), cmdID, corr)

		resp := map[string]interface{}{
			"correlation_id": corr,
			"status":         "OK",
			"ts":             time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(resp)
		token := client.Publish(topicResp, 1, false, data)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Printf("publish resp error: %v", err)
		} else {
			log.Printf("published resp to %s", topicResp)
		}
	})

	log.Printf("loopback listening on %s, project=%s imei=%s", topicCmd, project, imei)
	select {}
}

func newClient(broker, user, pass string) mqtt.Client {
	opts := mqtt.NewClientOptions().AddBroker(broker)
	opts.SetClientID(fmt.Sprintf("loopback-%d", time.Now().UnixNano()))
	if user != "" {
		opts.SetUsername(user)
	}
	if pass != "" {
		opts.SetPassword(pass)
	}
	opts.SetAutoReconnect(true)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("mqtt connect failed: %v", token.Error())
	}
	return client
}

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
