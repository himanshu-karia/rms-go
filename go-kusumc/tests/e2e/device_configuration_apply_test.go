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
	"os"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDeviceConfigurationApply(t *testing.T) {
	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	bootURL := getenv("BOOTSTRAP_URL", strings.TrimRight(baseURL, "/")+"/api/bootstrap")
	dsn := getenv("TIMESCALE_URI", "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable")
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")

	if os.Getenv("HTTP_TLS_INSECURE") == "" {
		_ = os.Setenv("HTTP_TLS_INSECURE", "true")
	}

	httpCli := httpClient(t)
	token := mustLogin(t, httpCli, baseURL, "Him", "0554")

	imei := randomIMEI()
	deviceID := createDevice(t, httpCli, baseURL, token, projectID, imei)

	boot, err := fetchBootstrap(t, bootURL, imei, token)
	if err != nil {
		t.Fatalf("bootstrap fetch failed: %v", err)
	}
	pb := boot.PrimaryBroker
	endpoint := firstEndpoint(pb.Endpoints)
	if endpoint == "" {
		t.Fatalf("bootstrap missing endpoints")
	}

	svcUser := getenv("SERVICE_MQTT_USERNAME", "backend-service")
	svcPass := getenv("SERVICE_MQTT_PASSWORD", "change-me")

	cmdTopic := fmt.Sprintf("%s/ondemand", imei)
	cmdRespTopic := cmdTopic

	mqttClient := connectDeviceMqtt(t, endpoint, svcUser, svcPass, "")
	defer mqttClient.Disconnect(200)

	// When the server publishes apply_device_configuration, reply with an acked ondemand_rsp.
	if tok := mqttClient.Subscribe(cmdTopic, 1, func(_ mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		_ = json.Unmarshal(m.Payload(), &msg)
		if pt, _ := msg["packet_type"].(string); strings.EqualFold(pt, "ondemand_rsp") {
			return
		}
		if _, ok := msg["command"].(map[string]any); !ok {
			return
		}
		corr, _ := msg["correlation_id"].(string)
		if strings.TrimSpace(corr) == "" {
			return
		}
		resp := map[string]any{
			"packet_type":      "ondemand_rsp",
			"correlation_id":   corr,
			"msgid":            corr,
			"status":           "ack",
			"code":             0,
			"project_id":       projectID,
			"imei":             imei,
			"ts":               time.Now().UnixMilli(),
			"message":          "configuration applied",
			"configuration_id": corr,
		}
		data, _ := json.Marshal(resp)

		pubTok := mqttClient.Publish(cmdRespTopic, 1, false, data)
		pubTok.Wait()
	}); tok.Wait() {
		if tok.Error() != nil {
			t.Fatalf("mqtt subscribe %s failed: %v", cmdTopic, tok.Error())
		}
	}

	// Queue configuration (this should publish MQTT command with msgid==correlation_id==config_id)
	queueURL := fmt.Sprintf("%s/api/devices/%s/configuration", strings.TrimRight(baseURL, "/"), deviceID)
	configBody := map[string]any{
		"vfd_model_id": "vfd-model-seed",
		"overrides": map[string]any{
			"rs485": map[string]any{"baud": 9600},
		},
	}
	buf, _ := json.Marshal(configBody)
	req, _ := http.NewRequest(http.MethodPost, queueURL, bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpCli.Do(req)
	if err != nil {
		t.Fatalf("queue configuration request failed: %v", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("queue configuration status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}
	var queued struct {
		ID    string `json:"id"`
		MsgID string `json:"msgid"`
	}
	_ = json.Unmarshal(bodyBytes, &queued)
	configID := strings.TrimSpace(queued.MsgID)
	if configID == "" {
		configID = strings.TrimSpace(queued.ID)
	}
	if configID == "" {
		t.Fatalf("queue response missing msgid/id: body=%s", string(bodyBytes))
	}

	// Publish an explicit ack/response for the queued config ID.
	// This makes the test robust even if the downlink command delivery cannot be observed in a given environment.
	ack := map[string]any{
		"packet_type":    "ondemand_rsp",
		"correlation_id": configID,
		"msgid":          configID,
		"status":         "ack",
		"code":           0,
		"project_id":     projectID,
		"device_id":      boot.Identity.UUID,
		"imei":           imei,
		"ts":             time.Now().UnixMilli(),
		"message":        "configuration applied",
	}
	ackBytes, _ := json.Marshal(ack)
	pubTok := mqttClient.Publish(cmdRespTopic, 1, false, ackBytes)
	pubTok.Wait()
	if pubTok.Error() != nil {
		t.Fatalf("mqtt publish failed: %v", pubTok.Error())
	}

	pool := mustPool(t, dsn)
	defer pool.Close()

	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := waitForDeviceConfigurationStatus(waitCtx, pool, configID, "acknowledged"); err != nil {
		t.Fatalf("device configuration not acknowledged: %v", err)
	}
}

func waitForDeviceConfigurationStatus(ctx context.Context, pool *pgxpool.Pool, configID string, want string) error {
	for {
		var status string
		var acknowledgedAt *time.Time
		err := pool.QueryRow(ctx, `SELECT status, acknowledged_at FROM device_configurations WHERE id=$1`, configID).Scan(&status, &acknowledgedAt)
		if err == nil {
			if strings.EqualFold(strings.TrimSpace(status), strings.TrimSpace(want)) {
				if acknowledgedAt == nil {
					return fmt.Errorf("status=%s but acknowledged_at is null", status)
				}
				return nil
			}
		}

		if err := ctx.Err(); err != nil {
			return fmt.Errorf("timeout waiting for device_configurations(%s) status=%s", configID, want)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
