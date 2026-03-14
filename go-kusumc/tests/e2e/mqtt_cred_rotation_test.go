//go:build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMQTTCredRotation validates that rotating device MQTT credentials causes old creds to be rejected,
// and new creds can publish and persist telemetry.
func TestMQTTCredRotation(t *testing.T) {
	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	dsn := getenv("TIMESCALE_URI", "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable")
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")
	bootURL := os.Getenv("BOOTSTRAP_URL")
	if bootURL == "" {
		bootURL = strings.TrimRight(baseURL, "/") + "/api/bootstrap"
	}
	if os.Getenv("MQTT_TLS_INSECURE") == "" && os.Getenv("MQTT_CA_PATH") == "" {
		_ = os.Setenv("MQTT_TLS_INSECURE", "true")
	}

	httpCli := httpClient(t)

	// 0) Login with seeded admin user
	loginBody := map[string]string{"username": "Him", "password": "0554"}
	loginBuf, _ := json.Marshal(loginBody)
	loginResp, err := httpCli.Post(fmt.Sprintf("%s/api/auth/login", strings.TrimRight(baseURL, "/")), "application/json", bytes.NewReader(loginBuf))
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

	// 1) Provision device
	imei := randomIMEI()
	protocolID := "rms-v1"

	reqBody := map[string]interface{}{
		"name":        "e2e-rotate-creds",
		"imei":        imei,
		"projectId":   projectID,
		"protocol_id": protocolID,
	}

	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices", strings.TrimRight(baseURL, "/")), bytes.NewReader(buf))
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

	// 2) Fetch bootstrap creds and publish once (expect success)
	boot1, err := fetchBootstrap(t, bootURL, imei, loginRes.Token)
	if err != nil {
		t.Fatalf("bootstrap fetch failed: %v", err)
	}
	pb1 := boot1.PrimaryBroker
	if len(pb1.Endpoints) == 0 || len(pb1.PublishTopics) == 0 {
		t.Fatalf("bootstrap missing endpoints/topics")
	}
	broker1 := normalizeMqttEndpoint(pb1.Endpoints[0])

	msgID1 := fmt.Sprintf("rotate-pre-%d", time.Now().UnixNano())
	payload1 := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  boot1.Context.Project.ID,
		"protocol_id": pb1.ProtocolID,
		"device_id":   boot1.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msg_id":      msgID1,
		"msgid":       msgID1,
	}

	if err := publishOnce(broker1, pb1.Username, pb1.Password, pb1.ClientID, pb1.PublishTopics[0], payload1); err != nil {
		t.Fatalf("publish with initial creds failed: broker=%s user=%s err=%v", broker1, pb1.Username, err)
	}

	// 3) Rotate creds
	rotReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices/%s/rotate-creds", strings.TrimRight(baseURL, "/"), provisioned.DeviceID), nil)
	rotReq.Header.Set("Authorization", "Bearer "+loginRes.Token)

	rotResp, err := httpCli.Do(rotReq)
	if err != nil {
		t.Fatalf("rotate request failed: %v", err)
	}
	defer rotResp.Body.Close()
	if rotResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(rotResp.Body)
		t.Fatalf("rotate failed: status=%d body=%s", rotResp.StatusCode, string(body))
	}

	mustWaitForProvisioningJobs(t, ctx, dsn, provisioned.DeviceID)

	// 4) Fetch bootstrap again and ensure creds changed (poll briefly to avoid propagation race)
	boot2, err := fetchBootstrapUntilChanged(t, ctx, bootURL, imei, pb1.Username, pb1.Password, loginRes.Token)
	if err != nil {
		t.Fatalf("bootstrap fetch (post-rotate) failed: %v", err)
	}
	pb2 := boot2.PrimaryBroker
	if pb2.Username == "" || pb2.Password == "" {
		t.Fatalf("bootstrap returned empty creds post-rotate")
	}
	if pb1.Username == pb2.Username && pb1.Password == pb2.Password {
		t.Fatalf("expected bootstrap creds to change after rotate, but they did not")
	}

	// 5) Publish with old creds should fail (connect or publish)
	expectPublishFail(t, broker1, pb1.Username, pb1.Password, pb1.ClientID, pb1.PublishTopics[0], payload1)

	// 6) Publish with new creds should succeed and persist
	msgID2 := fmt.Sprintf("rotate-post-%d", time.Now().UnixNano())
	payload2 := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  boot2.Context.Project.ID,
		"protocol_id": pb2.ProtocolID,
		"device_id":   boot2.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msg_id":      msgID2,
		"msgid":       msgID2,
	}

	if len(pb2.Endpoints) == 0 || len(pb2.PublishTopics) == 0 {
		t.Fatalf("bootstrap missing endpoints/topics post-rotate")
	}
	broker2 := normalizeMqttEndpoint(pb2.Endpoints[0])
	if err := publishOnce(broker2, pb2.Username, pb2.Password, pb2.ClientID, pb2.PublishTopics[0], payload2); err != nil {
		t.Fatalf("publish with new creds failed: broker=%s user=%s err=%v", broker2, pb2.Username, err)
	}

	persistCtx, persistCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer persistCancel()

	present, err := waitForMsgID(persistCtx, dsn, msgID2)
	if err != nil {
		t.Fatalf("persistence check failed: %v", err)
	}
	if !present {
		t.Fatalf("message with msg_id %s not found in telemetry", msgID2)
	}
}

