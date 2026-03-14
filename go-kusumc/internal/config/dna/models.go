package dna

import "ingestion-go/internal/config/payloadschema"

// ProjectPayloadSchema represents the canonical data stored for a single project.
type ProjectPayloadSchema struct {
	ProjectID       string                `json:"projectId"`
	Rows            []payloadschema.Entry `json:"rows"`
	EdgeRules       []EdgeRule            `json:"edgeRules,omitempty"`
	VirtualSensors  []VirtualSensor       `json:"virtualSensors,omitempty"`
	AutomationFlows []AutomationFlowRef   `json:"automationFlows,omitempty"`
	Metadata        map[string]any        `json:"metadata,omitempty"`
}

// EdgeRule describes a simple threshold intended for edge enforcement.
type EdgeRule struct {
	Param     string  `json:"param"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Severity  string  `json:"severity,omitempty"`
	Enabled   bool    `json:"enabled"`
}

// VirtualSensor defines a derived metric.
type VirtualSensor struct {
	Param       string `json:"param"`
	Expression  string `json:"expression"`
	Description string `json:"description,omitempty"`
	Unit        string `json:"unit,omitempty"`
}

// AutomationFlowRef points to a stored automation flow (ReactFlow graph).
type AutomationFlowRef struct {
	FlowID   string `json:"flowId"`
	Revision int    `json:"revision"`
}
