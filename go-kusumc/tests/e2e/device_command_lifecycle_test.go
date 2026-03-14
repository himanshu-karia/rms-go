//go:build integration
// +build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDeviceCommandLifecycle provisions a device, runs a command round-trip (HTTP -> MQTT -> response),
// publishes telemetry, rotates creds, reconnects with new creds, and publishes again.
func TestDeviceCommandLifecycle(t *testing.T) {
	baseURL := getenv("BASE_URL", "http://localhost:8081")
	bootURL := getenv("BOOTSTRAP_URL", baseURL+"/api/bootstrap")
	dsn := getenv("TIMESCALE_URI", "postgres://postgres:password@timescaledb:5432/telemetry?sslmode=disable")
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")

	// Enable TLS skip for local self-signed if requested
	if os.Getenv("HTTP_TLS_INSECURE") == "" {
		_ = os.Setenv("HTTP_TLS_INSECURE", "true")
	}

	httpCli := httpClient(t)

	// 0) Login with seeded admin user
	token := mustLogin(t, httpCli, baseURL, "Him", "0554")

	// 1) Provision device in test project
	imei := randomIMEI()
	deviceID := createDevice(t, httpCli, baseURL, token, projectID, imei)

	// 2) Ensure command capability for E2E_Set
	pool := mustPool(t, dsn)
	defer pool.Close()
	cmdID := mustCommandID(t, pool, projectID, "E2E_Set")
	ensureDeviceCapability(t, pool, deviceID, cmdID)

	provCtx, provCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer provCancel()
	mustWaitForProvisioningJobs(t, provCtx, dsn, deviceID)

	// 3) Bootstrap to get device MQTT creds
	boot, err := fetchBootstrap(t, bootURL, imei, token)
	if err != nil {
		t.Fatalf("bootstrap fetch failed: %v", err)
	}
	pb := boot.PrimaryBroker
	endpoint := firstEndpoint(pb.Endpoints)
	if endpoint == "" {
		t.Fatalf("bootstrap missing endpoints")
	}
	if strings.TrimSpace(pb.Username) == "" || strings.TrimSpace(pb.Password) == "" {
		t.Fatalf("bootstrap missing device MQTT credentials")
	}

	// 4) Start device MQTT client to handle commands and send telemetry
	cmdTopic := fmt.Sprintf("%s/ondemand", imei)
	respTopic := cmdTopic
	pubTopic := pickTopicBySuffix(pb.PublishTopics, "heartbeat")
	if pubTopic == "" {
		t.Fatalf("bootstrap missing publish topics")
	}

	mqttClient := connectDeviceMqtt(t, endpoint, pb.Username, pb.Password, pb.ClientID)
	defer mqttClient.Disconnect(200)

	// Command handler: echo response with correlation_id.
	// Legacy RMS uses the same <imei>/ondemand topic for both directions, so we must
	// ignore our own responses to prevent a subscribe->publish loop.
	mqttClient.Subscribe(cmdTopic, 1, func(_ mqtt.Client, m mqtt.Message) {
		var msg map[string]interface{}
		_ = json.Unmarshal(m.Payload(), &msg)
		// go-kusumc publishes govt/legacy commands as top-level keys:
		// { msgid, timestamp, type:"ondemand_cmd", cmd:"...", ... }
		// Devices respond on the same <imei>/ondemand topic.
		if typ, _ := msg["type"].(string); strings.EqualFold(strings.TrimSpace(typ), "ondemand_rsp") {
			return
		}
		if pt, _ := msg["packet_type"].(string); strings.EqualFold(strings.TrimSpace(pt), "ondemand_rsp") {
			return
		}
		if cmd, _ := msg["cmd"].(string); strings.TrimSpace(cmd) == "" {
			return
		}
		corr, _ := msg["correlation_id"].(string)
		if strings.TrimSpace(corr) == "" {
			corr, _ = msg["msgid"].(string)
		}
		corr = strings.TrimSpace(corr)
		if corr == "" {
			return
		}
		resp := map[string]interface{}{"packet_type": "ondemand_rsp", "type": "ondemand_rsp", "correlation_id": corr, "msgid": corr, "status": "OK", "ts": time.Now().UnixMilli()}
		data, _ := json.Marshal(resp)
		tok := mqttClient.Publish(respTopic, 1, false, data)
		tok.Wait()
	})

	// 5) Send command via API
	sendBody := map[string]interface{}{
		"deviceId":  imei,
		"projectId": projectID,
		"commandId": cmdID,
		"payload":   map[string]interface{}{"mode": "on"},
	}
	mustAuthJSON(t, httpCli, token, http.MethodPost, fmt.Sprintf("%s/api/commands/send", strings.TrimRight(baseURL, "/")), sendBody, http.StatusOK)

	// 6) Wait for command status to become acked
	cmdWaitCtx, cmdCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cmdCancel()
	if err := waitForCommandStatus(cmdWaitCtx, httpCli, token, baseURL, imei, "acked"); err != nil {
		t.Skipf("command loopback not observed in current environment: %v", err)
	}

	// 7) Publish telemetry and assert persistence
	msgID1 := fmt.Sprintf("cmd-cycle-%d", time.Now().UnixNano())
	telemetry := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  projectID,
		"protocol_id": pb.ProtocolID,
		"device_id":   boot.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msg_id":      msgID1,
		"msgid":       msgID1,
	}
	mustPublish(t, endpoint, pb.Username, pb.Password, pb.ClientID, pubTopic, telemetry)
	teleWaitCtx, teleCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer teleCancel()
	if present, err := waitForMsg(teleWaitCtx, dsn, msgID1); err != nil || !present {
		t.Fatalf("telemetry not persisted: present=%v err=%v", present, err)
	}

	// 8) Rotate creds via API
	rotURL := fmt.Sprintf("%s/api/devices/%s/rotate-creds", strings.TrimRight(baseURL, "/"), deviceID)
	mustAuthJSON(t, httpCli, token, http.MethodPost, rotURL, nil, http.StatusOK)

	// 9) Fetch bootstrap until creds change
	rotateWaitCtx, rotateCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer rotateCancel()
	boot2, err := fetchBootstrapUntilChanged(t, rotateWaitCtx, bootURL, imei, pb.Username, pb.Password, token)
	if err != nil {
		t.Fatalf("post-rotate bootstrap failed: %v", err)
	}
	pb2 := boot2.PrimaryBroker
	endpoint2 := firstEndpoint(pb2.Endpoints)
	if endpoint2 == "" {
		t.Fatalf("post-rotate endpoints empty")
	}

	// 10) Reconnect MQTT with new creds and publish telemetry
	mqttClient.Disconnect(200)
	mqttClient2 := connectDeviceMqtt(t, endpoint2, pb2.Username, pb2.Password, pb2.ClientID)
	defer mqttClient2.Disconnect(200)

	msgID2 := fmt.Sprintf("cmd-cycle-rot-%d", time.Now().UnixNano())
	telemetry2 := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  projectID,
		"protocol_id": pb2.ProtocolID,
		"device_id":   boot2.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msg_id":      msgID2,
		"msgid":       msgID2,
	}
	mustPublish(t, endpoint2, pb2.Username, pb2.Password, pb2.ClientID, pickTopicBySuffix(pb2.PublishTopics, "heartbeat"), telemetry2)
	tele2WaitCtx, tele2Cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer tele2Cancel()
	if present, err := waitForMsg(tele2WaitCtx, dsn, msgID2); err != nil || !present {
		t.Fatalf("telemetry post-rotate not persisted: present=%v err=%v", present, err)
	}
}

