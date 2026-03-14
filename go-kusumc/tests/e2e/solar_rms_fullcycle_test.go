//go:build integration
// +build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSolarRMSFullCycle validates the RMS-style MQTT payload path:
// provision -> ACL-backed MQTT publish -> Timescale persistence.
func TestSolarRMSFullCycle(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	bootURL := getenv("BOOTSTRAP_URL", strings.TrimRight(baseURL, "/")+"/api/bootstrap")
	dsn := getenv("TIMESCALE_URI", "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable")
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")
	httpCli := httpClient(t)

	imei := randomIMEI()
	protocolID := "rms-v1" // matches refer-rms-deploy taxonomy

	// 0) Login with seeded admin user
	loginBody := map[string]string{"username": "Him", "password": "0554"}
	loginBuf, _ := json.Marshal(loginBody)
	loginResp, err := httpCli.Post(fmt.Sprintf("%s/api/auth/login", baseURL), "application/json", bytes.NewReader(loginBuf))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login failed: status=%d body=%s", loginResp.StatusCode, string(body))
	}
	var loginRes struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginRes); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginRes.Token == "" {
		t.Fatalf("empty token from login")
	}

	// 1) Provision device with RMS protocol
	reqBody := map[string]interface{}{
		"name":        "e2e-solar-rms-device",
		"imei":        imei,
		"projectId":   projectID,
		"protocol_id": protocolID,
	}

	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices", baseURL), bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginRes.Token)
	resp, err := httpCli.Do(req)
	if err != nil {
		t.Fatalf("provision request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("provision failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var provisioned struct {
		DeviceID string `json:"device_id"`
		IMEI     string `json:"imei"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&provisioned); err != nil {
		t.Fatalf("decode provision response: %v", err)
	}
	if provisioned.DeviceID == "" {
		t.Fatalf("empty device_id in response")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	mustWaitForProvisioningJobs(t, ctx, dsn, provisioned.DeviceID)

	boot, err := fetchBootstrap(t, bootURL, imei, loginRes.Token)
	if err != nil {
		t.Fatalf("bootstrap fetch failed: %v", err)
	}
	if len(boot.PrimaryBroker.Endpoints) == 0 {
		t.Fatalf("bootstrap missing endpoints")
	}
	mqttURL := normalizeMqttEndpoint(boot.PrimaryBroker.Endpoints[0])
	mqttUser := boot.PrimaryBroker.Username
	mqttPass := boot.PrimaryBroker.Password
	if mqttUser == "" || mqttPass == "" {
		t.Fatalf("bootstrap missing mqtt credentials")
	}

	// 2) MQTT publish RMS-style heartbeat on canonical ingest topic
	opts := mqtt.NewClientOptions().AddBroker(mqttURL).SetClientID("solar-rms-sim-" + imei)
	opts.SetUsername(mqttUser)
	opts.SetPassword(mqttPass)
	if strings.HasPrefix(mqttURL, "mqtts://") || strings.HasPrefix(mqttURL, "ssl://") || strings.HasPrefix(mqttURL, "tls://") {
		tlsCfg, err := tlsConfigFromEnv()
		if err != nil {
			t.Fatalf("mqtt TLS config: %v", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}
	client, err := connectMQTTWithRetry(opts, 30*time.Second)
	if err != nil {
		t.Fatalf("mqtt connect error: %v", err)
	}
	defer client.Disconnect(200)

	msgID := fmt.Sprintf("msg-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		// Envelope aligned to RMS JSON MDs and refer-rms-deploy
		"packet_type": "heartbeat",
		"project_id":  projectID,
		"protocol_id": protocolID,
		"device_id":   provisioned.DeviceID,
		"imei":        imei,
		"IMEI":        imei,
		"msg_id":      msgID,
		"msgid":       msgID,
		"timestamp":   time.Now().Unix(),

		// Common RMS fields (upper-case style)
		"ASN":        "AS-01",
		"RSSI":       -67,
		"BATT":       12.4,
		"TEMP":       31.2,
		"PUMP_ON":    1,
		"GPS":        "0,0",
		"HEART_BEAT": 1,
	}

	payloadBytes, _ := json.Marshal(payload)
	topic := ""
	if len(boot.PrimaryBroker.PublishTopics) > 0 {
		topic = boot.PrimaryBroker.PublishTopics[0]
	}
	if topic == "" {
		topic = fmt.Sprintf("%s/heartbeat", imei)
	}
	token := client.Publish(topic, 1, false, payloadBytes)
	token.Wait()
	if token.Error() != nil {
		t.Fatalf("mqtt publish error: %v", token.Error())
	}

	// 3) Verify persistence in Timescale/Postgres, ensuring RMS fields are present
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer verifyCancel()

	pool, err := pgxpool.New(verifyCtx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	time.Sleep(3 * time.Second)

	var count int
	// Prefer main telemetry table; fall back to telemetry_hyper (used by CopyFrom path)
	queryPrimary := `select count(*) from telemetry where project_id = $1 and device_id = $2 and data->>'packet_type' = 'heartbeat' and data ? 'RSSI' and data ? 'BATT'`
	if err := pool.QueryRow(verifyCtx, queryPrimary, projectID, provisioned.DeviceID).Scan(&count); err != nil {
		t.Fatalf("query telemetry: %v", err)
	}

	if count == 0 {
		var hyperCount int
		hyperQuery := `select count(*) from telemetry_hyper where project_id = $1 and device_uuid = $2 and data->>'packet_type' = 'heartbeat' and data ? 'RSSI' and data ? 'BATT'`
		if err := pool.QueryRow(verifyCtx, hyperQuery, projectID, provisioned.DeviceID).Scan(&hyperCount); err != nil {
			t.Fatalf("query telemetry_hyper: %v", err)
		}
		count = hyperCount
	}

	if count == 0 {
		t.Fatalf("no RMS telemetry rows found for project=%s device=%s", projectID, provisioned.DeviceID)
	}
}
