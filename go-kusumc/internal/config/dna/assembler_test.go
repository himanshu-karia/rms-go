package dna

import (
	"reflect"
	"testing"

	"ingestion-go/internal/config/payloadschema"
)

func TestAssembleScopes(t *testing.T) {
	records := []ProjectPayloadSchema{
		{
			ProjectID: "proj-1",
			Rows: []payloadschema.Entry{
				{
					PacketType:    "uplink",
					ExpectedFor:   "project",
					ScopeID:       "proj-1",
					Key:           "temperature",
					Description:   "Temperature",
					Required:      true,
					TopicTemplate: "<IMEI>/data",
				},
				{
					PacketType:       "uplink",
					ExpectedFor:      "project",
					ScopeID:          "proj-1",
					Key:              "envelope_key",
					Description:      "Envelope",
					EnvelopeRequired: true,
				},
			},
		},
		{
			ProjectID: "proj-2",
			Rows: []payloadschema.Entry{
				{
					PacketType:  "uplink",
					Key:         "humidity",
					Description: "Humidity",
				},
			},
		},
	}

	scopes, err := AssembleScopes(records)
	if err != nil {
		t.Fatalf("assemble scopes: %v", err)
	}

	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}

	scope1, ok := scopes["project:proj-1"]
	if !ok {
		t.Fatalf("scope project:proj-1 missing")
	}
	if scope1.ScopeID != "proj-1" || scope1.ExpectedFor != "project" {
		t.Fatalf("unexpected scope1 metadata: %#v", scope1)
	}
	packet := scope1.PacketSchemas["uplink"]
	if packet.TopicTemplate != "<IMEI>/data" {
		t.Fatalf("topic template lost, got %q", packet.TopicTemplate)
	}
	if len(packet.Keys) != 1 || packet.Keys[0].Key != "temperature" {
		t.Fatalf("unexpected packet keys: %#v", packet.Keys)
	}
	if len(packet.EnvelopeKeys) != 1 || packet.EnvelopeKeys[0].Key != "envelope_key" {
		t.Fatalf("envelope key missing: %#v", packet.EnvelopeKeys)
	}

	scope2, ok := scopes["project:proj-2"]
	if !ok {
		t.Fatalf("scope project:proj-2 missing")
	}
	if scope2.ExpectedFor != "project" || scope2.ScopeID != "proj-2" {
		t.Fatalf("normalisation failed: %#v", scope2)
	}
	packet2 := scope2.PacketSchemas["uplink"]
	wantKey := payloadschema.KeySpec{Key: "humidity", Description: "Humidity"}
	if !reflect.DeepEqual(packet2.Keys, []payloadschema.KeySpec{wantKey}) {
		t.Fatalf("unexpected packet2 keys: %#v", packet2.Keys)
	}
}

func TestAssembleScopesMissingProjectID(t *testing.T) {
	records := []ProjectPayloadSchema{{}}

	_, err := AssembleScopes(records)
	if err == nil {
		t.Fatal("expected error when project id missing")
	}
}
