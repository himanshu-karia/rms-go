package services

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type WebhookService struct {
	client *http.Client
}

func NewWebhookService() *WebhookService {
	return &WebhookService{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Dispatch sends payload to all subscribers (Simulated DB lookup)
func (s *WebhookService) Dispatch(event string, payload interface{}, projectId string) {
	// 1. Fetch Subscribers (Mocked for V1 MVP)
	// subs := s.repo.GetSubscribers(event, projectId)
	// For MVP, we log or assume hardcoded dev webhook

	// log.Printf("📡 Webhook: Dispatching '%s' (Proj: %s)", event, projectId)

	// 2. Fire and Forget
	// go s.send(...)
}

func (s *WebhookService) Send(url, event string, payload interface{}) {
	body := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now(),
		"payload":   payload,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Unified-IoT-Portal-Ingestion/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("❌ Webhook Failed [%s]: %v", url, err)
		return
	}
	defer resp.Body.Close()
}
