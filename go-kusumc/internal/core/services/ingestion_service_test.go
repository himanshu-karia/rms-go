package services

import (
	"errors"
	"testing"
	"time"

	"ingestion-go/internal/config/payloadschema"
	"ingestion-go/internal/core/domain"
	"ingestion-go/tests/mocks"
)

func TestHandleCommandResponse_PatternMatchRegex(t *testing.T) {
	cmdRepo := &mocks.MockCommandRepo{}

	// Simulate existing request
	cmdRepo.GetCommandRequestByCorrelationFunc = func(correlationID string) (*domain.CommandRequest, error) {
		return &domain.CommandRequest{ID: "r1", CommandID: "cmd-1"}, nil
	}

	// Provide a response pattern that matches regex and marks success=true
	cmdRepo.GetResponsePatternsFunc = func(commandID string) ([]domain.ResponsePattern, error) {
		return []domain.ResponsePattern{{ID: "p1", PatternType: "regex", Pattern: "\\\"status\\\":\\\"OK\\\"", Success: true}}, nil
	}

	saved := false
	var savedResp domain.CommandResponse
	cmdRepo.SaveCommandResponseFunc = func(resp domain.CommandResponse) error {
		saved = true
		savedResp = resp
		return nil
	}

	updated := false
	var updatedStatus string
	cmdRepo.UpdateCommandRequestStatusFunc = func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
		updated = true
		updatedStatus = status
		return nil
	}

	svc := &IngestionService{commands: cmdRepo}

	raw := map[string]interface{}{"device_id": "dev-1", "project_id": "proj-1"}
	processed := map[string]interface{}{"status": "OK", "correlation_id": "corr-1"}

	svc.handleCommandResponse("corr-1", raw, processed)

	if !saved {
		t.Fatalf("expected SaveCommandResponse to be called")
	}
	if !updated || updatedStatus != "acked" {
		t.Fatalf("expected UpdateCommandRequestStatus acked, got %v (updated=%v)", updatedStatus, updated)
	}
	if savedResp.CorrelationID != "corr-1" {
		t.Fatalf("unexpected saved correlation: %s", savedResp.CorrelationID)
	}
}

type mockDeviceRepo struct {
	device map[string]interface{}
	err    error
}

func (m *mockDeviceRepo) GetDeviceByIMEI(imei string) (interface{}, error)        { return m.device, m.err }
func (m *mockDeviceRepo) GetDeviceByID(id string) (map[string]interface{}, error) { return nil, nil }
func (m *mockDeviceRepo) GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) ListDevices(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error) {
	return nil, 0, nil
}
func (m *mockDeviceRepo) UpdateDeviceByIDOrIMEI(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) CreateDeviceStruct(projectId, name, imei string, mqttBundle map[string]interface{}, attrs map[string]interface{}) (string, error) {
	return "", nil
}
func (m *mockDeviceRepo) SoftDeleteDevice(idOrIMEI string) error {
	return nil
}
func (m *mockDeviceRepo) InsertCredentialHistory(deviceID string, bundle map[string]interface{}) (string, error) {
	return "", nil
}
func (m *mockDeviceRepo) GetAutomationFlow(projectId string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) GetInstallationByDevice(deviceId string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) GetBeneficiary(id string) (map[string]interface{}, error) { return nil, nil }
func (m *mockDeviceRepo) CreateMqttProvisioningJob(deviceId string, credHistoryId *string) error {
	return nil
}
func (m *mockDeviceRepo) GetLatestCredentialHistory(deviceId string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) ListCredentialHistory(deviceId string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) GetPendingCommands(deviceId string) ([]map[string]interface{}, error) {
	return nil, nil
}

