package services

import "testing"

func TestValidateAutomationFlowPayload_ReportsMissingFields(t *testing.T) {
	diagnostics := validateAutomationFlowPayload(map[string]interface{}{})

	if len(diagnostics.Errors) == 0 {
		t.Fatalf("expected validation errors for empty payload")
	}
	if diagnostics.Saved {
		t.Fatalf("expected saved=false before persistence")
	}
}

func TestValidateAutomationFlowPayload_AcceptsValidBundle(t *testing.T) {
	payload := map[string]interface{}{
		"project_id":     "proj-1",
		"schema_version": "1.0.0",
		"nodes": []interface{}{
			map[string]interface{}{"id": "n1", "type": "trigger"},
			map[string]interface{}{"id": "n2", "type": "action", "data": map[string]interface{}{"message": "ok"}},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "n1", "target": "n2"},
		},
		"compiled_rules": []interface{}{
			map[string]interface{}{
				"id":      "rule-1",
				"trigger": "true",
				"actions": []interface{}{map[string]interface{}{"type": "ALERT", "payload": map[string]interface{}{"message": "ok"}}},
			},
		},
	}

	diagnostics := validateAutomationFlowPayload(payload)
	if len(diagnostics.Errors) != 0 {
		t.Fatalf("expected no validation errors, got %v", diagnostics.Errors)
	}
	if diagnostics.CompiledCount != 1 {
		t.Fatalf("expected compiled count 1, got %d", diagnostics.CompiledCount)
	}
}

func TestValidateAutomationFlowPayload_ReportsNodeIssueMetadata(t *testing.T) {
	payload := map[string]interface{}{
		"project_id": "proj-1",
		"nodes": []interface{}{
			map[string]interface{}{"id": "t1", "type": "trigger"},
			map[string]interface{}{
				"id":   "a1",
				"type": "action",
				"data": map[string]interface{}{"actionType": "ALERT", "message": ""},
			},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "t1", "target": "a1"},
		},
		"compiled_rules": []interface{}{},
	}

	diagnostics := validateAutomationFlowPayload(payload)
	if len(diagnostics.Errors) == 0 {
		t.Fatalf("expected validation errors")
	}
	if len(diagnostics.Issues) == 0 {
		t.Fatalf("expected structured issues")
	}

	foundNodeIssue := false
	for _, issue := range diagnostics.Issues {
		if issue.Code == "alert_message_required" {
			foundNodeIssue = true
			if issue.NodeID != "a1" {
				t.Fatalf("expected node id a1, got %s", issue.NodeID)
			}
		}
	}
	if !foundNodeIssue {
		t.Fatalf("expected alert_message_required issue")
	}
}
