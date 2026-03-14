package engine

import (
	"fmt"
	"log"
	"strings"

	"github.com/Knetic/govaluate"
)

// Simplified Rule Structure matching Studio JSON
type Rule struct {
	Name    string
	Trigger string // "temp > 50"
	Actions []Action
}

type Action struct {
	Type    string                 // "ALERT"
	Payload map[string]interface{} // { "message": "High Temp" }
}

type RulesEngine struct {
	// Cache or other state if needed
}

func NewRulesEngine() *RulesEngine {
	return &RulesEngine{}
}

// Evaluate checks telemetry against loaded rules
func (e *RulesEngine) Evaluate(telemetry map[string]interface{}, flow map[string]interface{}) []Action {
	if actions, ok := e.evaluateCompiledRules(telemetry, flow["compiled_rules"]); ok {
		return actions
	}

	return e.evaluateGraphFallback(telemetry, flow)
}

func (e *RulesEngine) evaluateCompiledRules(telemetry map[string]interface{}, compiledRaw interface{}) ([]Action, bool) {
	rulesSlice, ok := compiledRaw.([]interface{})
	if !ok {
		return nil, false
	}

	triggeredActions := make([]Action, 0)
	for _, raw := range rulesSlice {
		ruleMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		triggerExpr, _ := ruleMap["trigger"].(string)
		triggerExpr = strings.TrimSpace(triggerExpr)
		if triggerExpr == "" {
			triggerExpr = "true"
		}

		satisfied, err := e.evaluateExpression(triggerExpr, telemetry)
		if err != nil {
			log.Printf("Rule Error [%s]: %v", triggerExpr, err)
			continue
		}
		if !satisfied {
			continue
		}

		actionsRaw, ok := ruleMap["actions"].([]interface{})
		if !ok {
			continue
		}

		for _, actionRaw := range actionsRaw {
			actionMap, ok := actionRaw.(map[string]interface{})
			if !ok {
				continue
			}

			actionType, _ := actionMap["type"].(string)
			payload, _ := actionMap["payload"].(map[string]interface{})
			if payload == nil {
				payload = map[string]interface{}{}
			}

			triggeredActions = append(triggeredActions, Action{Type: actionType, Payload: payload})
		}
	}

	return triggeredActions, true
}

func (e *RulesEngine) evaluateGraphFallback(telemetry map[string]interface{}, flow map[string]interface{}) []Action {
	var triggeredActions []Action

	nodesRaw, ok := flow["nodes"].([]interface{})
	if !ok {
		return nil
	}
	edgesRaw, ok := flow["edges"].([]interface{})
	if !ok {
		return nil
	}

	// Build Lookup
	nodeMap := make(map[string]map[string]interface{})
	for _, n := range nodesRaw {
		nm, _ := n.(map[string]interface{})
		id, _ := nm["id"].(string)
		nodeMap[id] = nm
	}

	// Build Edges Map: Source -> Target
	edgeMap := make(map[string]string)
	for _, e := range edgesRaw {
		em, _ := e.(map[string]interface{})
		src, _ := em["source"].(string)
		tgt, _ := em["target"].(string)
		edgeMap[src] = tgt
	}

	// Find Triggers
	for _, n := range nodesRaw {
		nm, _ := n.(map[string]interface{})
		data, _ := nm["data"].(map[string]interface{})
		typeStr, _ := data["type"].(string)
		if strings.TrimSpace(typeStr) == "" {
			typeStr, _ = nm["type"].(string)
		}
		typeStr = strings.ToUpper(strings.TrimSpace(typeStr))

		if typeStr == "TRIGGER" {
			// Traverse: Trigger -> Condition -> Action
			// This mimics `LogicCompiler.ts`

			// 1. Trigger
			// data.config.field "temp"
			config, _ := data["config"].(map[string]interface{})
			field, _ := config["field"].(string)
			if strings.TrimSpace(field) == "" {
				field, _ = data["field"].(string)
			}

			// Get Next (Condition)
			nextId, exists := edgeMap[nm["id"].(string)]
			if !exists {
				continue
			}
			condNode := nodeMap[nextId]
			condData, _ := condNode["data"].(map[string]interface{})
			condType, _ := condData["type"].(string)
			if strings.TrimSpace(condType) == "" {
				condType, _ = condNode["type"].(string)
			}
			if strings.ToUpper(strings.TrimSpace(condType)) != "CONDITION" {
				continue
			}
			condConfig, _ := condData["config"].(map[string]interface{})
			operator, _ := condConfig["operator"].(string)
			value := condConfig["value"] // might be string or number

			// Get Next (Action)
			nextId2, exists2 := edgeMap[condNode["id"].(string)]
			if !exists2 {
				continue
			}
			actionNode := nodeMap[nextId2]
			actionData, _ := actionNode["data"].(map[string]interface{})
			actionTypeRaw, _ := actionData["type"].(string)
			if strings.TrimSpace(actionTypeRaw) == "" {
				actionTypeRaw, _ = actionNode["type"].(string)
			}
			if strings.ToUpper(strings.TrimSpace(actionTypeRaw)) != "ACTION" {
				continue
			}
			actionConfig, _ := actionData["config"].(map[string]interface{})
			if actionConfig == nil {
				actionConfig = map[string]interface{}{}
			}

			// Formulate Expression
			expressionStr := fmt.Sprintf("%v %s %v", field, operator, value)
			// e.g., "temp > 50"

			// Evaluate
			satisfied, err := e.evaluateExpression(expressionStr, telemetry)
			if err != nil {
				log.Printf("Rule Error [%s]: %v", expressionStr, err)
				continue
			}

			if satisfied {
				actionType, _ := actionConfig["actionType"].(string)
				if strings.TrimSpace(actionType) == "" {
					actionType = "ALERT"
				}
				triggeredActions = append(triggeredActions, Action{
					Type:    actionType,
					Payload: actionConfig,
				})
			}
		}
	}

	return triggeredActions
}

func (e *RulesEngine) evaluateExpression(expr string, params map[string]interface{}) (bool, error) {
	expression, err := govaluate.NewEvaluableExpression(expr)
	if err != nil {
		return false, err
	}

	result, err := expression.Evaluate(params)
	if err != nil {
		return false, err
	}

	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression did not return boolean")
	}

	return boolResult, nil
}
