package engine

import (
	"testing"

	"ingestion-go/internal/config/payloadschema"
)

func TestVerifyPayloadWithSchemas(t *testing.T) {
	project := &ProjectConfig{
		PayloadSchemas: map[string]payloadschema.PacketSchema{
			"data": {
				PacketType: "data",
				Keys: []payloadschema.KeySpec{
					{Key: "TIMESTAMP", Required: true},
					{Key: "PDKWH1"},
				},
				EnvelopeKeys: []payloadschema.KeySpec{
					{Key: "imei"},
					{Key: "project_id"},
				},
			},
		},
	}

	payload := map[string]interface{}{
		"TIMESTAMP":  "1",
		"PDKWH1":     42,
		"imei":       "123",
		"project_id": "proj-1",
	}

	ok, unknown := VerifyPayload(project, payload)
	if !ok {
		t.Fatalf("expected payload to be accepted, unknown=%v", unknown)
	}

	payload["extra"] = true
	if ok, unknown := VerifyPayload(project, payload); ok || len(unknown) == 0 {
		t.Fatalf("expected extra key to be flagged, got ok=%v unknown=%v", ok, unknown)
	}
}

func TestVerifyPayloadFallsBackToSensors(t *testing.T) {
	project := &ProjectConfig{
		Hardware: HardwareConfig{
			Sensors: []SensorConfig{{Param: "TEMP"}},
		},
	}

	payload := map[string]interface{}{"TEMP": 10, "imei": "123", "timestamp": 1, "project_id": "proj"}
	if ok, unknown := VerifyPayload(project, payload); !ok {
		t.Fatalf("expected fallback sensors to allow payload, unknown=%v", unknown)
	}
}

func TestVerifyPayloadMissingRequired(t *testing.T) {
	project := &ProjectConfig{
		PayloadSchemas: map[string]payloadschema.PacketSchema{},
	}

	payload := map[string]interface{}{"TEMP": 10}
	if ok, missing := VerifyPayload(project, payload); ok || len(missing) == 0 {
		t.Fatalf("expected required fields to be enforced, got ok=%v missing=%v", ok, missing)
	}
}
