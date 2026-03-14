package services_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"ingestion-go/internal/core/domain"
	"ingestion-go/internal/core/services"
	"ingestion-go/tests/mocks"
)

func TestProcessPacket_LegacyTopics_GovtMinimalFull_AllPersist(t *testing.T) {
	flushCh := make(chan map[string]interface{}, 8)

	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) { return true, nil },
		PushPacketFunc:  func(deviceId string, packet interface{}) error { return nil },
	}
	mockRepo := &mocks.MockTelemetryRepo{
		SaveBatchFunc: func(batch []interface{}) error {
			for _, row := range batch {
				envelope, ok := row.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unexpected envelope type: %T", row)
				}
				select {
				case flushCh <- envelope:
				default:
				}
			}
			return nil
		},
	}
	mockDeviceRepo := &mocks.MockDeviceRepo{
		GetDeviceByIMEIFunc: func(imei string) (interface{}, error) {
			if imei == "356000000000999" {
				return map[string]interface{}{
					"id":         "dev-legacy-01",
					"project_id": "proj_legacy",
				}, nil
			}
			return nil, errors.New("imei not found")
		},
	}

	deviceService := services.NewDeviceService(mockDeviceRepo, nil, nil, nil)
	realTransf := services.NewGovaluateTransformer()
	svc := services.NewIngestionService(mockState, mockRepo, nil, realTransf, deviceService, nil)

	tests := []struct {
		name           string
		topic          string
		payload        []byte
		expectType     string
		expectDeviceID string
		expectProject  string
	}{
		{
			name:  "govt-only heartbeat",
			topic: "356000000000999/heartbeat",
			payload: []byte(`{
				"VD":"1",
				"TIMESTAMP":"2026-02-28 10:15:30",
				"IMEI":"356000000000999",
				"ASN":"ASN-001",
				"RSSI":"-71"
			}`),
			expectType:     "heartbeat",
			expectDeviceID: "dev-legacy-01",
			expectProject:  "proj_legacy",
		},
		{
			name:  "minimal data",
			topic: "356000000000999/data",
			payload: []byte(`{
				"IMEI":"356000000000999",
				"TIMESTAMP":"2026-02-28 10:16:00",
				"PDKWH1":"14.2",
				"packet_type":"data",
				"msgid":"356000000000999-b1-0002",
				"ts":1761339360000
			}`),
			expectType:     "data",
			expectDeviceID: "dev-legacy-01",
			expectProject:  "proj_legacy",
		},
		{
			name:  "full envelope daq",
			topic: "356000000000999/daq",
			payload: []byte(`{
				"packet_type":"daq",
				"project_id":"proj_full",
				"device_id":"dev-full-01",
				"imei":"356000000000999",
				"msg_id":"daq-0001",
				"ts":1761339390000,
				"AI11":"12.4"
			}`),
			expectType:     "daq",
			expectDeviceID: "dev-full-01",
			expectProject:  "proj_full",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.ProcessPacket(tc.topic, tc.payload, ""); err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}

			select {
			case envelope := <-flushCh:
				if envelope["device_id"] != tc.expectDeviceID {
					t.Fatalf("expected device_id %q, got %#v", tc.expectDeviceID, envelope["device_id"])
				}
				if envelope["project_id"] != tc.expectProject {
					t.Fatalf("expected project_id %q, got %#v", tc.expectProject, envelope["project_id"])
				}
				payloadMap, ok := envelope["payload"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected payload map")
				}
				if payloadMap["packet_type"] != tc.expectType {
					t.Fatalf("expected packet_type %q, got %#v", tc.expectType, payloadMap["packet_type"])
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("timed out waiting for persisted envelope")
			}
		})
	}
}

