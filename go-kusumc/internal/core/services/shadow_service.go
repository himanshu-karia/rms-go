package services

import (
	"encoding/json"
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

type ShadowService struct {
	repo   *secondary.PostgresRepo
	client mqtt.Client
}

func NewShadowService(repo *secondary.PostgresRepo) *ShadowService {
	return &ShadowService{repo: repo}
}

func (s *ShadowService) Start() {
	// 1. Config (Reuse Env Vars)
	broker := os.Getenv("MQTT_HOST")
	port := os.Getenv("MQTT_PORT")
	user := os.Getenv("SERVICE_MQTT_USERNAME")
	pass := os.Getenv("SERVICE_MQTT_PASSWORD")
	if broker == "" {
		broker = "localhost"
	}
	if port == "" {
		port = "1883"
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID("go_shadow_service_" + uuid.New().String()[:8]) // Unique ID
	if user != "" {
		opts.SetUsername(user)
	}
	if pass != "" {
		opts.SetPassword(pass)
	}
	opts.SetAutoReconnect(true)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Println("[ShadowService] Connected. Listening for Birth Messages...")
		c.Subscribe("device/+/connected", 1, s.handleBirth)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("[ShadowService] Connection failed: %v", token.Error())
		return
	}
	s.client = client
}

func (s *ShadowService) handleBirth(client mqtt.Client, msg mqtt.Message) {
	// Topic: device/{imei}/connected
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 2 {
		return
	}
	imei := parts[1]

	log.Printf("[ShadowService] Device Online: %s", imei)

	// Fetch Device Shadow
	// We need a specific method to get Shadow from DB.
	// Reuse GetDeviceByIMEI, assuming it fetches shadow (jsonb).
	dev, err := s.repo.GetDeviceByIMEI(imei)
	if err != nil || dev == nil {
		log.Printf("[ShadowService] Device %s not found in DB", imei)
		return
	}

	// Logic: Check Diff
	// For V1, we blindly PUSH if shadow exists.
	// In real app, we compare Reported vs Desired.

	dMap, ok := dev.(map[string]interface{})
	if !ok {
		return
	}

	// Check if "desired" exists in Shadow
	shadow, ok := dMap["shadow"].(map[string]interface{})
	if !ok || shadow == nil {
		return
	}

	desired, ok := shadow["desired"].(map[string]interface{})
	if !ok || len(desired) == 0 {
		return
	}

	// Prepare Sync Command
	log.Printf("[ShadowService] Pushing Shadow to %s", imei)

	cmdPayload := map[string]interface{}{
		"msgid":  uuid.New().String(),
		"cmd":    "SHADOW_SYNC",
		"params": desired,
		"ts":     time.Now().UnixMilli(),
	}

	data, _ := json.Marshal(cmdPayload)

	// Publish to legacy RMS ondemand topic.
	topic := fmt.Sprintf("%s/ondemand", imei)

	token := s.client.Publish(topic, 1, false, data)
	token.Wait()

	if token.Error() != nil {
		log.Printf("[ShadowService] Publish Failed: %v", token.Error())
	} else {
		log.Printf("[ShadowService] Sync Sent to %s", topic)
	}
}