func mustWaitForProvisioningJobs(t testing.TB, ctx context.Context, dbURI, deviceID string) {
	t.Helper()

	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatalf("connect postgres (wait provisioning): %v", err)
	}
	defer pool.Close()

	deadline := time.Now().Add(60 * time.Second)
	for {
		var pending int
		err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM mqtt_provisioning_jobs WHERE device_id=$1 AND status IN ('pending','processing')`, deviceID).Scan(&pending)
		if err != nil {
			t.Fatalf("query provisioning jobs: %v", err)
		}
		if pending == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("provisioning jobs still pending for device %s", deviceID)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func fetchBootstrapUntilChanged(t testing.TB, ctx context.Context, bootURL, imei, oldUser, oldPass, bearerToken string) (*bootstrapResponse, error) {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		b, err := fetchBootstrap(t, bootURL, imei, bearerToken)
		if err != nil {
			return nil, err
		}
		pb := b.PrimaryBroker
		if pb.Username != "" && pb.Password != "" {
			if pb.Username != oldUser || pb.Password != oldPass {
				return b, nil
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return b, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func expectPublishFail(t testing.TB, endpoint, username, password, clientID, topic string, payload map[string]interface{}) {
	t.Helper()

	opts := mqtt.NewClientOptions().AddBroker(endpoint)
	if clientID == "" {
		clientID = fmt.Sprintf("e2e-old-%d", time.Now().UnixNano())
	}
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	if strings.HasPrefix(endpoint, "mqtts://") || strings.HasPrefix(endpoint, "ssl://") || strings.HasPrefix(endpoint, "tls://") {
		tlsCfg, err := tlsConfigFromEnv()
		if err != nil {
			t.Fatalf("mqtt TLS config: %v", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}
	opts.SetConnectTimeout(4 * time.Second)
	opts.SetCleanSession(true)

	cli := mqtt.NewClient(opts)
	tok := cli.Connect()
	if tok.WaitTimeout(5*time.Second) && tok.Error() == nil {
		// Connection unexpectedly succeeded; publishing should still be rejected.
		defer cli.Disconnect(200)
		buf, _ := json.Marshal(payload)
		pub := cli.Publish(topic, 1, false, buf)
		if pub.WaitTimeout(5*time.Second) && pub.Error() == nil {
			t.Fatalf("expected old credentials to fail, but publish succeeded")
		}
		return
	}
}

func normalizeMqttEndpoint(endpoint string) string {
	// Bootstrap may return Paho's "tcp://" even when pointing at the TLS listener (8883).
	// Map tcp://<host>:8883 -> mqtts://<host>:8883 so publishOnce applies TLS.
	if strings.HasPrefix(endpoint, "tcp://") && strings.Contains(endpoint, ":8883") {
		return "mqtts://" + strings.TrimPrefix(endpoint, "tcp://")
	}
	return endpoint
}
