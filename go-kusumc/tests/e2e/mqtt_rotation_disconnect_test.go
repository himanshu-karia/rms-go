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
)

// TestMQTTRotationForcesDisconnect verifies that rotating device credentials
// invalidates an active MQTT session, forcing device re-bootstrap/reconnect with new creds.
func TestMQTTRotationForcesDisconnect(t *testing.T) {
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
	token := mustLoginForRotation(t, httpCli, baseURL)

	imei := randomIMEI()
	deviceID := mustCreateRotationDevice(t, httpCli, baseURL, token, projectID, imei)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	mustWaitForProvisioningJobs(t, ctx, dsn, deviceID)

	bootBefore, err := fetchBootstrap(t, bootURL, imei, token)
	if err != nil {
		t.Fatalf("bootstrap before rotate failed: %v", err)
	}
	pbOld := bootBefore.PrimaryBroker
	if len(pbOld.Endpoints) == 0 || len(pbOld.PublishTopics) == 0 {
		t.Fatalf("bootstrap before rotate missing mqtt endpoint/topic")
	}

	lost := make(chan error, 1)
	clientOld := mustConnectPersistentMQTTClient(t, normalizeMqttEndpoint(pbOld.Endpoints[0]), fmt.Sprintf("rotate-live-%d", time.Now().UnixNano()), pbOld.Username, pbOld.Password, lost)
	defer clientOld.Disconnect(200)

	preRotatePayload := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  bootBefore.Context.Project.ID,
		"protocol_id": pbOld.ProtocolID,
		"device_id":   bootBefore.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msgid":       fmt.Sprintf("disconnect-pre-%d", time.Now().UnixNano()),
	}
	mustPublishWithClient(t, clientOld, pbOld.PublishTopics[0], preRotatePayload)

	mustRotateCreds(t, httpCli, baseURL, token, deviceID)
	mustWaitForProvisioningJobs(t, ctx, dsn, deviceID)

	dropped := false
	select {
	case <-time.After(12 * time.Second):
		dropped = !clientOld.IsConnected()
	case <-lost:
		dropped = true
	}

	postRotatePayload := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  bootBefore.Context.Project.ID,
		"protocol_id": pbOld.ProtocolID,
		"device_id":   bootBefore.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msgid":       fmt.Sprintf("disconnect-post-%d", time.Now().UnixNano()),
	}

	publishErr := publishWithClient(clientOld, pbOld.PublishTopics[0], postRotatePayload)
	if !dropped && publishErr == nil {
		t.Fatalf("old MQTT session still active after credential rotation; expected disconnect or publish rejection")
	}

	bootAfter, err := fetchBootstrapUntilChanged(t, ctx, bootURL, imei, pbOld.Username, pbOld.Password, token)
	if err != nil {
		t.Fatalf("bootstrap after rotate failed: %v", err)
	}
	pbNew := bootAfter.PrimaryBroker
	if pbNew.Username == "" || pbNew.Password == "" {
		t.Fatalf("bootstrap after rotate returned empty credentials")
	}
	if pbNew.Username == pbOld.Username && pbNew.Password == pbOld.Password {
		t.Fatalf("credentials did not change after rotation")
	}

	if err := publishOnce(normalizeMqttEndpoint(pbNew.Endpoints[0]), pbNew.Username, pbNew.Password, pbNew.ClientID, pbNew.PublishTopics[0], map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  bootAfter.Context.Project.ID,
		"protocol_id": pbNew.ProtocolID,
		"device_id":   bootAfter.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msgid":       fmt.Sprintf("disconnect-new-%d", time.Now().UnixNano()),
	}); err != nil {
		t.Fatalf("publish with new credentials failed: %v", err)
	}
}

func mustLoginForRotation(t testing.TB, httpCli *http.Client, baseURL string) string {
	t.Helper()
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
	return loginRes.Token
}

func mustCreateRotationDevice(t testing.TB, httpCli *http.Client, baseURL, token, projectID, imei string) string {
	t.Helper()
	body := map[string]interface{}{
		"name":        "e2e-rotation-disconnect",
		"imei":        imei,
		"projectId":   projectID,
		"protocol_id": "rms-v1",
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices", strings.TrimRight(baseURL, "/")), bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpCli.Do(req)
	if err != nil {
		t.Fatalf("create device request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create device failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var out struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode create device response: %v", err)
	}
	if out.DeviceID == "" {
		t.Fatalf("empty device_id in create response")
	}
	return out.DeviceID
}

func mustRotateCreds(t testing.TB, httpCli *http.Client, baseURL, token, deviceID string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices/%s/rotate-creds", strings.TrimRight(baseURL, "/"), deviceID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpCli.Do(req)
	if err != nil {
		t.Fatalf("rotate creds request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("rotate creds failed: status=%d body=%s", resp.StatusCode, string(body))
	}
}

func mustConnectPersistentMQTTClient(t testing.TB, endpoint, clientID, username, password string, lost chan<- error) mqtt.Client {
	t.Helper()
	opts := mqtt.NewClientOptions().AddBroker(endpoint)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetAutoReconnect(false)
	if strings.HasPrefix(endpoint, "mqtts://") || strings.HasPrefix(endpoint, "ssl://") || strings.HasPrefix(endpoint, "tls://") {
		tlsCfg, err := tlsConfigFromEnv()
		if err != nil {
			t.Fatalf("mqtt TLS config: %v", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		select {
		case lost <- err:
		default:
		}
	})

	cli, err := connectMQTTWithRetry(opts, 30*time.Second)
	if err != nil {
		t.Fatalf("mqtt connect failed: %v", err)
	}
	return cli
}

func mustPublishWithClient(t testing.TB, cli mqtt.Client, topic string, payload map[string]interface{}) {
	t.Helper()
	if err := publishWithClient(cli, topic, payload); err != nil {
		t.Fatalf("mqtt publish failed: %v", err)
	}
}

func publishWithClient(cli mqtt.Client, topic string, payload map[string]interface{}) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tok := cli.Publish(topic, 1, false, buf)
	if !tok.WaitTimeout(6 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if tok.Error() != nil {
		return tok.Error()
	}
	return nil
}