func TestExtractPayloadSchemas(t *testing.T) {
	project := map[string]interface{}{
		"config": map[string]interface{}{
			"payloadSchemas": map[string]interface{}{
				"heartbeat": map[string]interface{}{
					"packetType":    "heartbeat",
					"topicTemplate": "<IMEI>/heartbeat",
					"keys": []interface{}{
						map[string]interface{}{"key": "TIMESTAMP", "required": true},
						map[string]interface{}{"key": "RSSI"},
					},
				},
			},
		},
	}

	schemas := extractPayloadSchemas(project)
	if schemas == nil {
		t.Fatalf("expected schemas to be parsed")
	}
	heartbeat, ok := schemas["heartbeat"]
	if !ok {
		t.Fatalf("expected heartbeat schema to be present")
	}
	if heartbeat.TopicTemplate != "<IMEI>/heartbeat" {
		t.Fatalf("unexpected topic template: %s", heartbeat.TopicTemplate)
	}
	if len(heartbeat.Keys) != 2 {
		t.Fatalf("expected 2 key specs, got %d", len(heartbeat.Keys))
	}
	if !heartbeat.Keys[0].Required {
		t.Fatalf("expected required flag to survive decoding")
	}
}

func TestResolveDeviceIdentity(t *testing.T) {
	svc := NewDeviceService(&mockDeviceRepo{device: map[string]interface{}{
		"id":         "dev-123",
		"project_id": "proj-1",
	}}, nil, nil, nil)

	devID, pid, err := svc.ResolveDeviceIdentity("imeix")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if devID != "dev-123" || pid != "proj-1" {
		t.Fatalf("unexpected identity: %s %s", devID, pid)
	}

	svcErr := NewDeviceService(&mockDeviceRepo{err: errors.New("fail")}, nil, nil, nil)
	if _, _, err := svcErr.ResolveDeviceIdentity("imeix"); err == nil {
		t.Fatalf("expected error when repo fails")
	}
}

func TestDetectPacketType(t *testing.T) {
	schemas := map[string]payloadschema.PacketSchema{
		"heartbeat": {PacketType: "heartbeat", TopicTemplate: "<IMEI>/heartbeat"},
		"legacy":    {PacketType: "legacy", TopicTemplate: "<IMEI>/legacy"},
	}

	raw := map[string]interface{}{"imei": "12345"}
	if pt := detectPacketType(raw, "12345/heartbeat", schemas, "12345", ""); pt != "heartbeat" {
		t.Fatalf("expected heartbeat from topic match, got %s", pt)
	}

	raw["packet_type"] = "legacy"
	if pt := detectPacketType(raw, "ignored", schemas, "12345", ""); pt != "legacy" {
		t.Fatalf("packet_type field should remain legacy before strict validation stage, got %s", pt)
	}

	raw["packet_type"] = "data"
	if pt := detectPacketType(raw, "ignored", schemas, "12345", ""); pt != "data" {
		t.Fatalf("packet_type=data should remain data, got %s", pt)
	}

	delete(raw, "packet_type")
	if pt := detectPacketType(raw, "channels/proj-a/messages/12345/data", schemas, "12345", "proj-a"); pt != "data" {
		t.Fatalf("channels compact data suffix should infer data, got %s", pt)
	}
	if pt := detectPacketType(raw, "channels/proj-a/messages/12345/legacy", schemas, "12345", "proj-a"); pt != "" {
		t.Fatalf("channels compact legacy suffix should no longer infer packet type, got %s", pt)
	}
	if pt := detectPacketType(raw, "devices/12345/telemetry/daq", schemas, "12345", ""); pt != "daq" {
		t.Fatalf("devices compact daq suffix should infer daq, got %s", pt)
	}
}

func TestHasUnsupportedTelemetrySuffix(t *testing.T) {
	if !hasUnsupportedTelemetrySuffix("356000000000001/legacy") {
		t.Fatalf("expected legacy suffix to be detected")
	}
	if !hasUnsupportedTelemetrySuffix("channels/proj/messages/12345/legacy") {
		t.Fatalf("expected channels legacy suffix to be detected")
	}
	if !hasUnsupportedTelemetrySuffix("devices/12345/telemetry/legacy") {
		t.Fatalf("expected devices telemetry legacy suffix to be detected")
	}
	if hasUnsupportedTelemetrySuffix("356000000000001/data") {
		t.Fatalf("did not expect data suffix to be treated as unsupported")
	}
}

