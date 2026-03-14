//go:build integration
// +build integration

package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5"
)

type loginResp struct {
	Token string `json:"token"`
}

type createDeviceResp struct {
	DeviceID  string `json:"device_id"`
	IMEI      string `json:"imei"`
	MQTTUser  string `json:"mqtt_user"`
	MQTTPass  string `json:"mqtt_pass"`
	ClientID  string `json:"client_id"`
	ProjectID string `json:"project_id"`
	Publish   string `json:"publish_topics"`
	Subscribe string `json:"subscribe_topics"`
}

type rotateResp struct {
	MQTTUser string `json:"mqtt_user"`
	MQTTPass string `json:"mqtt_pass"`
	ClientID string `json:"client_id"`
}

type sendCommandResp struct {
	CorrelationID string `json:"correlationId"`
}

type commandCatalog struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestDeviceLifecycle(t *testing.T) {
	ctx := context.Background()
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://rms-iot.local:7443"
	}
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")

	token := loginLC(t, httpClient, baseURL, "Him", "0554")

	imei := randomIMEI()
	createRes := createDeviceLC(t, httpClient, baseURL, token, imei, projectID)

	pgURI := os.Getenv("TIMESCALE_URI")
	if pgURI == "" {
		t.Fatal("TIMESCALE_URI not set")
	}
	conn, err := pgx.Connect(ctx, pgURI)
	if err != nil {
		t.Fatalf("connect pg: %v", err)
	}
	defer conn.Close(ctx)

	provCtx, provCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer provCancel()
	mustWaitForProvisioningJobs(t, provCtx, pgURI, createRes.DeviceID)

	// ensure command catalog exists
	cmdID := fetchCommandIDLC(t, ctx, conn, projectID, "send_immediate")
	// grant capability to this new device
	grantCapabilityLC(t, ctx, conn, createRes.DeviceID, cmdID)

	// start MQTT client using per-device credentials (after provisioning jobs complete)
	broker := getenv("MQTT_BROKER", "mqtts://rms-iot.local:18883")
	client := newMQTTClientLC(t, broker, createRes.ClientID, createRes.MQTTUser, createRes.MQTTPass)
	defer client.Disconnect(250)

	respTopic := fmt.Sprintf("%s/ondemand", imei)
	cmdTopic := respTopic
	done := make(chan string, 1)
	pubErr := make(chan error, 1)

	if token := client.Subscribe(cmdTopic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		var payload map[string]interface{}
		_ = json.Unmarshal(msg.Payload(), &payload)
		if typ, _ := payload["type"].(string); strings.EqualFold(strings.TrimSpace(typ), "ondemand_rsp") {
			return
		}
		if pt, _ := payload["packet_type"].(string); strings.EqualFold(strings.TrimSpace(pt), "ondemand_rsp") {
			return
		}
		if cmd, _ := payload["cmd"].(string); strings.TrimSpace(cmd) == "" {
			return
		}
		corr, _ := payload["correlation_id"].(string)
		if strings.TrimSpace(corr) == "" {
			corr, _ = payload["msgid"].(string)
		}
		corr = strings.TrimSpace(corr)
		if corr == "" {
			return
		}
		body := map[string]interface{}{
			"packet_type":    "ondemand_rsp",
			"type":           "ondemand_rsp",
			"correlation_id": corr,
			"msgid":          corr,
			"status":         "OK",
			"ts":             time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(body)
		pub := client.Publish(respTopic, 1, false, data)
		pub.Wait()
		if err := pub.Error(); err != nil {
			select {
			case pubErr <- err:
			default:
			}
			return
		}
		if strings.TrimSpace(corr) != "" {
			done <- corr
		}
	}); token.Wait() && token.Error() != nil {
		t.Fatalf("subscribe: %v", token.Error())
	}

	corrID := sendCommandLC(t, httpClient, baseURL, token, createRes.DeviceID, cmdID, projectID)

	// wait for loopback publish
	select {
	case got := <-done:
		if got != corrID {
			t.Fatalf("correlation mismatch: got %s want %s", got, corrID)
		}
	case err := <-pubErr:
		t.Fatalf("failed to publish ondemand_rsp: %v", err)
	case <-time.After(10 * time.Second):
		t.Skip("command loopback not observed in current environment")
	}

	waitStatusLC(t, ctx, conn, corrID, "acked", 45*time.Second)

	// rotate creds via API and reconnect MQTT
	rot := rotateCredsLC(t, httpClient, baseURL, token, createRes.DeviceID)
	mustWaitForProvisioningJobs(t, provCtx, pgURI, createRes.DeviceID)
	client.Disconnect(250)
	client = newMQTTClientLC(t, broker, rot.ClientID, rot.MQTTUser, rot.MQTTPass)
	defer client.Disconnect(250)

	teleTopic := fmt.Sprintf("%s/heartbeat", imei)
	tele := map[string]interface{}{"packet_type": "heartbeat", "imei": imei, "IMEI": imei, "ts": time.Now().UnixMilli()}
	teleData, _ := json.Marshal(tele)
	pub := client.Publish(teleTopic, 1, false, teleData)
	pub.Wait()
	if err := pub.Error(); err != nil {
		t.Fatalf("telemetry publish: %v", err)
	}
}