func TestProcessPacket_Success(t *testing.T) {
	// Setup Mocks
	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) {
			return true, nil // Lock Acquired
		},
		PushPacketFunc: func(deviceId string, packet interface{}) error {
			return nil
		},
	}
	mockRepo := &mocks.MockTelemetryRepo{
		SaveTelemetryFunc: func(telemetry interface{}) error {
			// Verify fields
			m := telemetry.(map[string]interface{})
			if m["project_id"] != "proj_01" {
				return errors.New("wrong project id")
			}
			return nil
		},
	}
	// mockTransf := &mocks.MockTransformer{}

	svc := services.NewIngestionService(mockState, mockRepo, nil, nil, nil, nil)
	// Manually inject transformer if needed, or if interface allows.
	// IngestionService uses concrete *GovaluateTransformer currently in definition?
	// Let's check definition. It uses *GovaluateTransformer.
	// If I can't inject MockTransformer because of strict typing, I might need to refactor.
	// BUT for now, let's use the real transformer because it's pure logic (govaluate).
	// Or... wait. `NewIngestionService` takes `*GovaluateTransformer`.
	// I will use `nil` for now and assume it defaults or I will init a real one.

	// Real Transformer Init
	realTransf := services.NewGovaluateTransformer()
	svc = services.NewIngestionService(mockState, mockRepo, nil, realTransf, nil, nil)

	// Payload
	payload := []byte(`{"imei":"12345", "project_id":"proj_01", "temp": 25.5}`)

	// Execution
	err := svc.ProcessPacket("telemetry/12345", payload, "proj_01")

	// Assert
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestProcessPacket_LegacyTopicIMEIAndLegacyIMEIKey(t *testing.T) {
	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) { return true, nil },
		PushPacketFunc:  func(deviceId string, packet interface{}) error { return nil },
	}
	mockRepo := &mocks.MockTelemetryRepo{SaveBatchFunc: func(batch []interface{}) error { return nil }}

	realTransf := services.NewGovaluateTransformer()
	svc := services.NewIngestionService(mockState, mockRepo, nil, realTransf, nil, nil)

	// Legacy devices commonly send IMEI as "IMEI" and use per-device topics: <imei>/heartbeat
	payload := []byte(`{"IMEI":"356000000000999","TIMESTAMP":"2026-02-24T00:00:00Z"}`)

	if err := svc.ProcessPacket("356000000000999/heartbeat", payload, ""); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestProcessPacket_DuplicateLock(t *testing.T) {
	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) {
			return false, nil // Locked by another
		},
	}
	svc := services.NewIngestionService(mockState, &mocks.MockTelemetryRepo{}, nil, nil, nil, nil) // Transformer nil ok?

	payload := []byte(`{"imei":"123456"}`)
	err := svc.ProcessPacket("topic", payload, "")

	if err == nil {
		t.Fatal("Expected lock error")
	}
}

func TestProcessPacket_LegacyDataTopicInfersDataSchema(t *testing.T) {
	flushCh := make(chan map[string]interface{}, 1)

	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) { return true, nil },
		PushPacketFunc:  func(deviceId string, packet interface{}) error { return nil },
		GetConfigBundleFunc: func(projectId string) (map[string]interface{}, bool) {
			return map[string]interface{}{
				"payloadSchemas": map[string]interface{}{
					"data": map[string]interface{}{
						"packetType":    "data",
						"topicTemplate": "<IMEI>/data",
						"keys": []interface{}{
							map[string]interface{}{"key": "PDC1V1", "required": false},
						},
					},
				},
			}, true
		},
	}
	mockRepo := &mocks.MockTelemetryRepo{
		SaveBatchFunc: func(batch []interface{}) error {
			if len(batch) == 0 {
				return nil
			}
			envelope, ok := batch[0].(map[string]interface{})
			if !ok {
				return errors.New("unexpected envelope type")
			}
			select {
			case flushCh <- envelope:
			default:
			}
			return nil
		},
	}

	realTransf := services.NewGovaluateTransformer()
	svc := services.NewIngestionService(mockState, mockRepo, nil, realTransf, nil, nil)

	payload := []byte(`{"IMEI":"356000000000999","project_id":"proj_alpha","TIMESTAMP":"2026-02-24T00:00:00Z","PDC1V1":12.3}`)
	if err := svc.ProcessPacket("356000000000999/data", payload, ""); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	select {
	case envelope := <-flushCh:
		if envelope["status"] != "verified" {
			t.Fatalf("expected verified status, got: %#v", envelope["status"])
		}
		payload, ok := envelope["payload"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected payload map")
		}
		if payload["packet_type"] != "data" {
			t.Fatalf("expected packet_type data, got %#v", payload["packet_type"])
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for batch flush")
	}
}