func TestValidatePacketQualityWithSchema(t *testing.T) {
	svc := &IngestionService{}
	schema := &payloadschema.PacketSchema{
		PacketType: "data",
		Keys: []payloadschema.KeySpec{
			{Key: "TIMESTAMP", Required: true},
			{Key: "PDKWH1", MaxLength: func() *int { v := 5; return &v }()},
		},
	}

	report := svc.validatePacketQuality(map[string]interface{}{"TIMESTAMP": "1", "PDKWH1": "123"}, nil, schema)
	if !report.ok() {
		t.Fatalf("expected payload to be valid, report=%v", report)
	}

	if report := svc.validatePacketQuality(map[string]interface{}{"PDKWH1": "123"}, nil, schema); report.ok() || len(report.Missing) == 0 {
		t.Fatalf("missing required key should fail validation, report=%v", report)
	}

	if report := svc.validatePacketQuality(map[string]interface{}{"TIMESTAMP": "1", "PDKWH1": "123456"}, nil, schema); report.ok() || len(report.Oversized) == 0 {
		t.Fatalf("value exceeding max length should fail validation, report=%v", report)
	}

	if report := svc.validatePacketQuality(map[string]interface{}{"TIMESTAMP": "1", "PDKWH1": "123", "EXTRA": 10}, nil, schema); report.ok() || len(report.Unknown) == 0 {
		t.Fatalf("unknown key should fail validation, report=%v", report)
	}
}

func TestValidatePacketQuality_AllowsPacketTypeBuiltin(t *testing.T) {
	svc := &IngestionService{}
	report := svc.validatePacketQuality(map[string]interface{}{
		"imei":        "356000000000001",
		"project_id":  "proj-alpha",
		"msgid":       "m-1",
		"packet_type": "heartbeat",
		"metadata":    map[string]interface{}{},
	}, nil, nil)

	if !report.ok() {
		t.Fatalf("expected packet_type to be accepted as builtin, report=%v", report)
	}
}

func TestResolvePacketEventTime_PrefersTsMillis(t *testing.T) {
	processed := map[string]interface{}{"ts": float64(1760870400123)}
	got := resolvePacketEventTime(processed, nil)
	if got.UnixMilli() != 1760870400123 {
		t.Fatalf("expected unix ms 1760870400123, got %d", got.UnixMilli())
	}
}

func TestResolvePacketEventTime_ParsesRFC3339(t *testing.T) {
	raw := map[string]interface{}{"timestamp": "2026-02-25T10:00:00Z"}
	got := resolvePacketEventTime(nil, raw)
	if got.UTC().Format(time.RFC3339) != "2026-02-25T10:00:00Z" {
		t.Fatalf("expected RFC3339 timestamp parse, got %s", got.UTC().Format(time.RFC3339))
	}
}

func TestResolvePacketEventTime_ParsesEpochSeconds(t *testing.T) {
	processed := map[string]interface{}{"timestamp": int64(1760870400)}
	got := resolvePacketEventTime(processed, nil)
	if got.Unix() != 1760870400 {
		t.Fatalf("expected unix seconds 1760870400, got %d", got.Unix())
	}
}

func TestResolvePacketEventTime_FallsBackToNow(t *testing.T) {
	before := time.Now().UTC().Add(-2 * time.Second)
	got := resolvePacketEventTime(nil, nil)
	after := time.Now().UTC().Add(2 * time.Second)
	if got.Before(before) || got.After(after) {
		t.Fatalf("expected fallback around now, got %s", got.Format(time.RFC3339Nano))
	}
}

func TestValidatePacketQuality_ForwardedRoutingRequired(t *testing.T) {
	svc := &IngestionService{}
	report := svc.validatePacketQuality(map[string]interface{}{
		"imei":        "356000000000999",
		"project_id":  "proj-alpha",
		"msgid":       "fwd-1",
		"packet_type": "forwarded_data",
		"metadata": map[string]interface{}{
			"forwarded": true,
			"route": map[string]interface{}{
				"path":    []interface{}{"field-node-11", "gateway-99"},
				"hops":    float64(1),
				"ingress": "mesh/lora",
			},
		},
	}, nil, nil)

	if report.ok() {
		t.Fatalf("expected missing origin_imei to fail validation")
	}

	missingOrigin := false
	for _, key := range report.Missing {
		if key == "metadata.origin_imei|metadata.origin_node_id" {
			missingOrigin = true
			break
		}
	}
	if !missingOrigin {
		t.Fatalf("expected metadata.origin_imei|metadata.origin_node_id missing key, report=%v", report)
	}
}