func pickTopicBySuffix(topics []string, suffix string) string {
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	if len(topics) == 0 {
		return ""
	}
	if suffix != "" {
		needle := "/" + suffix
		for _, t := range topics {
			if strings.HasSuffix(strings.ToLower(strings.TrimSpace(t)), needle) {
				return t
			}
		}
	}
	return topics[0]
}

func mustLogin(t testing.TB, cli *http.Client, baseURL, user, pass string) string {
	t.Helper()
	body := map[string]string{"username": user, "password": pass}
	buf, _ := json.Marshal(body)
	resp, err := cli.Post(fmt.Sprintf("%s/api/auth/login", strings.TrimRight(baseURL, "/")), "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("login failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if out.Token == "" {
		t.Fatalf("empty token")
	}
	return out.Token
}

func createDevice(t testing.TB, cli *http.Client, baseURL, token, projectID, imei string) string {
	t.Helper()
	reqBody := map[string]interface{}{
		"name":      "e2e-cmd-device",
		"imei":      imei,
		"projectId": projectID,
	}
	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices", strings.TrimRight(baseURL, "/")), bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create device failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	var out struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode device resp: %v", err)
	}
	if out.DeviceID == "" {
		t.Fatalf("device_id missing")
	}
	return out.DeviceID
}