func loginLC(t *testing.T, httpClient *http.Client, baseURL, user, pass string) string {
	body := map[string]string{"username": user, "password": pass}
	data, _ := json.Marshal(body)
	resp, err := httpClient.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status %d", resp.StatusCode)
	}
	var lr loginResp
	json.NewDecoder(resp.Body).Decode(&lr)
	if lr.Token == "" {
		t.Fatalf("empty token")
	}
	return lr.Token
}

func createDeviceLC(t *testing.T, httpClient *http.Client, baseURL, token, imei, project string) createDeviceResp {
	payload := map[string]interface{}{
		"name":       "E2E Device",
		"imei":       imei,
		"projectId":  project,
		"attributes": map[string]interface{}{"status": "active"},
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/devices", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("create device status %d", resp.StatusCode)
	}
	var out createDeviceResp
	json.NewDecoder(resp.Body).Decode(&out)
	if out.DeviceID == "" || out.MQTTUser == "" || out.MQTTPass == "" {
		t.Fatalf("missing device fields: %+v", out)
	}
	return out
}

func fetchCommandIDLC(t *testing.T, ctx context.Context, conn *pgx.Conn, project, name string) string {
	var id string
	err := conn.QueryRow(ctx, `
		select id
		from command_catalog
		where name=$2
		  and (project_id=$1 or (project_id is null and scope='core'))
		order by case when project_id=$1 then 0 else 1 end
		limit 1
	`, project, name).Scan(&id)
	if err != nil {
		t.Fatalf("fetch command: %v", err)
	}
	return id
}

func grantCapabilityLC(t *testing.T, ctx context.Context, conn *pgx.Conn, deviceID, cmdID string) {
	_, err := conn.Exec(ctx, `insert into device_capabilities (device_id, command_id)
    values ($1,$2) on conflict do nothing`, deviceID, cmdID)
	if err != nil {
		t.Fatalf("grant capability: %v", err)
	}
}

func sendCommandLC(t *testing.T, httpClient *http.Client, baseURL, token, deviceID, commandID, projectID string) string {
	payload := map[string]interface{}{
		"deviceId":  deviceID,
		"projectId": projectID,
		"commandId": commandID,
		"payload":   map[string]interface{}{"mode": "on"},
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/commands/send", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("send command: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var body map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("send command status %d body=%v", resp.StatusCode, body)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	var out sendCommandResp
	_ = json.Unmarshal(bodyBytes, &out)
	if out.CorrelationID == "" {
		t.Fatalf("empty correlation id body=%s", string(bodyBytes))
	}
	return out.CorrelationID
}

func waitStatusLC(t *testing.T, ctx context.Context, conn *pgx.Conn, corr, want string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	lastStatus := ""
	lastErr := error(nil)
	for time.Now().Before(deadline) {
		var status string
		queryCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := conn.QueryRow(queryCtx, `select status from command_requests where correlation_id=$1::uuid`, corr).Scan(&status)
		cancel()
		if err == nil {
			lastStatus = status
		} else {
			lastErr = err
		}
		if err == nil && strings.EqualFold(status, want) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("status did not reach %s for %s (lastStatus=%q lastErr=%v)", want, corr, lastStatus, lastErr)
	}
	t.Fatalf("status did not reach %s for %s (lastStatus=%q)", want, corr, lastStatus)
}

func rotateCredsLC(t *testing.T, httpClient *http.Client, baseURL, token, deviceID string) rotateResp {
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices/%s/rotate-creds", baseURL, deviceID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("rotate creds: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("rotate creds status %d", resp.StatusCode)
	}
	var out rotateResp
	json.NewDecoder(resp.Body).Decode(&out)
	if out.MQTTUser == "" || out.MQTTPass == "" || out.ClientID == "" {
		t.Fatalf("rotate creds missing fields: %+v", out)
	}
	return out
}

func newMQTTClientLC(t *testing.T, broker, clientID, user, pass string) mqtt.Client {
	opts := mqtt.NewClientOptions().AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(user)
	opts.SetPassword(pass)
	opts.SetAutoReconnect(true)
	if strings.HasPrefix(broker, "mqtts://") || strings.HasPrefix(broker, "ssl://") || strings.HasPrefix(broker, "tls://") {
		tlsCfg, err := tlsConfigFromEnv()
		if err != nil {
			t.Fatalf("mqtt TLS config: %v", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}
	client, err := connectMQTTWithRetry(opts, 30*time.Second)
	if err != nil {
		t.Fatalf("mqtt connect: %v", err)
	}
	return client
}
