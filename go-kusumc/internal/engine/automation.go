package engine

import (
	"context"
	"fmt"
	"ingestion-go/internal/repository" // For Redis Access
	"log"
	"time"
)

// ProcessActions iterates through triggered rules and executes their actions
// It enforces Cooldown logic using Redis.
func ProcessActions(ctx context.Context, results []RuleEvaluationResult, deviceID string, projectID string, mqttPublish func(topic string, payload interface{})) {
	for _, res := range results {
		for _, action := range res.Actions {
			// 1. Check Cooldown
			if action.Cooldown > 0 {
				if isOnCooldown(ctx, res.RuleID, deviceID) {
					log.Printf("⏳ Rule '%s' suppressed (Cooldown active).", res.RuleName)
					continue
				}
				// Set Cooldown
				setCooldown(ctx, res.RuleID, deviceID, time.Duration(action.Cooldown)*time.Minute)
			}

			// 2. Execute Action
			executeAction(res, action, deviceID, projectID, mqttPublish)
		}
	}
}

func isOnCooldown(ctx context.Context, ruleID, deviceID string) bool {
	if repository.Rdb == nil {
		return false
	}
	key := fmt.Sprintf("rule:cooldown:%s:%s", ruleID, deviceID)
	exists, _ := repository.Rdb.Exists(ctx, key).Result()
	return exists > 0
}

func setCooldown(ctx context.Context, ruleID, deviceID string, duration time.Duration) {
	if repository.Rdb == nil {
		return
	}
	key := fmt.Sprintf("rule:cooldown:%s:%s", ruleID, deviceID)
	repository.Rdb.Set(ctx, key, "1", duration)
}

func executeAction(res RuleEvaluationResult, action ActionConfig, deviceID, projectID string, mqttPublish func(topic string, payload interface{})) {
	switch action.Type {
	case "alert":
		// Standard Alert Format
		alertPayload := map[string]interface{}{
			"type":      "ALERT",
			"ruleId":    res.RuleID,
			"ruleName":  res.RuleName,
			"deviceId":  deviceID,
			"projectId": projectID,
			"timestamp": time.Now().UTC(),
			"value":     res.TriggeredValue,
			"message":   fmt.Sprintf("Rule %s triggered! Value: %v", res.RuleName, res.TriggeredValue),
		}

		// Topic: channels/{projectId}/alerts
		topic := fmt.Sprintf("channels/%s/alerts", projectID)
		mqttPublish(topic, alertPayload)

		log.Printf("🚨 ALERT SENT: %s -> %s", res.RuleName, topic)

	case "mqtt_command":
		// Custom Command
		topic := action.Target // e.g., "devices/123/cmd"
		mqttPublish(topic, action.Payload)
		log.Printf("⚡ COMMAND SENT: %s -> %s", action.Payload, topic)

	default:
		log.Printf("⚠️ Unknown Action Type: %s", action.Type)
	}
}
