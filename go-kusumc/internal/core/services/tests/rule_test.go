package services_test

import (
	"ingestion-go/internal/core/services"
	"ingestion-go/tests/mocks"
	"testing"
)

// MockRuleRepo - Local mock since interface is in Services
type MockRuleRepo struct {
	// Add function fields if RuleRepo methods are added
}

// Ensure Mock implements Interface
func (m *MockRuleRepo) CreateRule(projectId, name string, trigger, actions interface{}) error {
	return nil
}

func (m *MockRuleRepo) GetRules(projectId string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func TestCreateRule(t *testing.T) {
	mockState := &mocks.MockStateStore{
		SetProjectConfigFunc: func(id string, cfg interface{}) error { return nil },
	}
	mockRepo := &MockRuleRepo{}

	svc := services.NewRuleService(mockRepo, mockState)

	err := svc.CreateRule("p1", "d1", "rule1", nil, nil)
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
}
