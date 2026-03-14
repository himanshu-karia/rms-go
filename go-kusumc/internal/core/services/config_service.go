package services

import (
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"os"
	"strings"
)

// ConfigService handles Advanced Configuration Objects
// distinct from standard ERP entities.
type ConfigService struct {
	repo                       *secondary.PostgresRepo
	EnableChirpstack           bool
	EnableHttpsTelemetryMirror bool
	EnableHttpsTelemetryIngest bool
}

type AutomationSaveDiagnostics struct {
	Saved         bool                        `json:"saved"`
	ProjectID     string                      `json:"project_id"`
	SchemaVersion string                      `json:"schema_version"`
	CompiledCount int                         `json:"compiled_count"`
	Errors        []string                    `json:"errors"`
	Warnings      []string                    `json:"warnings"`
	Issues        []AutomationDiagnosticIssue `json:"issues"`
}

type AutomationDiagnosticIssue struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
	Path    string `json:"path,omitempty"`
}

func NewConfigService(repo *secondary.PostgresRepo) *ConfigService {
	enable := os.Getenv("ENABLE_CHIRPSTACK") == "true"
	enableMirror := os.Getenv("ENABLE_HTTPS_TELEMETRY_MIRROR") == "true"
	enableIngest := os.Getenv("ENABLE_HTTPS_TELEMETRY_INGEST") == "true"
	return &ConfigService{
		repo:                       repo,
		EnableChirpstack:           enable,
		EnableHttpsTelemetryMirror: enableMirror,
		EnableHttpsTelemetryIngest: enableIngest,
	}
}

// --- Automation Flows ---
func (s *ConfigService) SaveAutomationFlow(flow map[string]interface{}) (AutomationSaveDiagnostics, error) {
	diagnostics := validateAutomationFlowPayload(flow)
	if len(diagnostics.Errors) > 0 {
		return diagnostics, nil
	}
	if err := s.repo.CreateAutomationFlow(flow); err != nil {
		return diagnostics, err
	}
	diagnostics.Saved = true
	return diagnostics, nil
}

func (s *ConfigService) GetAutomationFlow(projectId string) (map[string]interface{}, error) {
	return s.repo.GetAutomationFlow(projectId)
}