func TestValidatePacketQuality_ForwardedRoutingValid(t *testing.T) {
	svc := &IngestionService{}
	report := svc.validatePacketQuality(map[string]interface{}{
		"imei":        "356000000000999",
		"project_id":  "proj-alpha",
		"msgid":       "fwd-2",
		"packet_type": "forwarded_data",
		"metadata": map[string]interface{}{
			"forwarded":   true,
			"origin_imei": "356000000000111",
			"route": map[string]interface{}{
				"path":    []interface{}{"field-node-11", "repeater-07", "gateway-99"},
				"hops":    float64(2),
				"ingress": "mesh/lora",
			},
		},
	}, nil, nil)

	if !report.ok() {
		t.Fatalf("expected forwarded payload routing metadata to pass, report=%v", report)
	}
}

func TestNormalizeForwardedTelemetryPayload_FillsDefaults(t *testing.T) {
	processed := map[string]interface{}{
		"imei":        "356000000000999",
		"project_id":  "proj-alpha",
		"packet_type": "forwarded_data",
		"metadata": map[string]interface{}{
			"origin_node_imei": "356000000000111",
		},
	}

	normalizeForwardedTelemetryPayload(processed, processed, "channels/proj-alpha/messages/356000000000999", "356000000000999")

	meta, _ := processed["metadata"].(map[string]interface{})
	if forwarded, ok := meta["forwarded"].(bool); !ok || !forwarded {
		t.Fatalf("expected metadata.forwarded=true, got %#v", meta["forwarded"])
	}
	if meta["origin_imei"] != "356000000000111" {
		t.Fatalf("expected origin_imei fallback from origin_node_imei, got %#v", meta["origin_imei"])
	}
	route, _ := meta["route"].(map[string]interface{})
	if route == nil {
		t.Fatalf("expected route object to be created")
	}
	if ingress, _ := route["ingress"].(string); ingress == "" {
		t.Fatalf("expected route.ingress to be defaulted")
	}
	path, _ := route["path"].([]interface{})
	if len(path) == 0 {
		t.Fatalf("expected route.path default")
	}
	if hops, ok := route["hops"].(int); !ok || hops != len(path)-1 {
		t.Fatalf("expected route.hops default, got %#v", route["hops"])
	}
}

func TestNormalizeForwardedTelemetryPayload_InjectsMetadataFromRaw(t *testing.T) {
	processed := map[string]interface{}{
		"imei":       "356000000000999",
		"project_id": "proj-alpha",
	}
	raw := map[string]interface{}{
		"imei":        "356000000000999",
		"project_id":  "proj-alpha",
		"packet_type": "forwarded_data",
		"metadata": map[string]interface{}{
			"forwarded":   true,
			"origin_imei": "356000000000111",
			"route": map[string]interface{}{
				"path":    []interface{}{"field-1", "gw-1"},
				"hops":    float64(1),
				"ingress": "mesh/lora",
			},
		},
	}

	normalizeForwardedTelemetryPayload(processed, raw, "channels/proj-alpha/messages/356000000000999", "356000000000999")

	if processed["packet_type"] != "forwarded_data" {
		t.Fatalf("expected packet_type injected from raw, got %#v", processed["packet_type"])
	}
	meta, _ := processed["metadata"].(map[string]interface{})
	if meta == nil {
		t.Fatalf("expected metadata to be injected from raw")
	}
	report := (&IngestionService{}).validatePacketQuality(processed, nil, nil)
	if !report.ok() {
		t.Fatalf("expected normalized forwarded payload to pass strict validation, report=%v", report)
	}
}