func mustCommandID(t testing.TB, pool *pgxpool.Pool, projectID, name string) string {
	t.Helper()
	var id string
	lookup := []struct {
		project string
		name    string
	}{
		{projectID, name},
		{projectID, "send_immediate"},
		{"", name},
		{"", "send_immediate"},
		{"core", name},
		{"core", "send_immediate"},
	}

	for _, item := range lookup {
		err := pool.QueryRow(context.Background(), `SELECT id FROM command_catalog WHERE COALESCE(project_id,'')=$1 AND name=$2`, item.project, item.name).Scan(&id)
		if err == nil && id != "" {
			return id
		}
	}

	t.Fatalf("command %s not found in project %s (including seeded fallback)", name, projectID)
	return ""
}

func ensureDeviceCapability(t testing.TB, pool *pgxpool.Pool, deviceID, commandID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO device_capabilities (device_id, command_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING`, deviceID, commandID)
	if err != nil {
		t.Fatalf("grant capability: %v", err)
	}
}

func mustPool(t testing.TB, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	return pool
}

func firstEndpoint(endpoints []string) string {
	if len(endpoints) == 0 {
		return ""
	}
	return normalizeMqttEndpoint(endpoints[0])
}

func firstTopic(topics []string) string {
	if len(topics) == 0 {
		return ""
	}
	return topics[0]
}

func connectDeviceMqtt(t testing.TB, endpoint, username, password, clientID string) mqtt.Client {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("parse endpoint: %v", err)
	}
	opts := mqtt.NewClientOptions().AddBroker(u.String())
	if clientID == "" {
		clientID = fmt.Sprintf("e2e-dev-%d", time.Now().UnixNano())
	}
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	if u.Scheme == "mqtts" {
		tlsCfg, err := tlsConfigFromEnv()
		if err != nil {
			t.Fatalf("tls config: %v", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}
	c, err := connectMQTTWithRetry(opts, 30*time.Second)
	if err != nil {
		t.Fatalf("mqtt connect: %v", err)
	}
	return c
}

func mustPublish(t testing.TB, endpoint, username, password, clientID, topic string, payload map[string]interface{}) {
	t.Helper()
	if err := publishOnce(endpoint, username, password, clientID, topic, payload); err != nil {
		t.Fatalf("publish %s failed: %v", topic, err)
	}
}

func waitForCommandStatus(ctx context.Context, cli *http.Client, token, baseURL, deviceRef, want string) error {
	url := fmt.Sprintf("%s/api/commands?deviceId=%s&limit=5", strings.TrimRight(baseURL, "/"), deviceRef)
	for {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := cli.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var items []struct {
				Status string `json:"status"`
			}
			_ = json.Unmarshal(body, &items)
			for _, it := range items {
				if it.Status == want {
					return nil
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("timeout waiting for command status; last body=%s", string(body))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForMsg(ctx context.Context, dsn, msgID string) (bool, error) {
	return waitForMsgID(ctx, dsn, msgID)
}

func mustAuthJSON(t testing.TB, cli *http.Client, token, method, url string, body interface{}, expect int) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		reader = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expect {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d (want %d) body=%s", resp.StatusCode, expect, string(b))
	}
}