func validateAutomationFlowPayload(flow map[string]interface{}) AutomationSaveDiagnostics {
	projectID, _ := flow["project_id"].(string)
	schemaVersion, _ := flow["schema_version"].(string)
	if strings.TrimSpace(schemaVersion) == "" {
		schemaVersion = "1.0.0"
	}

	diagnostics := AutomationSaveDiagnostics{
		Saved:         false,
		ProjectID:     projectID,
		SchemaVersion: schemaVersion,
		Errors:        []string{},
		Warnings:      []string{},
		Issues:        []AutomationDiagnosticIssue{},
	}

	if strings.TrimSpace(projectID) == "" {
		diagnostics.addIssue("error", "project_id_required", "project_id is required", "", "project_id")
	}

	nodesRaw, nodesOk := flow["nodes"].([]interface{})
	if !nodesOk {
		diagnostics.addIssue("error", "nodes_invalid_type", "nodes must be an array", "", "nodes")
	} else if len(nodesRaw) == 0 {
		diagnostics.addIssue("warning", "nodes_empty", "nodes is empty", "", "nodes")
	}

	edgesRaw, edgesOk := flow["edges"].([]interface{})
	if !edgesOk {
		diagnostics.addIssue("error", "edges_invalid_type", "edges must be an array", "", "edges")
	}

	if compiledRaw, ok := flow["compiled_rules"]; ok {
		if compiledRules, ok := compiledRaw.([]interface{}); ok {
			diagnostics.CompiledCount = len(compiledRules)
			if len(compiledRules) == 0 {
				diagnostics.addIssue("warning", "compiled_rules_empty", "compiled_rules is empty; runtime will use graph fallback", "", "compiled_rules")
			}
		} else {
			diagnostics.addIssue("error", "compiled_rules_invalid_type", "compiled_rules must be an array when provided", "", "compiled_rules")
		}
	} else {
		diagnostics.addIssue("warning", "compiled_rules_missing", "compiled_rules missing; runtime will use graph fallback", "", "compiled_rules")
	}

	if nodesOk {
		triggerCount := 0
		actionCount := 0
		for idx, nodeRaw := range nodesRaw {
			nodeMap, ok := nodeRaw.(map[string]interface{})
			if !ok {
				diagnostics.addIssue("error", "node_invalid_object", fmt.Sprintf("node[%d] must be an object", idx), "", fmt.Sprintf("nodes[%d]", idx))
				continue
			}
			nodeID, _ := nodeMap["id"].(string)
			nodeType := ""
			if t, ok := nodeMap["type"].(string); ok {
				nodeType = strings.ToLower(strings.TrimSpace(t))
			}
			dataMap, _ := nodeMap["data"].(map[string]interface{})
			if nodeType == "" {
				if t, ok := dataMap["type"].(string); ok {
					nodeType = strings.ToLower(strings.TrimSpace(t))
				}
			}

			if nodeType == "action" {
				actionType := strings.ToUpper(strings.TrimSpace(readNodeString(nodeMap, "actionType")))
				if actionType == "" {
					actionType = "ALERT"
				}

				if actionType == "ALERT" {
					message := strings.TrimSpace(readNodeString(nodeMap, "message"))
					if message == "" {
						diagnostics.addIssue("error", "alert_message_required", "Alert action missing message", nodeID, fmt.Sprintf("nodes[%d].data.message", idx))
					}
				}

				if actionType == "MQTT_COMMAND" {
					topic := strings.TrimSpace(readNodeString(nodeMap, "topic"))
					payload := strings.TrimSpace(readNodeString(nodeMap, "payload"))
					if topic == "" && payload == "" {
						diagnostics.addIssue("error", "mqtt_action_target_required", "MQTT action missing topic or payload", nodeID, fmt.Sprintf("nodes[%d].data", idx))
					}
				}
			}
			switch nodeType {
			case "trigger":
				triggerCount++
			case "action":
				actionCount++
			}
		}
		if triggerCount == 0 {
			diagnostics.addIssue("error", "trigger_required", "at least one trigger node is required", "", "nodes")
		}
		if actionCount == 0 {
			diagnostics.addIssue("error", "action_required", "at least one action node is required", "", "nodes")
		}
	}

	if edgesOk && nodesOk && len(edgesRaw) == 0 && len(nodesRaw) > 1 {
		diagnostics.addIssue("warning", "edges_empty", "edges is empty while multiple nodes exist", "", "edges")
	}

	return diagnostics
}

func readNodeString(nodeMap map[string]interface{}, key string) string {
	if value, ok := nodeMap[key].(string); ok {
		return value
	}
	if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
		if value, ok := dataMap[key].(string); ok {
			return value
		}
		if configMap, ok := dataMap["config"].(map[string]interface{}); ok {
			if value, ok := configMap[key].(string); ok {
				return value
			}
		}
	}
	return ""
}

func (d *AutomationSaveDiagnostics) addIssue(level, code, message, nodeID, path string) {
	issue := AutomationDiagnosticIssue{
		Level:   level,
		Code:    code,
		Message: message,
		NodeID:  nodeID,
		Path:    path,
	}
	d.Issues = append(d.Issues, issue)
	if level == "warning" {
		d.Warnings = append(d.Warnings, message)
		return
	}
	d.Errors = append(d.Errors, message)
}

// --- Device Profiles ---
func (s *ConfigService) CreateDeviceProfile(profile map[string]interface{}) error {
	// Add JSON validation here
	return s.repo.CreateDeviceProfile(profile)
}

func (s *ConfigService) GetDeviceProfiles() ([]map[string]interface{}, error) {
	return s.repo.GetDeviceProfiles()
}