func TestProcessPacket_ServerOndemandCommandEchoIgnored(t *testing.T) {
	flushCh := make(chan struct{}, 1)

	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) { return true, nil },
		PushPacketFunc:  func(deviceId string, packet interface{}) error { return nil },
	}
	mockRepo := &mocks.MockTelemetryRepo{
		SaveBatchFunc: func(batch []interface{}) error {
			select {
			case flushCh <- struct{}{}:
			default:
			}
			return nil
		},
	}

	realTransf := services.NewGovaluateTransformer()
	svc := services.NewIngestionService(mockState, mockRepo, nil, realTransf, nil, nil)

	// Mirrors strict govt legacy command shape (type/cmd/msgid)
	payload := []byte(`{"type":"ondemand_cmd","msgid":"cid-1","cmd":"do"}`)
	if err := svc.ProcessPacket("356000000000999/ondemand", payload, ""); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	select {
	case <-flushCh:
		t.Fatalf("expected ondemand command echo to be ignored (no persistence)")
	case <-time.After(600 * time.Millisecond):
		// ok
	}
}

func TestProcessPacket_OndemandResponseCorrelatesToLatestRequestWhenNoCorrelationID(t *testing.T) {
	statusCh := make(chan string, 1)

	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) { return true, nil },
		PushPacketFunc:  func(deviceId string, packet interface{}) error { return nil },
	}
	mockRepo := &mocks.MockTelemetryRepo{SaveBatchFunc: func(batch []interface{}) error { return nil }}
	mockCommands := &mocks.MockCommandRepo{
		ListCommandRequestsFunc: func(deviceID string, limit int) ([]domain.CommandRequest, error) {
			now := time.Now()
			pub := now.Add(-30 * time.Second)
			return []domain.CommandRequest{{
				DeviceID:      deviceID,
				ProjectID:     "proj_alpha",
				CommandID:     "cmd-1",
				Status:        "published",
				CorrelationID: "cid-1",
				CreatedAt:     now.Add(-1 * time.Minute),
				PublishedAt:   &pub,
			}}, nil
		},
		GetCommandRequestByCorrelationFunc: func(correlationID string) (*domain.CommandRequest, error) {
			if correlationID != "cid-1" {
				return nil, nil
			}
			req := &domain.CommandRequest{CorrelationID: "cid-1", CommandID: "cmd-1", DeviceID: "dev-1", ProjectID: "proj_alpha"}
			return req, nil
		},
		GetResponsePatternsFunc: func(commandID string) ([]domain.ResponsePattern, error) {
			return nil, nil
		},
		SaveCommandResponseFunc: func(resp domain.CommandResponse) error { return nil },
		UpdateCommandRequestStatusFunc: func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
			if correlationID == "cid-1" {
				select {
				case statusCh <- status:
				default:
				}
			}
			return nil
		},
		GetCommandByIDFunc: func(commandID string) (*domain.CommandCatalog, error) {
			return nil, nil
		},
	}

	realTransf := services.NewGovaluateTransformer()
	svc := services.NewIngestionService(mockState, mockRepo, mockCommands, realTransf, nil, nil)

	// Govt legacy response without correlation fields.
	payload := []byte(`{"imei":"356000000000999","device_id":"dev-1","project_id":"proj_alpha","status":"ack"}`)
	if err := svc.ProcessPacket("356000000000999/ondemand", payload, ""); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	select {
	case status := <-statusCh:
		if status != "acked" {
			t.Fatalf("expected status acked, got %q", status)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for command correlation update")
	}
}

