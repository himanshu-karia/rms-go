package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"

	"github.com/Knetic/govaluate"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type RulesRepository interface {
	GetRules(projectId, deviceId string) ([]map[string]interface{}, error)
	CreateRule(rule map[string]interface{}) (string, error)
	DeleteRule(id string) error
	CreateWorkOrder(wo map[string]interface{}) error
	CreateAlert(deviceId, projectId, msg, severity string) error
	CreateAlertWithData(deviceId, projectId, msg, severity string, data interface{}) error
}

type RulesService struct {
	repo       RulesRepository
	redisStore *secondary.RedisStore
	configSync *ConfigSyncService
	mqttClient mqtt.Client
}

func NewRulesService(repo RulesRepository, redisStore *secondary.RedisStore, sync *ConfigSyncService) *RulesService {
	return &RulesService{repo: repo, redisStore: redisStore, configSync: sync}
}

// Start initializes the MQTT client if needed, or we can reuse one.
// For V1, passing a client in Evaluate or initializing here is fine.
// Let's attach the shared client via a setter or just Init logic.
func (s *RulesService) SetMqttClient(client mqtt.Client) {
	s.mqttClient = client
}

// EmitDeviceAlert persists and publishes a device-originated alert/event.
// This is used for device offline-rule alerts and runtime errors sent by firmware.
func (s *RulesService) EmitDeviceAlert(projectId, deviceId, message, severity string, data map[string]interface{}) {
	projectId = strings.TrimSpace(projectId)
	deviceId = strings.TrimSpace(deviceId)
	message = strings.TrimSpace(message)
	severity = strings.ToLower(strings.TrimSpace(severity))
	if severity == "" {
		severity = "warning"
	}

	alert := map[string]interface{}{
		"device_id":  deviceId,
		"project_id": projectId,
		"message":    message,
		"severity":   severity,
		"status":     "active",
		"created_at": time.Now(),
		"data":       data,
	}

	// Persist when we have enough identifiers.
	if projectId != "" && deviceId != "" {
		if err := s.repo.CreateAlertWithData(deviceId, projectId, message, severity, data); err != nil {
			log.Printf("[Rules] Failed to persist device alert: %v", err)
		}
	}

	// Publish to the project alerts stream.
	if s.mqttClient != nil && projectId != "" {
		topic := fmt.Sprintf("channels/%s/alerts", projectId)
		bytes, _ := json.Marshal(alert)
		s.mqttClient.Publish(topic, 1, false, bytes)
	}
}

// Evaluate checks incoming telemetry against rules
func (s *RulesService) Evaluate(packet map[string]interface{}) {
	projectId, _ := packet["project_id"].(string)
	deviceId, _ := packet["device_id"].(string)
	payload, ok := packet["payload"].(map[string]interface{})

	if !ok || projectId == "" {
		return
	}

	rules, err := s.loadRules(projectId)
	if err != nil {
		log.Printf("[Rules] Fetch Error: %v", err)
		return
	}

	for _, rule := range rules {
		// Filter by deviceId if rule is specific
		rDevId, _ := rule["device_id"].(string)
		if rDevId != "" && rDevId != deviceId {
			continue // Rule is for another device
		}

		if s.checkCondition(rule, payload) {
			s.triggerAlert(rule, packet)
		}
	}
}

func (s *RulesService) checkCondition(rule map[string]interface{}, data map[string]interface{}) bool {
	trigger := strings.TrimSpace(resolveRuleTriggerExpression(rule["trigger"]))
	if trigger == "" {
		return false
	}

	expression, err := govaluate.NewEvaluableExpression(trigger)
	if err != nil {
		log.Printf("[Rules] Formula Error in '%s': %v", rule["name"], err)
		return false
	}

	result, err := expression.Evaluate(data)
	if err != nil {
		// This usually happens if a parameter is missing. In strict rules, we might want to fail.
		// For now, log and return false.
		// log.Printf("[Rules] Eval Error: %v", err)
		return false
	}

	if boolResult, ok := result.(bool); ok {
		return boolResult
	}
	return false
}

