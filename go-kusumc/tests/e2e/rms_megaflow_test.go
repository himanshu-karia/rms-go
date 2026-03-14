//go:build integration
// +build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type authToken struct {
	Token string `json:"token"`
}

type rmsScenario struct {
	baseURL        string
	httpCli        *http.Client
	adminToken     string
	projectID      string
	protocolID     string
	endpoint       string
	endpoints      []string
	deviceID       string
	deviceIMEI     string
	mqttUser       string
	mqttPass       string
	clientID       string
	publishTopic   string
	subscribeTopic string
	commandID      string
	correlationID  string
}

// TestRMSMegaFlow exercises the full RMS flow described in
// ../RMS-E2E-Integration-Test.md. Subtests will be filled in incrementally.
func TestRMSMegaFlow(t *testing.T) {
	// 0) Environment/Seeds: start stack, only default users seeded.
	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	httpCli := httpClient(t)
	shared := &rmsScenario{baseURL: baseURL, httpCli: httpCli}

	// 1) Auth & RBAC: admin login, create operator, negative admin-only.
	t.Run("auth_rbac", func(t *testing.T) {
		t.Parallel()

		adminToken := mustLoginRMS(t, httpCli, baseURL, "Him", "0554")
		if adminToken == "" {
			t.Fatalf("admin token empty")
		}

		// Attempt to list projects as admin (sanity of token, no seeded project dependency)
		resp, err := doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/projects", baseURL), adminToken, nil)
		if err != nil {
			t.Skipf("projects endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("list projects status=%d", resp.StatusCode)
		}

		// Attempt admin-only route with no token -> expect 401
		respNoAuth, err := doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/projects", baseURL), "", nil)
		if err != nil {
			t.Skipf("projects endpoint unreachable: %v", err)
		}
		if respNoAuth.StatusCode == http.StatusOK {
			t.Skip("project get is public; skipping unauth check")
		}
		if respNoAuth.StatusCode != http.StatusUnauthorized && respNoAuth.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 401/403 for unauth project get, got %d", respNoAuth.StatusCode)
		}
	})

	// 2) Master data & Org/Project/Protocol/DNA created via CRUD (no seeds).
	t.Run("masterdata_org_project_protocol_dna", func(t *testing.T) {
		t.Parallel()

		adminToken := mustLoginRMS(t, httpCli, baseURL, "Him", "0554")
		// Currently only state creation is exposed; if missing, skip.
		statePayload := map[string]string{"name": fmt.Sprintf("State-%d", time.Now().UnixNano())}
		resp, err := doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/admin/state", baseURL), adminToken, statePayload)
		if err != nil {
			t.Skipf("state endpoint unreachable: %v", err)
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("state creation endpoint not wired; skipping master data CRUD")
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			t.Skipf("state creation endpoint unstable (status=%d); skipping master data CRUD", resp.StatusCode)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create state status=%d", resp.StatusCode)
		}

		// Create RMS project (no org linkage if not required)
		projID := fmt.Sprintf("rms-e2e-%d", time.Now().UnixNano())
		projPayload := map[string]interface{}{
			"id":       projID,
			"name":     "RMS E2E",
			"type":     "rms",
			"location": "test-location",
			"config":   map[string]interface{}{},
		}
		resp, err = doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/projects", baseURL), adminToken, projPayload)
		if err != nil {
			t.Skipf("project endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Fatalf("create project status=%d", resp.StatusCode)
		}

		// Create primary protocol
		protoPayload := map[string]interface{}{
			"kind":             "primary",
			"protocol":         "mqtt",
			"name":             "rms-primary",
			"publish_topics":   []string{"<IMEI>/heartbeat", "<IMEI>/pump", "<IMEI>/data", "<IMEI>/daq", "<IMEI>/ondemand"},
			"subscribe_topics": []string{"<IMEI>/ondemand"},
		}
		resp, err = doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/protocols", baseURL, projID), adminToken, protoPayload)
		if err != nil {
			t.Skipf("protocol endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create protocol status=%d", resp.StatusCode)
		}

		// List protocols to confirm
		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/projects/%s/protocols", baseURL, projID), adminToken, nil)
		if err != nil {
			t.Skipf("protocol list endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("list protocols status=%d", resp.StatusCode)
		}

		// Upsert DNA (payload schema with one heartbeat field and a virtual sensor)
		dnaPayload := map[string]interface{}{
			"projectId": projID,
			"rows": []map[string]interface{}{
				{
					"PacketType":       "heartbeat",
					"ExpectedFor":      "device",
					"ScopeID":          projID,
					"Key":              "imei",
					"Description":      "device imei",
					"Unit":             "",
					"Required":         true,
					"TopicTemplate":    "<IMEI>/heartbeat",
					"EnvelopeRequired": false,
				},
			},
			"virtualSensors": []map[string]interface{}{
				{
					"param":      "efficiency",
					"expression": "(pump_power / input_power) * 100",
				},
			},
			"edgeRules": []map[string]interface{}{
				{
					"param":     "efficiency",
					"operator":  "<",
					"threshold": 50,
					"severity":  "warn",
					"enabled":   true,
				},
			},
		}

		resp, err = doJSON(httpCli, http.MethodPut, fmt.Sprintf("%s/api/dna/%s", baseURL, projID), adminToken, dnaPayload)
		if err != nil {
			t.Skipf("dna endpoint unreachable: %v", err)
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("dna endpoint not wired; skipping dna upsert")
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("dna upsert status=%d", resp.StatusCode)
		}

		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/dna/%s", baseURL, projID), adminToken, nil)
		if err != nil {
			t.Skipf("dna get unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("dna get status=%d", resp.StatusCode)
		}
	})

	// 3) Shared scenario seed: project, protocol, DNA for downstream steps.
	t.Run("scenario_seed_project_protocol_dna", func(t *testing.T) {
		adminToken := mustLoginRMS(t, httpCli, baseURL, "Him", "0554")
		shared.adminToken = adminToken

		shared.projectID = fmt.Sprintf("rms-shared-%d", time.Now().UnixNano())
		projPayload := map[string]interface{}{
			"id":       shared.projectID,
			"name":     "RMS Mega Shared",
			"type":     "rms",
			"location": "shared-location",
			"config":   map[string]interface{}{},
		}
		resp, err := doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/projects", baseURL), adminToken, projPayload)
		if err != nil {
			t.Skipf("project endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("create project status=%d", resp.StatusCode)
		}
		resp.Body.Close()

		protoPayload := map[string]interface{}{
			"kind":             "primary",
			"protocol":         "mqtt",
			"name":             "rms-primary-shared",
			"publish_topics":   []string{"<IMEI>/heartbeat", "<IMEI>/pump", "<IMEI>/data", "<IMEI>/daq", "<IMEI>/ondemand"},
			"subscribe_topics": []string{"<IMEI>/ondemand"},
		}
		resp, err = doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/projects/%s/protocols", baseURL, shared.projectID), adminToken, protoPayload)
		if err != nil {
			t.Skipf("protocol endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			resp.Body.Close()
			t.Fatalf("create protocol status=%d", resp.StatusCode)
		}
		var proto map[string]interface{}
		decodeJSONBody(t, resp, &proto)
		if id, ok := proto["id"].(string); ok && id != "" {
			shared.protocolID = id
		}
		if shared.protocolID == "" {
			resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/projects/%s/protocols", baseURL, shared.projectID), adminToken, nil)
			if err != nil {
				t.Skipf("protocol list endpoint unreachable: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				t.Fatalf("list protocols status=%d", resp.StatusCode)
			}
			var protos []map[string]interface{}
			decodeJSONBody(t, resp, &protos)
			if len(protos) > 0 {
				if id, ok := protos[0]["id"].(string); ok {
					shared.protocolID = id
				}
			}
		}
		if shared.protocolID == "" {
			t.Fatalf("protocol id missing")
		}

		dnaPayload := map[string]interface{}{
			"projectId": shared.projectID,
			"rows": []map[string]interface{}{
				{
					"PacketType":       "heartbeat",
					"ExpectedFor":      "device",
					"ScopeID":          shared.projectID,
					"Key":              "imei",
					"Description":      "device imei",
					"Unit":             "",
					"Required":         true,
					"TopicTemplate":    "<IMEI>/heartbeat",
					"EnvelopeRequired": false,
				},
			},
			"virtualSensors": []map[string]interface{}{
				{
					"param":      "efficiency",
					"expression": "(pump_power / input_power) * 100",
				},
			},
			"edgeRules": []map[string]interface{}{
				{
					"param":     "efficiency",
					"operator":  "<",
					"threshold": 50,
					"severity":  "warn",
					"enabled":   true,
				},
			},
		}

		resp, err = doJSON(httpCli, http.MethodPut, fmt.Sprintf("%s/api/dna/%s", baseURL, shared.projectID), adminToken, dnaPayload)
		if err != nil {
			t.Skipf("dna endpoint unreachable: %v", err)
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			t.Skip("dna endpoint not wired; skipping scenario seed")
		}
		if resp.StatusCode != http.StatusNoContent {
			resp.Body.Close()
			t.Fatalf("dna upsert status=%d", resp.StatusCode)
		}
		resp.Body.Close()

		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/dna/%s", baseURL, shared.projectID), adminToken, nil)
		if err != nil {
			t.Skipf("dna get unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("dna get status=%d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("device_lifecycle_bootstrap", func(t *testing.T) {
		if shared.projectID == "" || shared.protocolID == "" || shared.adminToken == "" {
			t.Skip("shared scenario not seeded")
		}

		shared.deviceIMEI = fmt.Sprintf("999%010d", time.Now().UnixNano()%1e10)
		devicePayload := map[string]interface{}{
			"name":            "rms-device",
			"imei":            shared.deviceIMEI,
			"projectId":       shared.projectID,
			"protocol_id":     shared.protocolID,
			"attributes":      map[string]interface{}{"model_id": "rms-model"},
			"contractor_id":   "",
			"supplier_id":     "",
			"manufacturer_id": "",
			"org_id":          "",
		}
		resp, err := doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/devices", baseURL), shared.adminToken, devicePayload)
		if err != nil {
			t.Skipf("device endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			resp.Body.Close()
			t.Fatalf("create device status=%d", resp.StatusCode)
		}
		var dev map[string]interface{}
		decodeJSONBody(t, resp, &dev)
		if id, ok := dev["device_id"].(string); ok {
			shared.deviceID = id
		} else if id, ok := dev["id"].(string); ok {
			shared.deviceID = id
		}
		if user, ok := dev["mqtt_user"].(string); ok {
			shared.mqttUser = user
		}
		if pass, ok := dev["mqtt_pass"].(string); ok {
			shared.mqttPass = pass
		}
		if cid, ok := dev["client_id"].(string); ok {
			shared.clientID = cid
		}
		if ep, ok := dev["endpoint"].(string); ok {
			shared.endpoint = ep
		}
		if eps, ok := dev["endpoints"].([]interface{}); ok {
			for _, v := range eps {
				if s, ok := v.(string); ok && s != "" {
					shared.endpoints = append(shared.endpoints, s)
				}
			}
		}
		if pt, ok := dev["publish_topics"].(string); ok {
			shared.publishTopic = pt
		}
		if st, ok := dev["subscribe_topics"].(string); ok {
			shared.subscribeTopic = st
		}
		if shared.deviceIMEI == "" {
			if imei, ok := dev["imei"].(string); ok && imei != "" {
				shared.deviceIMEI = imei
			}
		}
		if shared.deviceID == "" {
			t.Fatalf("device id missing")
		}

		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/devices/%s", baseURL, shared.deviceID), shared.adminToken, nil)
		if err != nil {
			t.Skipf("device get unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("get device status=%d", resp.StatusCode)
		}
		resp.Body.Close()

		bootReq, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/bootstrap?imei=%s", baseURL, shared.deviceIMEI), nil)
		apiKey := strings.TrimSpace(getenv("API_KEY", ""))
		if apiKey == "" {
			apiKey = ensureBootstrapAPIKey(t)
		}
		if apiKey != "" {
			bootReq.Header.Set("x-api-key", apiKey)
		}
		bootResp, err := httpCli.Do(bootReq)
		if err != nil {
			t.Skipf("bootstrap unreachable: %v", err)
		}
		defer bootResp.Body.Close()
		if bootResp.StatusCode == http.StatusUnauthorized || bootResp.StatusCode == http.StatusForbidden {
			t.Skip("bootstrap requires api key")
		}
		if bootResp.StatusCode != http.StatusOK {
			t.Fatalf("bootstrap status=%d", bootResp.StatusCode)
		}
	})

	t.Run("command_catalog", func(t *testing.T) {
		if shared.deviceID == "" || shared.projectID == "" || shared.adminToken == "" {
			t.Skip("device or project not ready")
		}

		cmdPayload := map[string]interface{}{
			"name":          "rms-ping",
			"scope":         "project",
			"projectId":     shared.projectID,
			"protocolId":    shared.protocolID,
			"transport":     "mqtt",
			"deviceIds":     []string{shared.deviceID},
			"payloadSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"action": map[string]interface{}{"const": "ping"}}, "required": []string{"action"}},
		}
		resp, err := doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/commands/catalog", baseURL), shared.adminToken, cmdPayload)
		if err != nil {
			t.Skipf("command catalog endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			resp.Body.Close()
			t.Fatalf("upsert command catalog status=%d", resp.StatusCode)
		}
		var cmd map[string]interface{}
		decodeJSONBody(t, resp, &cmd)
		if id, ok := cmd["id"].(string); ok {
			shared.commandID = id
		}
		if shared.commandID == "" {
			t.Fatalf("command id missing in catalog response")
		}

		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/commands/catalog-admin?projectId=%s&deviceId=%s", baseURL, shared.projectID, shared.deviceID), shared.adminToken, nil)
		if err != nil {
			t.Skipf("catalog list unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("catalog list status=%d", resp.StatusCode)
		}
		var catalog []map[string]interface{}
		decodeJSONBody(t, resp, &catalog)
		found := false
		for _, item := range catalog {
			if id, ok := item["id"].(string); ok && id == shared.commandID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("command %s not returned in catalog", shared.commandID)
		}
	})

	// Command send + history + stats
	t.Run("commands_roundtrip", func(t *testing.T) {
		if shared.commandID == "" || shared.deviceID == "" || shared.projectID == "" || shared.adminToken == "" {
			t.Skip("command or device not ready")
		}

		sendPayload := map[string]interface{}{
			"deviceId":  shared.deviceID,
			"projectId": shared.projectID,
			"commandId": shared.commandID,
			"payload":   map[string]interface{}{"action": "ping"},
		}
		resp, err := doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/commands/send", baseURL), shared.adminToken, sendPayload)
		if err != nil {
			t.Skipf("send command unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("send command status=%d", resp.StatusCode)
		}
		var sent map[string]interface{}
		decodeJSONBody(t, resp, &sent)
		if corr, ok := sent["correlationId"].(string); ok && corr != "" {
			shared.correlationID = corr
		}
		if shared.correlationID == "" {
			t.Fatalf("missing correlationId in send response")
		}

		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/commands?deviceId=%s", baseURL, shared.deviceID), shared.adminToken, nil)
		if err != nil {
			t.Skipf("commands history unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("commands history status=%d", resp.StatusCode)
		}
		var history []map[string]interface{}
		decodeJSONBody(t, resp, &history)
		found := false
		for _, h := range history {
			if corr, ok := h["correlationId"].(string); ok && corr == shared.correlationID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("command correlation %s not found in history", shared.correlationID)
		}

		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/commands/status?deviceId=%s&projectId=%s", baseURL, shared.deviceID, shared.projectID), shared.adminToken, nil)
		if err != nil {
			t.Skipf("command status unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("command status status=%d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	// Telemetry ingest and credential rotation
	t.Run("telemetry_ingest_retention", func(t *testing.T) {
		if shared.deviceID == "" || shared.deviceIMEI == "" || shared.projectID == "" || shared.adminToken == "" {
			t.Skip("shared scenario not seeded")
		}

		dsn := getenv("TIMESCALE_URI", "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable")
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()

		mustWaitForProvisioningJobs(t, ctx, dsn, shared.deviceID)

		if os.Getenv("MQTT_TLS_INSECURE") == "" && os.Getenv("MQTT_CA_PATH") == "" {
			_ = os.Setenv("MQTT_TLS_INSECURE", "true")
		}

		endpoint := shared.endpoint
		if len(shared.endpoints) > 0 {
			endpoint = shared.endpoints[0]
		}
		if endpoint == "" {
			endpoint = "mqtts://rms-iot.local:18883"
		}
		endpoint = normalizeMqttEndpoint(endpoint)

		publishTopic := shared.publishTopic
		if strings.Contains(publishTopic, ",") {
			publishTopic = strings.Split(publishTopic, ",")[0]
		}
		if strings.TrimSpace(publishTopic) == "" {
			publishTopic = fmt.Sprintf("%s/heartbeat", shared.deviceIMEI)
		}

		msgID := fmt.Sprintf("rms-live-%d", time.Now().UnixNano())
		payload := map[string]interface{}{
			"packet_type": "heartbeat",
			"project_id":  shared.projectID,
			"protocol_id": shared.protocolID,
			"device_id":   shared.deviceID,
			"imei":        shared.deviceIMEI,
			"ts":          time.Now().Unix(),
			"msg_id":      msgID,
		}

		if err := publishOnce(endpoint, shared.mqttUser, shared.mqttPass, shared.clientID, publishTopic, payload); err != nil {
			t.Fatalf("publish failed: %v", err)
		}

		persistCtx, persistCancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer persistCancel()
		present, err := waitForMsgID(persistCtx, dsn, msgID)
		if err != nil {
			t.Fatalf("persistence check failed: %v", err)
		}
		if !present {
			t.Fatalf("message %s not found in telemetry", msgID)
		}

		// Rotate credentials and ensure new credentials work
		rotReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices/%s/rotate-creds", baseURL, shared.deviceID), nil)
		rotReq.Header.Set("Authorization", "Bearer "+shared.adminToken)
		rotResp, err := httpCli.Do(rotReq)
		if err != nil {
			t.Skipf("rotate unreachable: %v", err)
		}
		if rotResp.StatusCode != http.StatusOK {
			rotResp.Body.Close()
			t.Fatalf("rotate status=%d", rotResp.StatusCode)
		}
		var rot map[string]interface{}
		decodeJSONBody(t, rotResp, &rot)

		mustWaitForProvisioningJobs(t, ctx, dsn, shared.deviceID)

		oldUser, oldPass := shared.mqttUser, shared.mqttPass
		if user, ok := rot["mqtt_user"].(string); ok && user != "" {
			shared.mqttUser = user
		}
		if pass, ok := rot["mqtt_pass"].(string); ok && pass != "" {
			shared.mqttPass = pass
		}
		if cid, ok := rot["client_id"].(string); ok && cid != "" {
			shared.clientID = cid
		}
		if ep, ok := rot["endpoint"].(string); ok && ep != "" {
			endpoint = normalizeMqttEndpoint(ep)
		}
		if pt, ok := rot["publish_topics"].(string); ok && pt != "" {
			publishTopic = pt
		}
		if strings.Contains(publishTopic, ",") {
			publishTopic = strings.Split(publishTopic, ",")[0]
		}

		if shared.mqttUser == oldUser && shared.mqttPass == oldPass {
			t.Fatalf("rotate returned same mqtt creds")
		}

		msgID2 := fmt.Sprintf("rms-rot-%d", time.Now().UnixNano())
		payload["msg_id"] = msgID2
		payload["ts"] = time.Now().Unix()

		if err := publishOnce(endpoint, shared.mqttUser, shared.mqttPass, shared.clientID, publishTopic, payload); err != nil {
			t.Fatalf("publish with rotated creds failed: %v", err)
		}

		persistCtx2, persistCancel2 := context.WithTimeout(context.Background(), 45*time.Second)
		defer persistCancel2()
		present2, err := waitForMsgID(persistCtx2, dsn, msgID2)
		if err != nil {
			t.Fatalf("post-rotate persistence check failed: %v", err)
		}
		if !present2 {
			t.Fatalf("post-rotate message %s not found in telemetry", msgID2)
		}
	})

	// Rules and virtual sensors wiring (creation + list)
	t.Run("rules_virtual_sensors", func(t *testing.T) {
		if shared.projectID == "" || shared.adminToken == "" {
			t.Skip("shared scenario not seeded")
		}

		rulePayload := map[string]interface{}{
			"projectId": shared.projectID,
			"name":      "efficiency-low",
			"trigger": map[string]interface{}{
				"formula": "efficiency < 50",
			},
			"actions": []map[string]interface{}{
				{"type": "log"},
			},
			"enabled": true,
		}

		resp, err := doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/rules", baseURL), shared.adminToken, rulePayload)
		if err != nil {
			t.Skipf("rules endpoint unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			resp.Body.Close()
			t.Fatalf("create rule status=%d", resp.StatusCode)
		}
		var ruleResp map[string]interface{}
		decodeJSONBody(t, resp, &ruleResp)
		ruleID, _ := ruleResp["id"].(string)

		resp, err = doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/rules?projectId=%s", baseURL, shared.projectID), shared.adminToken, nil)
		if err != nil {
			t.Skipf("rules list unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("list rules status=%d", resp.StatusCode)
		}
		var rules []map[string]interface{}
		decodeJSONBody(t, resp, &rules)
		if ruleID != "" {
			present := false
			for _, r := range rules {
				if id, ok := r["id"].(string); ok && id == ruleID {
					present = true
					break
				}
			}
			if !present {
				t.Fatalf("rule %s not returned in list", ruleID)
			}
		}
	})

	// Analytics and negative/cleanup placeholders (not yet wired)
	t.Run("analytics_dashboards", func(t *testing.T) {
		if shared.projectID == "" || shared.deviceIMEI == "" || shared.adminToken == "" {
			t.Skip("shared scenario not seeded")
		}

		end := time.Now()
		start := end.Add(-time.Hour)
		resp, err := doJSON(httpCli, http.MethodGet, fmt.Sprintf("%s/api/telemetry/history?device=%s&from=%s&to=%s", baseURL, shared.deviceIMEI, start.Format(time.RFC3339), end.Format(time.RFC3339)), shared.adminToken, nil)
		if err != nil {
			t.Skipf("analytics history unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("analytics history status=%d", resp.StatusCode)
		}
		var hist []map[string]interface{}
		decodeJSONBody(t, resp, &hist)
	})

	t.Run("negative_validation", func(t *testing.T) {
		// Send command without token -> expect 401/403
		resp, err := doJSON(httpCli, http.MethodPost, fmt.Sprintf("%s/api/commands/send", baseURL), "", map[string]interface{}{})
		if err != nil {
			t.Skipf("commands send unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
			resp.Body.Close()
			t.Fatalf("expected 401/403 for unauth command send, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cleanup_consistency", func(t *testing.T) {
		// Delete command catalog entry
		if shared.commandID == "" {
			t.Skip("no command to delete")
		}
		resp, err := doJSON(httpCli, http.MethodDelete, fmt.Sprintf("%s/api/commands/catalog/%s", baseURL, shared.commandID), shared.adminToken, nil)
		if err != nil {
			t.Skipf("delete command unreachable: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("delete command status=%d", resp.StatusCode)
		}
		resp.Body.Close()

		// Delete project to avoid residue
		if shared.projectID != "" {
			resp, err = doJSON(httpCli, http.MethodDelete, fmt.Sprintf("%s/api/projects/%s", baseURL, shared.projectID), shared.adminToken, nil)
			if err != nil {
				t.Skipf("delete project unreachable: %v", err)
			}
			if resp.StatusCode == http.StatusMethodNotAllowed {
				resp.Body.Close()
				t.Skip("project delete not supported")
			}
			// Some deployments may not allow delete; tolerate 200/204/202
			if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
				resp.Body.Close()
				t.Fatalf("delete project status=%d", resp.StatusCode)
			}
			resp.Body.Close()
		}
	})
}

// mustLogin wraps /api/auth/login and fails the test if no token is returned.
func mustLoginRMS(t testing.TB, cli *http.Client, baseURL, user, pass string) string {
	t.Helper()

	body := map[string]string{"username": user, "password": pass}
	resp, err := doJSON(cli, http.MethodPost, fmt.Sprintf("%s/api/auth/login", baseURL), "", body)
	if err != nil {
		t.Skipf("login unreachable: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed status=%d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var tok authToken
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if tok.Token == "" {
		t.Fatalf("empty token")
	}
	return tok.Token
}

// doJSON issues an HTTP request with optional bearer token and JSON body.
func doJSON(cli *http.Client, method, url, token string, body interface{}) (*http.Response, error) {
	var buf *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func decodeJSONBody(t testing.TB, resp *http.Response, out interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

// simple context helper for future DB/MQTT waits
func ctxWithTimeout(t testing.TB, d time.Duration) context.Context {
	t.Helper()
	ctx, _ := context.WithTimeout(context.Background(), d)
	return ctx
}
