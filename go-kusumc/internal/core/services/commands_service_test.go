package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"ingestion-go/internal/core/domain"
	"ingestion-go/tests/mocks"
)

type stubToken struct {
	err error
}

func (t *stubToken) Wait() bool                       { return true }
func (t *stubToken) WaitTimeout(d time.Duration) bool { return true }
func (t *stubToken) Error() error                     { return t.err }
func (t *stubToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

type stubPublisher struct {
	topic   string
	payload []byte
	err     error
}

func (p *stubPublisher) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	p.topic = topic
	if b, ok := payload.([]byte); ok {
		p.payload = b
	}
	return &stubToken{err: p.err}
}

func TestCommandsService_SendCommandSuccess(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}
	pub := &stubPublisher{}

	deviceRepo.GetDeviceByIDOrIMEIFunc = func(idOrIMEI string) (map[string]interface{}, error) {
		return map[string]interface{}{"id": "dev-1", "imei": "12345", "project_id": "proj-1"}, nil
	}
	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Name: "ping", Scope: "core", Transport: "mqtt"}, nil
	}
	cmdRepo.ListCommandsForDeviceFunc = func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
		return []domain.CommandCatalog{{ID: "cmd-1"}}, nil
	}
	cmdRepo.InsertCommandRequestFunc = func(req domain.CommandRequest) (string, error) {
		if req.Status != "queued" {
			t.Fatalf("expected queued status, got %s", req.Status)
		}
		return "req-1", nil
	}
	updated := struct {
		status string
		pubAt  *time.Time
	}{}
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		updated.status = status
		updated.pubAt = publishedAt
		return nil
	}
	cmdRepo.ListCommandRequestsFunc = func(deviceID string, limit int) ([]domain.CommandRequest, error) { return nil, nil }

	svc := NewCommandsService(cmdRepo, deviceRepo)
	svc.SetMqttClient(pub)

	req, err := svc.SendCommand("dev-1", "cmd-1", "", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatalf("expected request returned")
	}
	if req.ID != "req-1" {
		t.Fatalf("expected req ID req-1, got %s", req.ID)
	}
	if req.Status != "published" {
		t.Fatalf("expected status published, got %s", req.Status)
	}
	if updated.status != "published" || updated.pubAt == nil {
		t.Fatalf("expected published update with timestamp")
	}
	if pub.topic != "12345/ondemand" {
		t.Fatalf("unexpected topic: %s", pub.topic)
	}
	var payload map[string]any
	if err := json.Unmarshal(pub.payload, &payload); err != nil {
		t.Fatalf("payload not json: %v", err)
	}
	if payload["cmd"] != "ping" {
		t.Fatalf("expected cmd ping, got %v", payload["cmd"])
	}
	if payload["type"] != "ondemand_cmd" {
		t.Fatalf("expected type ondemand_cmd, got %v", payload["type"])
	}
	if v, ok := payload["msgid"].(string); !ok || v == "" {
		t.Fatalf("expected non-empty msgid, got %v", payload["msgid"])
	}
	if _, ok := payload["timestamp"]; !ok {
		t.Fatalf("expected timestamp field")
	}
	if payload["foo"] != "bar" {
		t.Fatalf("expected flattened payload field foo=bar, got %v", payload["foo"])
	}
}

func TestCommandsService_SendCommandPublishError(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}
	pub := &stubPublisher{err: errors.New("no conn")}

	deviceRepo.GetDeviceByIDOrIMEIFunc = func(idOrIMEI string) (map[string]interface{}, error) {
		return map[string]interface{}{"id": "dev-1", "imei": "12345", "project_id": "proj-1"}, nil
	}
	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Name: "ping", Scope: "core", Transport: "mqtt"}, nil
	}
	cmdRepo.ListCommandsForDeviceFunc = func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
		return []domain.CommandCatalog{{ID: "cmd-1"}}, nil
	}
	inserted := false
	cmdRepo.InsertCommandRequestFunc = func(req domain.CommandRequest) (string, error) {
		inserted = true
		return "req-2", nil
	}
	failedUpdate := ""
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		failedUpdate = status
		return nil
	}

	svc := NewCommandsService(cmdRepo, deviceRepo)
	svc.SetMqttClient(pub)

	req, err := svc.SendCommand("dev-1", "cmd-1", "", map[string]any{"foo": "bar"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !inserted {
		t.Fatalf("expected insert to run")
	}
	if failedUpdate != "failed" {
		t.Fatalf("expected failed status update, got %s", failedUpdate)
	}
	if req.Status != "failed" {
		t.Fatalf("expected failed status on req, got %s", req.Status)
	}
}

func TestCommandsService_RetryPublish(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}
	pub := &stubPublisher{}

	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Transport: "mqtt"}, nil
	}
	deviceRepo.GetDeviceByIDFunc = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{"id": id, "imei": "12345", "project_id": "proj-1"}, nil
	}
	var finalStatus string
	var finalRetries *int
	var finalPublishedAt *time.Time
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		// publishMQTT will call once (retries nil), RetryPublish will call again with retries bumped; capture the latter
		if retries != nil {
			finalStatus = status
			finalRetries = retries
			finalPublishedAt = publishedAt
		}
		return nil
	}

	svc := NewCommandsService(cmdRepo, deviceRepo)
	svc.SetMqttClient(pub)
	req := domain.CommandRequest{
		ID:            "req-3",
		DeviceID:      "dev-1",
		ProjectID:     "proj-1",
		CommandID:     "cmd-1",
		Payload:       map[string]any{"foo": "bar"},
		CorrelationID: "corr-1",
		Retries:       1,
	}

	if err := svc.RetryPublish(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finalStatus != "published" {
		t.Fatalf("expected published, got %s", finalStatus)
	}
	if finalRetries == nil || *finalRetries != 2 {
		t.Fatalf("expected retries bumped to 2, got %v", finalRetries)
	}
	if finalPublishedAt == nil {
		t.Fatalf("expected publishedAt set")
	}
}