func resolveRuleTriggerExpression(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return ""
		}
		if strings.HasPrefix(trimmed, "{") {
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
				return resolveRuleTriggerExpression(payload)
			}
		}
		return trimmed
	case map[string]interface{}:
		if formula, ok := v["formula"].(string); ok {
			return strings.TrimSpace(formula)
		}
		field, _ := v["field"].(string)
		op, _ := v["operator"].(string)
		value, hasValue := v["value"]
		field = strings.TrimSpace(field)
		op = strings.TrimSpace(op)
		if field == "" || op == "" || !hasValue {
			return ""
		}
		switch vv := value.(type) {
		case string:
			return fmt.Sprintf("%s %s %q", field, op, vv)
		default:
			return fmt.Sprintf("%s %s %v", field, op, vv)
		}
	default:
		return ""
	}
}

func resolveRuleActions(raw interface{}) []map[string]interface{} {
	switch v := raw.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]interface{}:
		return v
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		var parsed []interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil
		}
		out := make([]map[string]interface{}, 0, len(parsed))
		for _, item := range parsed {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func (s *RulesService) triggerAlert(rule, packet map[string]interface{}) {
	projectId, _ := packet["project_id"].(string)
	deviceId, _ := packet["device_id"].(string)
	// deviceName := packet["device_name"] ??
	severity := "warning"
	if sev, ok := rule["severity"].(string); ok && strings.TrimSpace(sev) != "" {
		severity = strings.ToLower(strings.TrimSpace(sev))
	}

	actions := resolveRuleActions(rule["actions"])

	alert := map[string]interface{}{
		"device_id":  deviceId,
		"project_id": projectId,
		"message":    fmt.Sprintf("Rule %s Triggered", rule["name"]),
		"severity":   severity,
		"status":     "active",
		"created_at": time.Now(),
		"rule_id":    rule["id"],
		"data": map[string]interface{}{
			"source": "server_rule",
			"rule": map[string]interface{}{
				"id":   rule["id"],
				"name": rule["name"],
			},
			"payload": packet["payload"],
		},
	}

	// 1. Save to DB (We reused 'alerts' table, but implied Repos)
	// repo.CreateAlert(alert) -> We need that method in repo?
	// We have GetAlerts. Let's assume CreateAlert exists or we query exec.
	// Check PostgresRepo... It had AckAlert. It didn't have CreateAlert explicitly exposed?
	// Let's double check repo. If missing, we add it.
	// Assuming it's missing, let's just log and MQTT for now.

	log.Printf("⚡ ALERT: %v", alert)

	// 1. Persist ALERT actions into DB (store trigger payload)
	if strings.TrimSpace(projectId) != "" && strings.TrimSpace(deviceId) != "" {
		for _, action := range actions {
			t, _ := action["type"].(string)
			if strings.EqualFold(strings.TrimSpace(t), "ALERT") {
				msg := ""
				if m, ok := action["message"].(string); ok {
					msg = strings.TrimSpace(m)
				}
				if msg == "" {
					if m, ok := action["msg"].(string); ok {
						msg = strings.TrimSpace(m)
					}
				}
				if msg == "" {
					msg = fmt.Sprintf("Rule %s Triggered", rule["name"])
				}
				// Store the raw trigger payload + rule metadata.
				payloadData := map[string]interface{}{
					"source": "server_rule",
					"rule": map[string]interface{}{
						"id":   rule["id"],
						"name": rule["name"],
					},
					"payload": packet["payload"],
				}
				if err := s.repo.CreateAlertWithData(deviceId, projectId, msg, severity, payloadData); err != nil {
					log.Printf("[Rules] Failed to persist alert: %v", err)
				}
			}
		}
	}

	// 2. MQTT Push
	if s.mqttClient != nil {
		topic := fmt.Sprintf("channels/%s/alerts", projectId)
		bytes, _ := json.Marshal(alert)
		s.mqttClient.Publish(topic, 1, false, bytes)
	}

	// 3. Auto-Automation (Work Order for Critical Alerts)
	if severity, ok := rule["severity"].(string); ok && severity == "critical" {
		// Auto-Create Ticket
		// We use PostgresRepo's CreateWorkOrder directly.
		// Need msgid (uuid) for ticket_id
		ticketId := fmt.Sprintf("TICKET-%d", time.Now().UnixNano())
		wo := map[string]interface{}{
			"ticket_id": ticketId,
			"title":     fmt.Sprintf("CRITICAL: %s", rule["name"]),
			"device_id": deviceId,
			"priority":  "high",
		}

		err := s.repo.CreateWorkOrder(wo)
		if err != nil {
			log.Printf("[Rules] Failed to auto-create work order: %v", err)
		} else {
			log.Printf("[Rules] Auto-Created Work Order %s", ticketId)
		}
	}
}

func (s *RulesService) loadRules(projectId string) ([]map[string]interface{}, error) {
	if projectId == "" {
		return nil, nil
	}

	if s.redisStore != nil {
		if bundle, ok := s.redisStore.GetConfigBundle(projectId); ok {
			if rawRules, ok := bundle["rules"].([]interface{}); ok {
				parsed := make([]map[string]interface{}, 0, len(rawRules))
				for _, r := range rawRules {
					if rm, ok := r.(map[string]interface{}); ok {
						parsed = append(parsed, rm)
					}
				}
				if len(parsed) == 0 {
					return nil, nil
				}
				return parsed, nil
			}
		}
	}

	if s.redisStore != nil {
		key := fmt.Sprintf("config:rules:%s", projectId)
		if raw, ok, err := s.redisStore.GetRaw(key); err == nil && ok {
			if raw == "" || raw == "[]" || raw == "null" {
				return nil, nil
			}

			var cached []map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &cached); err == nil {
				if len(cached) == 0 {
					return nil, nil
				}
				return cached, nil
			}

			log.Printf("[Rules] Failed to decode cached rules for %s: %v", projectId, err)
		} else if err != nil {
			log.Printf("[Rules] Redis lookup failed for %s: %v", projectId, err)
		}
	}

	rules, err := s.repo.GetRules(projectId, "")
	if err != nil {
		return nil, err
	}

	if s.redisStore != nil {
		key := fmt.Sprintf("config:rules:%s", projectId)
		if len(rules) == 0 {
			if err := s.redisStore.SetRaw(key, "[]", 0); err != nil {
				log.Printf("[Rules] Failed to cache empty rules for %s: %v", projectId, err)
			}
			return rules, nil
		}

		if data, err := json.Marshal(rules); err == nil {
			if err := s.redisStore.SetRaw(key, string(data), 0); err != nil {
				log.Printf("[Rules] Failed to cache rules for %s: %v", projectId, err)
			}
		} else {
			log.Printf("[Rules] Failed to marshal rules for %s: %v", projectId, err)
		}
	}

	return rules, nil
}

func (s *RulesService) GetRules(projectId, deviceId string) ([]map[string]interface{}, error) {
	return s.repo.GetRules(projectId, deviceId)
}

func (s *RulesService) CreateRule(rule map[string]interface{}) (string, error) {
	id, err := s.repo.CreateRule(rule)
	if err != nil {
		return "", err
	}

	var projectID string
	if v, ok := rule["project_id"].(string); ok {
		projectID = strings.TrimSpace(v)
	} else if v, ok := rule["projectId"].(string); ok {
		projectID = strings.TrimSpace(v)
	}

	if projectID != "" {
		go s.configSync.SyncProject(projectID)
	} else {
		go s.configSync.SyncAll()
	}

	return id, nil
}

func (s *RulesService) DeleteRule(id string) error {
	err := s.repo.DeleteRule(id)
	if err != nil {
		return err
	}

	go s.configSync.SyncAll()
	return nil
}
