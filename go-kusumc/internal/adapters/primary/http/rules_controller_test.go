package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type mockRulesService struct {
	getRulesCalls []struct {
		projectID string
		deviceID  string
	}
	createRuleCalls []map[string]interface{}
	deleteCalls     []string

	getRulesResult []map[string]interface{}
	getRulesErr    error
	createRuleID   string
	createRuleErr  error
	deleteErr      error
}

func (m *mockRulesService) GetRules(projectId, deviceId string) ([]map[string]interface{}, error) {
	m.getRulesCalls = append(m.getRulesCalls, struct {
		projectID string
		deviceID  string
	}{projectID: projectId, deviceID: deviceId})
	return m.getRulesResult, m.getRulesErr
}

func (m *mockRulesService) CreateRule(rule map[string]interface{}) (string, error) {
	m.createRuleCalls = append(m.createRuleCalls, rule)
	return m.createRuleID, m.createRuleErr
}

func (m *mockRulesService) DeleteRule(id string) error {
	m.deleteCalls = append(m.deleteCalls, id)
	return m.deleteErr
}

func TestRulesController_GetRules_SnakeCasePrecedenceAndResponseNormalized(t *testing.T) {
	mockSvc := &mockRulesService{
		getRulesResult: []map[string]interface{}{
			{"createdAt": "2026-01-01T00:00:00Z", "deviceId": "d1", "projectId": "p1"},
		},
	}
	controller := NewRulesController(mockSvc)

	app := fiber.New()
	app.Get("/rules", controller.GetRules)

	req := httptest.NewRequest("GET", "/rules?projectId=p-camel&project_id=p-snake&deviceId=d-camel&device_id=d-snake", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(mockSvc.getRulesCalls) != 1 {
		t.Fatalf("expected 1 GetRules call, got %d", len(mockSvc.getRulesCalls))
	}
	call := mockSvc.getRulesCalls[0]
	if call.projectID != "p-snake" {
		t.Fatalf("expected project_id snake precedence, got %q", call.projectID)
	}
	if call.deviceID != "d-snake" {
		t.Fatalf("expected device_id snake precedence, got %q", call.deviceID)
	}

	var payload []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(payload))
	}
	if _, ok := payload[0]["created_at"]; !ok {
		t.Fatalf("expected created_at in response, got %v", payload[0])
	}
	if _, ok := payload[0]["device_id"]; !ok {
		t.Fatalf("expected device_id in response, got %v", payload[0])
	}
	if _, ok := payload[0]["project_id"]; !ok {
		t.Fatalf("expected project_id in response, got %v", payload[0])
	}
}

func TestRulesController_CreateRule_NormalizesCamelCaseBody(t *testing.T) {
	mockSvc := &mockRulesService{createRuleID: "r1"}
	controller := NewRulesController(mockSvc)

	app := fiber.New()
	app.Post("/rules", controller.CreateRule)

	body := `{
		"projectId": "p1",
		"name": "rule-1",
		"trigger": {"field": "x", "operator": ">", "value": 1},
		"actions": [{"type": "alert", "targetId": "t1"}]
	}`
	req := httptest.NewRequest("POST", "/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(mockSvc.createRuleCalls) != 1 {
		t.Fatalf("expected 1 CreateRule call, got %d", len(mockSvc.createRuleCalls))
	}
	sent := mockSvc.createRuleCalls[0]
	if sent["project_id"] != "p1" {
		t.Fatalf("expected project_id normalized, got %v", sent["project_id"])
	}
	trigger, _ := sent["trigger"].(map[string]interface{})
	if trigger == nil {
		t.Fatalf("expected trigger map, got %T", sent["trigger"])
	}
	actions, _ := sent["actions"].([]interface{})
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", sent["actions"])
	}
	a0, _ := actions[0].(map[string]interface{})
	if a0 == nil {
		t.Fatalf("expected action map, got %T", actions[0])
	}
	if _, ok := a0["target_id"]; !ok {
		t.Fatalf("expected target_id normalized in action, got %v", a0)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if out["id"] != "r1" {
		t.Fatalf("expected id r1, got %v", out["id"])
	}
}
