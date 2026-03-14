package services

import (
	"testing"

	"ingestion-go/internal/config/payloadschema"
)

func TestAggregatePacketSchemas(t *testing.T) {
	globalScope := payloadschema.ScopeSchema{
		PacketSchemas: map[string]payloadschema.PacketSchema{
			"heartbeat": {
				PacketType:    "heartbeat",
				TopicTemplate: "<IMEI>/heartbeat",
				Keys: []payloadschema.KeySpec{
					{Key: "TIMESTAMP", Required: true},
				},
			},
		},
	}

	projectScope := payloadschema.ScopeSchema{
		PacketSchemas: map[string]payloadschema.PacketSchema{
			"heartbeat": {
				PacketType:    "heartbeat",
				TopicTemplate: "{IMEI}/heartbeat",
				Keys: []payloadschema.KeySpec{
					{Key: "TIMESTAMP", Description: "Device timestamp"},
					{Key: "RSSI", Notes: "Signal strength"},
				},
			},
			"data": {
				PacketType: "data",
				Keys: []payloadschema.KeySpec{
					{Key: "PDKWH1", Unit: "kWh"},
				},
			},
		},
	}

	scopeKey := payloadschema.ScopeKeyForProject("PROJECT_X")
	scopes := map[string]payloadschema.ScopeSchema{
		"global": globalScope,
		scopeKey: projectScope,
	}

	combined := aggregatePacketSchemas(scopeKey, scopes)
	if combined == nil {
		t.Fatalf("expected combined schema, got nil")
	}
	if len(combined) != 2 {
		t.Fatalf("expected 2 packet schemas, got %d", len(combined))
	}

	heartbeat := combined["heartbeat"]
	if heartbeat.TopicTemplate != "{IMEI}/heartbeat" {
		t.Fatalf("expected project topic template to win, got %s", heartbeat.TopicTemplate)
	}
	if len(heartbeat.Keys) != 2 {
		t.Fatalf("expected merged heartbeat keys, got %d", len(heartbeat.Keys))
	}
	if !heartbeat.Keys[0].Required {
		t.Fatalf("expected heartbeat timestamp to remain required")
	}

	data := combined["data"]
	if data.PacketType != "data" {
		t.Fatalf("expected data packet to be present")
	}

	// Ensure original scopes remain unchanged after merge
	if globalScope.PacketSchemas["heartbeat"].TopicTemplate != "<IMEI>/heartbeat" {
		t.Fatalf("global scope mutated during merge")
	}

	globalOnly := aggregatePacketSchemas(payloadschema.ScopeKeyForProject("UNKNOWN"), scopes)
	if len(globalOnly) != 1 {
		t.Fatalf("expected only global schema for unknown project, got %d", len(globalOnly))
	}

	if aggregatePacketSchemas(payloadschema.ScopeKeyForProject(""), nil) != nil {
		t.Fatalf("expected nil when no scope data provided")
	}
}
