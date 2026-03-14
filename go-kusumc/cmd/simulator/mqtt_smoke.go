package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := flag.String("broker", "tcp://localhost:1884", "MQTT broker URL")
	username := flag.String("username", "", "MQTT username")
	password := flag.String("password", "", "MQTT password")
	clientID := flag.String("client-id", "", "MQTT client ID (defaults to imei)")
	projectID := flag.String("project", "project_04_tank", "Project ID")
	imei := flag.String("imei", "test-imei", "Device IMEI")
	protocolID := flag.String("protocol", "", "Protocol ID")
	deviceID := flag.String("device-id", "", "Device ID")
	waitCmd := flag.Bool("sub", false, "Subscribe to command topic after publish")
	flag.Parse()

	if *clientID == "" {
		*clientID = *imei
	}

	msgID := randomID()
	payload := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  *projectID,
		"protocol_id": *protocolID,
		"device_id":   *deviceID,
		"imei":        *imei,
		"ts":          time.Now().Unix(),
		"msg_id":      msgID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("marshal payload: %v", err)
	}

	opts := mqtt.NewClientOptions().AddBroker(*broker).SetClientID(*clientID)
	if *username != "" {
		opts.SetUsername(*username)
	}
	if *password != "" {
		opts.SetPassword(*password)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("mqtt connect: %v", token.Error())
	}
	defer client.Disconnect(200)

	topicPub := fmt.Sprintf("%s/heartbeat", *imei)
	token := client.Publish(topicPub, 1, false, payloadBytes)
	token.Wait()
	if token.Error() != nil {
		log.Fatalf("publish: %v", token.Error())
	}
	log.Printf("Published heartbeat to %s (msg_id=%s)", topicPub, msgID)

	if !*waitCmd {
		return
	}

	topicSub := fmt.Sprintf("%s/ondemand", *imei)
	recv := make(chan struct{})
	if token := client.Subscribe(topicSub, 1, func(_ mqtt.Client, msg mqtt.Message) {
		log.Printf("Received command on %s: %s", topicSub, string(msg.Payload()))
		close(recv)
	}); token.Wait() && token.Error() != nil {
		log.Fatalf("subscribe: %v", token.Error())
	}

	log.Printf("Subscribed to %s; waiting up to 5s for a command...", topicSub)
	select {
	case <-recv:
	case <-time.After(5 * time.Second):
		log.Printf("No command received in window")
	}
}

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("rand-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
