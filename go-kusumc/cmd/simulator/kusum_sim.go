//go:build kusum
// +build kusum

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	baseURL := flag.String("base", "http://localhost:8081", "Backend base URL")
	mqttURL := flag.String("mqtt", "tcp://localhost:1884", "MQTT broker URL")
	projectID := flag.String("project", "pm-kusum-solar-pump-msedcl", "Project ID")
	imei := flag.String("imei", fmt.Sprintf("999%011d", rand.Int63n(1_000_000_00000)), "IMEI to simulate")
	protocolID := flag.String("protocol", "rms-v1", "Protocol ID")
	skipProvision := flag.Bool("skip-provision", false, "Skip provisioning (assumes device already exists)")
	flag.Parse()

	deviceID := ""

	if !*skipProvision {
		reqBody := map[string]interface{}{
			"name":            "sim-kusum-device",
			"imei":            *imei,
			"projectId":       *projectID,
			"protocol_id":     *protocolID,
			"contractor_id":   "",
			"supplier_id":     "",
			"manufacturer_id": "",
		}
		buf, _ := json.Marshal(reqBody)
		resp, err := http.Post(fmt.Sprintf("%s/api/devices", *baseURL), "application/json", bytes.NewReader(buf))
		if err != nil {
			log.Fatalf("provision request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Fatalf("provision failed: status=%d", resp.StatusCode)
		}
		var provisioned struct {
			DeviceID string `json:"device_id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&provisioned); err != nil {
			log.Fatalf("decode provision response: %v", err)
		}
		deviceID = provisioned.DeviceID
		log.Printf("Provisioned device: imei=%s device_id=%s", *imei, deviceID)
	}

	if deviceID == "" {
		deviceID = "existing-device"
	}

	opts := mqtt.NewClientOptions().AddBroker(*mqttURL).SetClientID("kusum-sim-" + *imei)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("mqtt connect error: %v", token.Error())
	}
	defer client.Disconnect(200)

	msgID := fmt.Sprintf("sim-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  *projectID,
		"protocol_id": *protocolID,
		"device_id":   deviceID,
		"imei":        *imei,
		"IMEI":        *imei,
		"ts":          time.Now().Unix(),
		"msg_id":      msgID,
		"msgid":       msgID,
		"TIMESTAMP":   time.Now().UTC().Format(time.RFC3339),
		"RSSI":        -64,
		"GPS":         "0",
		"TEMP":        29.4,
	}

	payloadBytes, _ := json.Marshal(payload)
	topic := fmt.Sprintf("%s/heartbeat", *imei)
	token := client.Publish(topic, 1, false, payloadBytes)
	token.Wait()
	if token.Error() != nil {
		log.Fatalf("mqtt publish error: %v", token.Error())
	}

	log.Printf("Published heartbeat to %s", topic)
}
