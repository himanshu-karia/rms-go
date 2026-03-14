//go:build integration

package e2e

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	bootstrapAPIKeyOnce sync.Once
	bootstrapAPIKey     string
)

// This test exercises the full device flow: bootstrap -> MQTT connect -> publish -> Timescale persistence check.
func TestBootstrapConnectPersist(t *testing.T) {
	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	bootURL := getenv("BOOTSTRAP_URL", strings.TrimRight(baseURL, "/")+"/api/bootstrap")
	imei := getenv("BOOTSTRAP_IMEI", randomIMEI())
	dbURI := getenv("TIMESCALE_URI", "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable")
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")

	httpCli := httpClient(t)
	token := loginOrSkip(t, httpCli, baseURL, "Him", "0554")
	_ = createDevice(t, httpCli, baseURL, token, projectID, imei)

	boot, err := fetchBootstrap(t, bootURL, imei)
	if err != nil {
		t.Fatalf("bootstrap fetch failed: %v", err)
	}

	pb := boot.PrimaryBroker
	if len(pb.Endpoints) == 0 {
		t.Fatalf("bootstrap returned no endpoints")
	}
	if len(pb.PublishTopics) == 0 {
		t.Fatalf("bootstrap returned no publish topics")
	}

	msgID := fmt.Sprintf("live-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"packet_type": "heartbeat",
		"project_id":  boot.Context.Project.ID,
		"protocol_id": pb.ProtocolID,
		"device_id":   boot.Identity.UUID,
		"imei":        imei,
		"ts":          time.Now().Unix(),
		"msg_id":      msgID,
	}

	publishTopic := pb.PublishTopics[0]
	endpoint := pb.Endpoints[0]

	if err := publishOnce(endpoint, pb.Username, pb.Password, pb.ClientID, publishTopic, payload); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	present, err := waitForMsgID(ctx, dbURI, msgID)
	if err != nil {
		t.Fatalf("persistence check failed: %v", err)
	}
	if !present {
		t.Fatalf("message with msg_id %s not found in telemetry", msgID)
	}
}

type bootstrapResponse struct {
	PrimaryBroker struct {
		Protocol        string   `json:"protocol"`
		ProtocolID      string   `json:"protocol_id"`
		Host            string   `json:"host"`
		Port            string   `json:"port"`
		Username        string   `json:"username"`
		Password        string   `json:"password"`
		ClientID        string   `json:"client_id"`
		PublishTopics   []string `json:"publish_topics"`
		SubscribeTopics []string `json:"subscribe_topics"`
		Endpoints       []string `json:"endpoints"`
	} `json:"primary_broker"`
	Identity struct {
		IMEI string `json:"imei"`
		UUID string `json:"uuid"`
	} `json:"identity"`
	Context struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"context"`
}

func fetchBootstrap(t testing.TB, bootURL, imei string, bearerToken ...string) (*bootstrapResponse, error) {
	t.Helper()

	apiKey := strings.TrimSpace(os.Getenv("BOOTSTRAP_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("API_KEY"))
	}
	if apiKey == "" {
		apiKey = ensureBootstrapAPIKey(t)
	}

	var lastStatus int
	var lastBody string
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s?imei=%s", strings.TrimRight(bootURL, "/"), imei), nil)
		if err != nil {
			return nil, err
		}

		if len(bearerToken) > 0 && strings.TrimSpace(bearerToken[0]) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken[0]))
		}
		if apiKey != "" {
			req.Header.Set("x-api-key", apiKey)
		}

		resp, err := httpClient(t).Do(req)
		if err != nil {
			return nil, err
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatus = resp.StatusCode
		lastBody = string(bodyBytes)

		if resp.StatusCode == http.StatusOK {
			var out bootstrapResponse
			if err := json.Unmarshal(bodyBytes, &out); err != nil {
				return nil, err
			}
			return &out, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil, fmt.Errorf("unexpected status %d: %s", lastStatus, lastBody)
}

func ensureBootstrapAPIKey(t testing.TB) string {
	t.Helper()

	bootstrapAPIKeyOnce.Do(func() {
		dsn := strings.TrimSpace(os.Getenv("TIMESCALE_URI"))
		if dsn == "" {
			dsn = "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return
		}
		defer pool.Close()

		projectID := strings.TrimSpace(os.Getenv("PROJECT_ID"))
		if projectID == "" {
			projectID = "pm-kusum-solar-pump-msedcl"
		}

		prefix := fmt.Sprintf("ak_e2e_%d", time.Now().UnixNano())
		secret := fmt.Sprintf("s%d", time.Now().UnixNano())
		hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
		if err != nil {
			return
		}

		_, err = pool.Exec(ctx, `INSERT INTO api_keys (name, key_hash, prefix, project_id, scopes, is_active)
			VALUES ($1, $2, $3, $4, $5::jsonb, true)`, "e2e-bootstrap-key", string(hash), prefix, projectID, `["read:telemetry"]`)
		if err != nil {
			return
		}

		bootstrapAPIKey = prefix + "." + secret
	})

	return bootstrapAPIKey
}

func publishOnce(endpoint, username, password, clientID, topic string, payload map[string]interface{}) error {
	serverURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	opts := mqtt.NewClientOptions().AddBroker(serverURL.String())
	if clientID == "" {
		clientID = fmt.Sprintf("e2e-%d", rand.Int63())
	}
	opts.SetClientID(clientID)
	if username != "" {
		opts.SetUsername(username)
	}
	if password != "" {
		opts.SetPassword(password)
	}

	if serverURL.Scheme == "mqtts" {
		tlsCfg, err := tlsConfigFromEnv()
		if err != nil {
			return err
		}
		opts.SetTLSConfig(tlsCfg)
	}

	client, err := connectMQTTWithRetry(opts, 30*time.Second)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Disconnect(200)

	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	pubTok := client.Publish(topic, 1, false, buf)
	if pubTok.Wait() && pubTok.Error() != nil {
		return fmt.Errorf("publish: %w", pubTok.Error())
	}
	return nil
}

func tlsConfigFromEnv() (*tls.Config, error) {
	caPath := os.Getenv("MQTT_CA_PATH")
	insecure := strings.EqualFold(os.Getenv("MQTT_TLS_INSECURE"), "true")
	cfg := &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec // controlled by env for tests
	if caPath == "" {
		return cfg, nil
	}
	pem, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("failed to load CA certs")
	}
	cfg.RootCAs = pool
	return cfg, nil
}

func waitForMsgID(ctx context.Context, dbURI, msgID string) (bool, error) {
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		return false, err
	}
	defer pool.Close()

	var hasHyper bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='telemetry_hyper')`).Scan(&hasHyper)

	for {
		var found int
		err = pool.QueryRow(ctx, `
			SELECT 1
			FROM telemetry
			WHERE (data->>'msg_id' = $1 OR data->>'msgid' = $1 OR data->'data'->>'msg_id' = $1 OR data->'data'->>'msgid' = $1)
			ORDER BY time DESC
			LIMIT 1
		`, msgID).Scan(&found)
		if err == nil {
			return true, nil
		}
		if hasHyper {
			err = pool.QueryRow(ctx, `
				SELECT 1
				FROM telemetry_hyper
				WHERE (data->>'msg_id' = $1 OR data->>'msgid' = $1 OR data->'data'->>'msg_id' = $1 OR data->'data'->>'msgid' = $1)
				ORDER BY time DESC
				LIMIT 1
			`, msgID).Scan(&found)
			if err == nil {
				return true, nil
			}
		}
		if err == nil {
			return true, nil
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		time.Sleep(500 * time.Millisecond)
	}
}
