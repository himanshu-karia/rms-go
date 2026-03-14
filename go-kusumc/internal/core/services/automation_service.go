package services

import (
	"encoding/json"
	"ingestion-go/internal/adapters/secondary"
	"log"
)

// Simple structures to mimic the Mongo 'AutomationFlow'
type Flow struct {
	ID    string
	Nodes []Node
	Edges []Edge
}
type Node struct {
	ID   string
	Type string // trigger, condition, action
	Data map[string]interface{}
}
type Edge struct {
	Source string
	Target string
}

type AutomationService struct {
	repo *secondary.PostgresRepo // For Alerts
	// flowCache map[string]Flow // In reality, fetch from DB
}

func NewAutomationService(repo *secondary.PostgresRepo) *AutomationService {
	return &AutomationService{repo: repo}
}

func (s *AutomationService) ProcessPacket(packet map[string]interface{}) {
	pid, ok := packet["project_id"].(string)
	if !ok {
		return
	}

	// 1. Fetch Flow from DB
	flowMap, err := s.repo.GetAutomationFlow(pid)
	if err != nil || flowMap == nil {
		return // No flow or DB error
	}

	// 2. Parse Flow (JSON -> Struct)
	// We handle flexible JSON structure.
	// flowMap["nodes"] and flowMap["edges"] are expected to be []interface{} or JSON string or bytes

	flow := Flow{}

	// Safely decode nodes/edges. Assumes PGX returns unmarshaled maps/slices if JSONB.
	// If the driver returns []uint8 (bytes), we unmarshal.
	// To be safe, we might need a helper, but assuming map[string]interface{}:

	nodesRaw, ok1 := flowMap["nodes"]
	edgesRaw, ok2 := flowMap["edges"]

	if ok1 && ok2 {
		// Convert to strong types (simplified for brevity)
		bytesN, _ := json.Marshal(nodesRaw)
		bytesE, _ := json.Marshal(edgesRaw)
		json.Unmarshal(bytesN, &flow.Nodes)
		json.Unmarshal(bytesE, &flow.Edges)
	}

	// 3. Traverse
	// Start with triggers (Nodes that have no incoming edges? Or explicit 'trigger' type)
	// In Node-RED, triggers initiate.
	for _, n := range flow.Nodes {
		if n.Type == "trigger" {
			s.traverse(flow, n.ID, packet)
		}
	}
}

func (s *AutomationService) traverse(flow Flow, nodeId string, packet map[string]interface{}) {
	// Find Node
	var node *Node
	for i := range flow.Nodes {
		if flow.Nodes[i].ID == nodeId {
			node = &flow.Nodes[i]
			break
		}
	}
	if node == nil {
		return
	}

	// Execute
	continueFlow := s.executeNode(node, packet)

	if continueFlow {
		// Find Edges
		for _, e := range flow.Edges {
			if e.Source == nodeId {
				s.traverse(flow, e.Target, packet)
			}
		}
	}
}

func (s *AutomationService) executeNode(node *Node, packet map[string]interface{}) bool {
	data := node.Data

	switch node.Type {
	case "trigger":
		return true

	case "condition":
		// Extract logic
		param := data["param"].(string)
		op := data["op"].(string)
		threshold := data["val"].(float64)

		// Get Val from Packet payload
		payload, ok := packet["payload"].(map[string]interface{})
		if !ok {
			return false
		}

		val, exists := payload[param].(float64)
		if !exists {
			return false
		}

		switch op {
		case ">":
			return val > threshold
		case "<":
			return val < threshold
		default:
			return false
		}

	case "action":
		if data["type"] == "alert" {
			msg := data["message"].(string)
			pid := packet["project_id"].(string)
			// did := packet["device_id"].(string)
			did := "uuid-placeholder"

			log.Printf("🔔 ALERT: %s (Proj: %s)", msg, pid)
			s.repo.CreateAlert(did, pid, msg, "warning")
			// Send MQTT here...
		}
		return false // Terminal
	}
	return false
}
