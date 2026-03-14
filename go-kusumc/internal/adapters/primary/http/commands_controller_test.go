package http

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gofiber/fiber/v2"

	"ingestion-go/internal/core/domain"
	"ingestion-go/internal/core/services"
	"ingestion-go/tests/mocks"
)

func TestCommandsController_GetCatalog_MissingDevice(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}
	svc := services.NewCommandsService(cmdRepo, deviceRepo)

	ctrl := NewCommandsController(svc)
	app := fiber.New()
	app.Get("/api/commands/catalog", ctrl.GetCatalog)

	req := httptest.NewRequest("GET", "/api/commands/catalog", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCommandsController_SendCommand_Success(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}

	// Device lookup
	deviceRepo.GetDeviceByIDOrIMEIFunc = func(idOrIMEI string) (map[string]interface{}, error) {
		return map[string]interface{}{"id": "dev-1", "imei": "12345", "project_id": "proj-1"}, nil
	}

	// Command lookup and allowed list
	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Name: "ping", Scope: "core", Transport: "mqtt"}, nil
	}
	cmdRepo.ListCommandsForDeviceFunc = func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
		return []domain.CommandCatalog{{ID: "cmd-1"}}, nil
	}

	// Persisting request
	cmdRepo.InsertCommandRequestFunc = func(req domain.CommandRequest) (string, error) {
		return "req-1", nil
	}
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		return nil
	}
	cmdRepo.ListCommandRequestsFunc = func(deviceID string, limit int) ([]domain.CommandRequest, error) { return nil, nil }

	svc := services.NewCommandsService(cmdRepo, deviceRepo)
	// Use a simple stub publisher so publish succeeds
	svc.SetMqttClient(&stubPublisher{})

	ctrl := NewCommandsController(svc)
	app := fiber.New()
	app.Post("/api/commands/send", ctrl.SendCommand)

	body := map[string]interface{}{
		"deviceId":  "dev-1",
		"commandId": "cmd-1",
		"projectId": "proj-1",
		"payload":   map[string]interface{}{"foo": "bar"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/commands/send", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out domain.CommandRequest
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ID != "req-1" {
		t.Fatalf("expected req id 'req-1', got '%s'", out.ID)
	}
}

// Minimal stub publisher used by service in tests
type stubPublisher struct{}

func (p *stubPublisher) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	return &stubToken{}
}

// Reuse stubToken from service tests
type stubToken struct{}

func (t *stubToken) Wait() bool                       { return true }
func (t *stubToken) WaitTimeout(d time.Duration) bool { return true }
func (t *stubToken) Error() error                     { return nil }
func (t *stubToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
