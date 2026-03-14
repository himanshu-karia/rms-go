package engine

import "testing"

func TestRulesEngineEvaluate_UsesCompiledRulesFirst(t *testing.T) {
	e := NewRulesEngine()
	flow := map[string]interface{}{
		"compiled_rules": []interface{}{
			map[string]interface{}{
				"id":      "r1",
				"name":    "High Temp",
				"trigger": "temperature > 40",
				"actions": []interface{}{
					map[string]interface{}{
						"type": "ALERT",
						"payload": map[string]interface{}{
							"message":  "Too hot",
							"severity": "critical",
						},
					},
				},
			},
		},
		"nodes": []interface{}{},
		"edges": []interface{}{},
	}

	actions := e.Evaluate(map[string]interface{}{"temperature": 45}, flow)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "ALERT" {
		t.Fatalf("expected ALERT action, got %s", actions[0].Type)
	}
}

func TestRulesEngineEvaluate_FallsBackToGraph(t *testing.T) {
	e := NewRulesEngine()
	flow := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{
				"id": "n1",
				"data": map[string]interface{}{
					"type":   "TRIGGER",
					"config": map[string]interface{}{"field": "temperature"},
				},
			},
			map[string]interface{}{
				"id": "n2",
				"data": map[string]interface{}{
					"type":   "CONDITION",
					"config": map[string]interface{}{"operator": ">", "value": 30},
				},
			},
			map[string]interface{}{
				"id": "n3",
				"data": map[string]interface{}{
					"type":   "ACTION",
					"config": map[string]interface{}{"actionType": "ALERT", "message": "Over threshold"},
				},
			},
		},
		"edges": []interface{}{
			map[string]interface{}{"source": "n1", "target": "n2"},
			map[string]interface{}{"source": "n2", "target": "n3"},
		},
	}

	actions := e.Evaluate(map[string]interface{}{"temperature": 33}, flow)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action from graph fallback, got %d", len(actions))
	}
	if actions[0].Type != "ALERT" {
		t.Fatalf("expected ALERT action, got %s", actions[0].Type)
	}
}