func TestCommandsService_RetryByCorrelation(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}
	pub := &stubPublisher{}

	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Transport: "mqtt"}, nil
	}
	req := domain.CommandRequest{ID: "req-4", DeviceID: "dev-4", ProjectID: "proj-4", CommandID: "cmd-4", Payload: map[string]any{"foo": "bar"}, CorrelationID: "corr-4", Retries: 0}
	cmdRepo.GetCommandRequestByCorrelationFunc = func(correlationID string) (*domain.CommandRequest, error) {
		if correlationID != "corr-4" {
			return nil, nil
		}
		copy := req
		return &copy, nil
	}
	deviceRepo.GetDeviceByIDFunc = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{"id": id, "imei": "444", "project_id": "proj-4"}, nil
	}
	var updatedStatus string
	var updatedRetries *int
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		updatedStatus = status
		updatedRetries = retries
		return nil
	}

	svc := NewCommandsService(cmdRepo, deviceRepo)
	svc.SetMqttClient(pub)

	updated, err := svc.RetryByCorrelation("corr-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated == nil || updated.CorrelationID != "corr-4" {
		t.Fatalf("expected updated request")
	}
	if updatedStatus != "published" {
		t.Fatalf("expected published status, got %s", updatedStatus)
	}
	if updatedRetries == nil || *updatedRetries != 1 {
		t.Fatalf("expected retries incremented to 1, got %v", updatedRetries)
	}
}

func TestCommandsService_SendCommandHTTP(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}

	deviceRepo.GetDeviceByIDOrIMEIFunc = func(idOrIMEI string) (map[string]interface{}, error) {
		return map[string]interface{}{"id": "dev-http", "imei": "12345", "project_id": "proj-http"}, nil
	}
	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Name: "httpcmd", Scope: "core", Transport: "http"}, nil
	}
	cmdRepo.ListCommandsForDeviceFunc = func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
		return []domain.CommandCatalog{{ID: "cmd-http"}}, nil
	}
	cmdRepo.InsertCommandRequestFunc = func(req domain.CommandRequest) (string, error) {
		return "req-http", nil
	}

	var updatedStatus string
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		updatedStatus = status
		return nil
	}

	called := false
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	svc := NewCommandsService(cmdRepo, deviceRepo)
	svc.SetHTTPClient(server.Client(), server.URL)

	req, err := svc.SendCommand("dev-http", "cmd-http", "", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected http transport to be invoked")
	}
	if req == nil || req.Status != "published" {
		t.Fatalf("expected published request, got %+v", req)
	}
	if updatedStatus != "published" {
		t.Fatalf("expected status update to published")
	}
	if captured == nil || captured["correlationId"] == "" {
		t.Fatalf("correlationId missing in http payload")
	}
}

func TestCommandsService_SendCommandHTTP_Failure(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}

	deviceRepo.GetDeviceByIDOrIMEIFunc = func(idOrIMEI string) (map[string]interface{}, error) {
		return map[string]interface{}{"id": "dev-http", "imei": "12345", "project_id": "proj-http"}, nil
	}
	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Name: "httpcmd", Scope: "core", Transport: "http"}, nil
	}
	cmdRepo.ListCommandsForDeviceFunc = func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
		return []domain.CommandCatalog{{ID: "cmd-http"}}, nil
	}
	cmdRepo.InsertCommandRequestFunc = func(req domain.CommandRequest) (string, error) {
		return "req-http", nil
	}

	var updatedStatus string
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		updatedStatus = status
		return nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	svc := NewCommandsService(cmdRepo, deviceRepo)
	svc.SetHTTPClient(server.Client(), server.URL)

	req, err := svc.SendCommand("dev-http", "cmd-http", "", map[string]any{"foo": "bar"})
	if err == nil {
		t.Fatalf("expected error on http failure")
	}
	if req == nil || req.Status != "failed" {
		t.Fatalf("expected failed request, got %+v", req)
	}
	if updatedStatus != "failed" {
		t.Fatalf("expected status update to failed, got %s", updatedStatus)
	}
}

func TestCommandsService_RetryPublishHTTP(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}
	deviceRepo := &mocks.MockDeviceRepo{}

	cmdRepo.GetCommandByIDFunc = func(commandID string) (*domain.CommandCatalog, error) {
		return &domain.CommandCatalog{ID: commandID, Transport: "http"}, nil
	}

	var updatedStatus string
	var updatedRetries *int
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		updatedStatus = status
		updatedRetries = retries
		return nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewCommandsService(cmdRepo, deviceRepo)
	svc.SetHTTPClient(server.Client(), server.URL)
	req := domain.CommandRequest{ID: "req-http", DeviceID: "dev-http", ProjectID: "proj-http", CommandID: "cmd-http", Payload: map[string]any{"foo": "bar"}, CorrelationID: "corr-http", Retries: 1}

	if err := svc.RetryPublish(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updatedStatus != "published" {
		t.Fatalf("expected published status, got %s", updatedStatus)
	}
	if updatedRetries == nil || *updatedRetries != 2 {
		t.Fatalf("expected retries incremented to 2, got %v", updatedRetries)
	}
}
