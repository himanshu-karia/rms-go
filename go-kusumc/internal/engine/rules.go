package engine

import (
	"fmt"
	"log"
)

type RuleEvaluationResult struct {
	RuleID   string
	RuleName string
	Actions  []ActionConfig
	TriggeredValue interface{}
}

// EvaluateRules checks the transformed data against all active rules
func EvaluateRules(data map[string]interface{}, rules []RuleConfig) []RuleEvaluationResult {
	var results []RuleEvaluationResult

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Get Value from Data
		fieldVal, exists := data[rule.Trigger.Field]
		if !exists {
			continue
		}

		// Evaluate Condition
		if checkCondition(fieldVal, rule.Trigger.Operator, rule.Trigger.Value) {
			results = append(results, RuleEvaluationResult{
				RuleID:         rule.ID,
				RuleName:       rule.Name,
				Actions:        rule.Actions,
				TriggeredValue: fieldVal,
			})
			log.Printf("🔥 Rule Triggered: %s (Value: %v %s %v)", rule.Name, fieldVal, rule.Trigger.Operator, rule.Trigger.Value)
		}
	}

	return results
}

// checkCondition compares actual value vs threshold
func checkCondition(actual interface{}, operator string, threshold interface{}) bool {
	// Attempt to convert both to float64 for numeric comparison
	f1, isNum1 := toFloat(actual)
	f2, isNum2 := toFloat(threshold)

	if isNum1 && isNum2 {
		switch operator {
		case ">":
			return f1 > f2
		case "<":
			return f1 < f2
		case ">=":
			return f1 >= f2
		case "<=":
			return f1 <= f2
		case "=":
			return f1 == f2
		case "!=":
			return f1 != f2
		}
	}

	// Fallback to String comparison
	s1 := fmt.Sprintf("%v", actual)
	s2 := fmt.Sprintf("%v", threshold)

	switch operator {
	case "=":
		return s1 == s2
	case "!=":
		return s1 != s2
	}

	return false
}