func TestProcessPacket_ForwardedLegacyPayloadNormalizedAndVerified(t *testing.T) {
	flushCh := make(chan map[string]interface{}, 1)

	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) {
			return true, nil
		},
		PushPacketFunc: func(deviceId string, packet interface{}) error {
			return nil
		},
	}
	mockRepo := &mocks.MockTelemetryRepo{
		SaveBatchFunc: func(batch []interface{}) error {
			if len(batch) == 0 {
				return nil
			}
			envelope, ok := batch[0].(map[string]interface{})
			if !ok {
				return errors.New("unexpected envelope type")
			}
			select {
			case flushCh <- envelope:
			default:
			}
			return nil
		},
	}

	realTransf := services.NewGovaluateTransformer()
	svc := services.NewIngestionService(mockState, mockRepo, nil, realTransf, nil, nil)

	legacyForwarded := []byte(`{
		"imei":"356000000000999",
		"project_id":"proj_alpha",
		"msgid":"fwd-legacy-001",
		"packet_type":"forwarded_data",
		"metadata":{
			"forwarded":true,
			"origin_node_imei":"356000000000111"
		}
	}`)

	if err := svc.ProcessPacket("channels/proj_alpha/messages/356000000000999", legacyForwarded, "proj_alpha"); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	select {
	case envelope := <-flushCh:
		if envelope["status"] != "verified" {
			t.Fatalf("expected verified status, got: %#v", envelope["status"])
		}
		payload, ok := envelope["payload"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected payload map in envelope")
		}
		meta, ok := payload["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected metadata map in payload")
		}
		if forwarded, ok := meta["forwarded"].(bool); !ok || !forwarded {
			t.Fatalf("expected metadata.forwarded=true, got %#v", meta["forwarded"])
		}
		if meta["origin_imei"] != "356000000000111" {
			t.Fatalf("expected origin_imei normalized from origin_node_imei, got %#v", meta["origin_imei"])
		}
		route, ok := meta["route"].(map[string]interface{})
		if !ok || route == nil {
			t.Fatalf("expected metadata.route map to be normalized")
		}
		if ingress, _ := route["ingress"].(string); ingress == "" {
			t.Fatalf("expected route.ingress to be populated")
		}
		if _, ok := route["hops"]; !ok {
			t.Fatalf("expected route.hops to be populated")
		}
		path, ok := route["path"].([]interface{})
		if !ok || len(path) == 0 {
			t.Fatalf("expected route.path to be populated")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for batch flush")
	}
}

func TestProcessPacket_ForwardedPayloadWithoutOriginBecomesSuspicious(t *testing.T) {
	flushCh := make(chan map[string]interface{}, 1)

	mockState := &mocks.MockStateStore{
		AcquireLockFunc: func(key string, ttl int) (bool, error) {
			return true, nil
		},
		PushPacketFunc: func(deviceId string, packet interface{}) error {
			return nil
		},
	}
	mockRepo := &mocks.MockTelemetryRepo{
		SaveBatchFunc: func(batch []interface{}) error {
			if len(batch) == 0 {
				return nil
			}
			envelope, ok := batch[0].(map[string]interface{})
			if !ok {
				return errors.New("unexpected envelope type")
			}
			select {
			case flushCh <- envelope:
			default:
			}
			return nil
		},
	}

	realTransf := services.NewGovaluateTransformer()
	svc := services.NewIngestionService(mockState, mockRepo, nil, realTransf, nil, nil)

	brokenForwarded := []byte(`{
		"imei":"356000000000999",
		"project_id":"proj_alpha",
		"msgid":"fwd-broken-001",
		"packet_type":"forwarded_data",
		"metadata":{
			"forwarded":true,
			"route":{
				"path":["gateway-99"],
				"hops":0,
				"ingress":"mesh/lora"
			}
		}
	}`)

	if err := svc.ProcessPacket("channels/proj_alpha/messages/356000000000999", brokenForwarded, "proj_alpha"); err != nil {
		t.Fatalf("expected ingestion to continue with suspicious marking, got error: %v", err)
	}

	select {
	case envelope := <-flushCh:
		if envelope["status"] != "suspicious" {
			t.Fatalf("expected suspicious status, got: %#v", envelope["status"])
		}
		validation, ok := envelope["validation"].(map[string]interface{})
		if !ok || validation == nil {
			t.Fatalf("expected validation report for suspicious payload")
		}
		missing, ok := validation["missing"].([]string)
		if !ok {
			if generic, ok := validation["missing"].([]interface{}); ok {
				missing = make([]string, 0, len(generic))
				for _, item := range generic {
					if text, ok := item.(string); ok {
						missing = append(missing, text)
					}
				}
			}
		}
		foundOrigin := false
		for _, item := range missing {
			if item == "metadata.origin_imei|metadata.origin_node_id" {
				foundOrigin = true
				break
			}
		}
		if !foundOrigin {
			t.Fatalf("expected missing metadata.origin_imei|metadata.origin_node_id in validation, got %#v", validation)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for batch flush")
	}
}
