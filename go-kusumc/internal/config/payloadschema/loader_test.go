package payloadschema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCSV(t *testing.T) {
	csvData := "packet_type,expected_for,scope_id,key,description,unit,required,max_length,topic_template,envelope_required,notes,value_min,value_max,resolution\n" +
		"envelope,global,,imei,Device IMEI,,Y,32,,Y,Primary identity,,,\n" +
		"data,project,PROJECT_X,PDKWH1,Energy (kWh),kWh,N,,<IMEI>/data,N,Daily energy,0,999.5,0.5\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "schema.csv")
	if err := os.WriteFile(path, []byte(csvData), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	entries, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV returned error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	envelope := entries[0]
	if !envelope.EnvelopeRequired {
		t.Fatalf("expected envelope entry to be marked as envelope key")
	}
	if envelope.MaxLength == nil || *envelope.MaxLength != 32 {
		t.Fatalf("expected envelope max length 32, got %#v", envelope.MaxLength)
	}
	if !envelope.Required {
		t.Fatalf("expected envelope required flag to be true")
	}

	data := entries[1]
	if data.PacketType != "data" || data.ExpectedFor != "project" {
		t.Fatalf("unexpected data metadata: %#v", data)
	}
	if data.Required {
		t.Fatalf("expected data required flag to be false")
	}
	if data.MaxLength != nil {
		t.Fatalf("expected data max length to be nil, got %#v", data.MaxLength)
	}
	if data.ValueMin == nil || *data.ValueMin != 0 {
		t.Fatalf("expected value min 0, got %#v", data.ValueMin)
	}
	if data.ValueMax == nil || *data.ValueMax != 999.5 {
		t.Fatalf("expected value max 999.5, got %#v", data.ValueMax)
	}
	if data.Resolution == nil || *data.Resolution != 0.5 {
		t.Fatalf("expected resolution 0.5, got %#v", data.Resolution)
	}
}

func TestBuildScopes(t *testing.T) {
	entries := []Entry{
		{
			PacketType:       "envelope",
			ExpectedFor:      "global",
			ScopeID:          "",
			Key:              "imei",
			Description:      "Device IMEI",
			Required:         true,
			MaxLength:        intPtr(32),
			EnvelopeRequired: true,
		},
		{
			PacketType:    "data",
			ExpectedFor:   "project",
			ScopeID:       "PROJECT_X",
			Key:           "PDKWH1",
			Description:   "Energy (kWh)",
			Unit:          "kWh",
			TopicTemplate: "<IMEI>/data",
		},
	}

	scopes := BuildScopes(entries)
	if len(scopes) != 2 {
		t.Fatalf("expected 2 distinct scopes, got %d", len(scopes))
	}

	globalScope, ok := scopes["global"]
	if !ok {
		t.Fatalf("missing global scope")
	}
	if len(globalScope.PacketSchemas) != 1 {
		t.Fatalf("expected 1 packet schema in global scope, got %d", len(globalScope.PacketSchemas))
	}
	envelopeSchema := globalScope.PacketSchemas["envelope"]
	if len(envelopeSchema.EnvelopeKeys) != 1 {
		t.Fatalf("expected 1 envelope key, got %d", len(envelopeSchema.EnvelopeKeys))
	}
	if envelopeSchema.EnvelopeKeys[0].Key != "imei" {
		t.Fatalf("unexpected envelope key: %s", envelopeSchema.EnvelopeKeys[0].Key)
	}

	projectScopeKey := ScopeKeyForProject("PROJECT_X")
	projectScope, ok := scopes[projectScopeKey]
	if !ok {
		t.Fatalf("missing project scope for key %s", projectScopeKey)
	}
	dataSchema, ok := projectScope.PacketSchemas["data"]
	if !ok {
		t.Fatalf("missing data schema in project scope")
	}
	if dataSchema.TopicTemplate != "<IMEI>/data" {
		t.Fatalf("unexpected topic template: %s", dataSchema.TopicTemplate)
	}
}

func intPtr(v int) *int {
	return &v
}
