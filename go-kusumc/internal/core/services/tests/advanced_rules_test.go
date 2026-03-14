package services_test

import (
	"ingestion-go/internal/core/services"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/mock"
)

// MockRulesRepo implements services.RulesRepository
type MockRulesRepo struct {
	mock.Mock
}

func (m *MockRulesRepo) GetRules(projectId, deviceId string) ([]map[string]interface{}, error) {
	args := m.Called(projectId, deviceId)
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockRulesRepo) CreateRule(rule map[string]interface{}) (string, error) {
	args := m.Called(rule)
	return args.String(0), args.Error(1)
}

func (m *MockRulesRepo) DeleteRule(id string) error {
	return m.Called(id).Error(0)
}

func (m *MockRulesRepo) CreateWorkOrder(wo map[string]interface{}) error {
	return m.Called(wo).Error(0)
}

func (m *MockRulesRepo) CreateAlert(deviceId, projectId, msg, severity string) error {
	return m.Called(deviceId, projectId, msg, severity).Error(0)
}

func (m *MockRulesRepo) CreateAlertWithData(deviceId, projectId, msg, severity string, data interface{}) error {
	return m.Called(deviceId, projectId, msg, severity, data).Error(0)
}

// Mock MQTT
type MockMqttClient struct {
	mock.Mock
}

func (m *MockMqttClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	// Simple stub
	m.Called(topic, payload)
	return &dummyToken{}
}
func (m *MockMqttClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return nil
}
func (m *MockMqttClient) Connect() mqtt.Token                                 { return nil }
func (m *MockMqttClient) Disconnect(quiesce uint)                             {}
func (m *MockMqttClient) IsConnected() bool                                   { return true }
func (m *MockMqttClient) IsConnectionOpen() bool                              { return true }
func (m *MockMqttClient) AddRoute(topic string, callback mqtt.MessageHandler) {}
func (m *MockMqttClient) Unsubscribe(topics ...string) mqtt.Token             { return nil }
func (m *MockMqttClient) OptionsReader() mqtt.ClientOptionsReader             { return mqtt.ClientOptionsReader{} }

type dummyToken struct {
	mqtt.Token
}

func (t *dummyToken) Wait() bool                       { return true }
func (t *dummyToken) WaitTimeout(d time.Duration) bool { return true }
func (t *dummyToken) Error() error                     { return nil }

func TestAdvancedRules_Evaluate(t *testing.T) {
	// 1. Setup
	mockRepo := new(MockRulesRepo)
	svc := services.NewRulesService(mockRepo, nil, nil) // configSync nil is safe for Evaluate

	// 2. Define Advanced Rule
	advancedRule := map[string]interface{}{
		"id":         "rule_001",
		"name":       "Battery & Temp Check",
		"project_id": "proj_123",
		"trigger":    "temp > 40 && battery < 20", // The syntax govaluate parses
		"severity":   "critical",
		"actions":    []interface{}{map[string]interface{}{"type": "ALERT", "message": "Battery & Temp Check triggered"}},
	}

	// Mock Repo Response
	mockRepo.On("GetRules", "proj_123", "").Return([]map[string]interface{}{advancedRule}, nil)
	mockRepo.On("CreateWorkOrder", mock.Anything).Return(nil) // Should trigger CRITICAL
	mockRepo.On("CreateAlertWithData", "dev_001", "proj_123", mock.Anything, "critical", mock.Anything).Return(nil)

	// 3. Define Packet
	packet := map[string]interface{}{
		"project_id": "proj_123",
		"device_id":  "dev_001",
		"payload": map[string]interface{}{
			"temp":    45.0, // > 40
			"battery": 15.0, // < 20
		},
	}

	// 4. Execute
	// We need to verify side effects (CreateWorkOrder) since Evaluate doesn't return value
	svc.Evaluate(packet)

	// 5. Assert
	mockRepo.AssertCalled(t, "GetRules", "proj_123", "")
	mockRepo.AssertCalled(t, "CreateAlertWithData", "dev_001", "proj_123", mock.Anything, "critical", mock.Anything)
	mockRepo.AssertCalled(t, "CreateWorkOrder", mock.MatchedBy(func(wo map[string]interface{}) bool {
		return wo["priority"] == "high" && wo["device_id"] == "dev_001"
	}))
}

func TestAdvancedRules_Evaluate_False(t *testing.T) {
	// 1. Setup
	mockRepo := new(MockRulesRepo)
	svc := services.NewRulesService(mockRepo, nil, nil)

	// 2. Define Advanced Rule
	advancedRule := map[string]interface{}{
		"id":       "rule_002",
		"name":     "Complex Logic",
		"trigger":  "rpm > 1000 || (temp > 80 && load > 0.9)",
		"severity": "warning",
		"actions":  []interface{}{map[string]interface{}{"type": "ALERT", "message": "Complex triggered"}},
	}

	mockRepo.On("GetRules", "proj_ABC", "").Return([]map[string]interface{}{advancedRule}, nil)

	// 3. Define Packet (Should NOT trigger)
	packet := map[string]interface{}{
		"project_id": "proj_ABC",
		"device_id":  "dev_ABC",
		"payload": map[string]interface{}{
			"rpm":  500.0,
			"temp": 70.0,
			"load": 0.5,
		},
	}

	// 4. Execute
	svc.Evaluate(packet)

	// 5. Assert
	mockRepo.AssertCalled(t, "GetRules", "proj_ABC", "")
	mockRepo.AssertNotCalled(t, "CreateWorkOrder", mock.Anything)
	mockRepo.AssertNotCalled(t, "CreateAlertWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
