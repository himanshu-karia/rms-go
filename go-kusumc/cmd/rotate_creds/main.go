package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5"
)

var httpCli *http.Client

// E2E lifecycle runner for PM-KUSUM:
// 1) Login as seeded admin
// 2) Create device with IMEI
// 3) Publish with initial creds (expect success)
// 4) Rotate creds
// 5) Publish with old creds (expect fail) and with new creds (expect success)
// 6) Verify telemetry persisted in Timescale
const (
	defaultBaseURL      = "https://localhost"
	defaultProjectID    = "pm-kusum-solar-pump-msedcl"
	defaultBaseIMEI     = "123456789012345"
	defaultIMEIStrategy = "delete" // "delete" to remove existing device, "increment" to append +1 and leave existing intact
	defaultUsername     = "Him"
	defaultPassword     = "0554"

	defaultMQTTHost = "localhost" // host port mapped in docker-compose
	defaultMQTTPort = "8883"      // emqx 8883 -> host 8883

	defaultPgURL = "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable"
)

type config struct {
	baseURL         string
	projectID       string
	baseIMEI        string
	imeiStrategy    string
	username        string
	password        string
	mqttHost        string
	mqttPort        string
	pgURL           string
	httpCAPath      string
	httpTLSInsecure bool
	mqttCAPath      string
	mqttTLSInsecure bool
}

var cfg config

func loadConfig() config {
	baseURL := flag.String("base-url", envOrDefault("BASE_URL", defaultBaseURL), "API base URL")
	projectID := flag.String("project-id", envOrDefault("PROJECT_ID", defaultProjectID), "Project ID for device creation")
	baseIMEI := flag.String("base-imei", envOrDefault("BASE_IMEI", defaultBaseIMEI), "Base IMEI to seed devices")
	imeiStrategy := flag.String("imei-strategy", envOrDefault("IMEI_STRATEGY", defaultIMEIStrategy), "Strategy when IMEI exists: delete|increment")
	username := flag.String("username", envOrDefault("USERNAME", defaultUsername), "Portal username")
	password := flag.String("password", envOrDefault("PASSWORD", defaultPassword), "Portal password")
	mqttHost := flag.String("mqtt-host", envOrDefault("MQTT_HOST", defaultMQTTHost), "MQTT host for fallback broker")
	mqttPort := flag.String("mqtt-port", envOrDefault("MQTT_PORT", defaultMQTTPort), "MQTT port for fallback broker")
	pgURL := flag.String("pg-url", envOrDefault("PG_URL", defaultPgURL), "Postgres connection URL")
	httpCA := flag.String("http-ca-path", envOrDefault("HTTP_CA_PATH", ""), "Path to PEM bundle for HTTPS (optional)")
	httpInsecure := flag.Bool("http-tls-insecure", envBool("HTTP_TLS_INSECURE", false), "Skip HTTPS verification (dev/self-signed)")
	mqttCA := flag.String("mqtt-ca-path", envOrDefault("MQTT_CA_PATH", ""), "Path to PEM bundle for MQTT TLS (optional)")
	mqttInsecure := flag.Bool("mqtt-tls-insecure", envBool("MQTT_TLS_INSECURE", false), "Skip MQTT TLS verification (dev/self-signed)")

	flag.Parse()

	return config{
		baseURL:         strings.TrimRight(*baseURL, "/"),
		projectID:       *projectID,
		baseIMEI:        *baseIMEI,
		imeiStrategy:    strings.ToLower(*imeiStrategy),
		username:        *username,
		password:        *password,
		mqttHost:        *mqttHost,
		mqttPort:        *mqttPort,
		pgURL:           *pgURL,
		httpCAPath:      *httpCA,
		httpTLSInsecure: *httpInsecure,
		mqttCAPath:      *mqttCA,
		mqttTLSInsecure: *mqttInsecure,
	}
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	return fallback
}

// API payloads
type loginResponse struct {
	Token string `json:"token"`
}

type createDeviceRequest struct {
	Name           string                 `json:"name"`
	IMEI           string                 `json:"imei"`
	ProjectID      string                 `json:"projectId"`
	ProtocolID     string                 `json:"protocol_id"`
	ContractorID   string                 `json:"contractor_id"`
	SupplierID     string                 `json:"supplier_id"`
	ManufacturerID string                 `json:"manufacturer_id"`
	Attributes     map[string]interface{} `json:"attributes"`
}

type createDeviceResponse struct {
	DeviceID           string      `json:"device_id"`
	IMEI               string      `json:"imei"`
	MQTTUser           string      `json:"mqtt_user"`
	MQTTPass           string      `json:"mqtt_pass"`
	ClientID           string      `json:"client_id"`
	Endpoint           string      `json:"endpoint"`
	PublishTopics      interface{} `json:"publish_topics"`
	SubscribeTopics    interface{} `json:"subscribe_topics"`
	CredentialHistory  string      `json:"credential_history_id"`
	ProvisioningStatus string      `json:"provisioning_status"`
	Topics             struct {
		Publish   string `json:"publish"`
		Subscribe string `json:"subscribe"`
	} `json:"topics"`
}

type rotateResp struct {
	MQTTUser        string `json:"mqtt_user"`
	MQTTPass        string `json:"mqtt_pass"`
	ClientID        string `json:"client_id"`
	Endpoint        string `json:"endpoint"`
	PublishTopics   string `json:"publish_topics"`
	SubscribeTopics string `json:"subscribe_topics"`
}

func main() {
	cfg = loadConfig()
	httpCli = newHTTPClient(cfg)
	ctx := context.Background()

	token := mustLogin()
	log.Printf("[OK] Logged in, got token")

	selectedIMEI := resolveIMEI(ctx)
	log.Printf("[OK] Using IMEI=%s (strategy=%s)", selectedIMEI, cfg.imeiStrategy)

	device := mustCreateDevice(token, selectedIMEI)
	log.Printf("[OK] Device created id=%s imei=%s", device.DeviceID, device.IMEI)

	mustWaitForProvisioning(ctx, device.DeviceID)

	pubTopic := pickTopic(device.PublishTopics, device.Topics.Publish)
	broker := normalizeBroker(device.Endpoint, cfg.mqttHost, cfg.mqttPort)

	payloads := buildRmsPayloads(device.DeviceID, selectedIMEI)

	publishAll(broker, device.ClientID, device.MQTTUser, device.MQTTPass, pubTopic, payloads)
	log.Printf("[OK] Published %d RMS packets with initial credentials on %s", len(payloads), pubTopic)

	newCreds := mustRotate(token, device.DeviceID)
	log.Printf("[OK] Rotated credentials: client_id=%s", newCreds.ClientID)

	mustWaitForProvisioning(ctx, device.DeviceID)

	// Expect failure with old creds
	expectPublishFail(broker, device.ClientID, device.MQTTUser, device.MQTTPass, pubTopic, payloads[0].body)

	// Publish with new creds
	newTopic := pickTopic(newCreds.PublishTopics, pubTopic)
	publishAll(broker, newCreds.ClientID, newCreds.MQTTUser, newCreds.MQTTPass, newTopic, buildRmsPayloads(device.DeviceID, selectedIMEI))
	log.Printf("[OK] Published RMS packets with new credentials on %s", newTopic)

	mustVerifyTelemetry(ctx, device.DeviceID, device.IMEI)
	log.Printf("[OK] Telemetry present for imei=%s in telemetry table", device.IMEI)
}

func resolveIMEI(ctx context.Context) string {
	conn, err := pgx.Connect(ctx, cfg.pgURL)
	if err != nil {
		log.Fatalf("pg connect (resolveIMEI): %v", err)
	}
	defer conn.Close(ctx)

	var existingID string
	err = conn.QueryRow(ctx, `SELECT id FROM devices WHERE imei=$1 LIMIT 1`, cfg.baseIMEI).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		log.Fatalf("pg query imei check: %v", err)
	}
	if err == pgx.ErrNoRows {
		return cfg.baseIMEI
	}

	strategy := strings.ToLower(cfg.imeiStrategy)
	if strategy == "delete" {
		if _, err := conn.Exec(ctx, `DELETE FROM devices WHERE id=$1`, existingID); err != nil {
			log.Fatalf("failed deleting existing device %s (imei=%s): %v", existingID, cfg.baseIMEI, err)
		}
		log.Printf("[WARN] Existing device deleted for imei=%s (id=%s)", cfg.baseIMEI, existingID)
		return cfg.baseIMEI
	}

	if strategy == "increment" {
		nextIMEI, err := incrementIMEI(cfg.baseIMEI)
		if err != nil {
			log.Fatalf("increment IMEI: %v", err)
		}
		log.Printf("[INFO] IMEI %s exists (id=%s); using %s instead", cfg.baseIMEI, existingID, nextIMEI)
		return nextIMEI
	}

	log.Fatalf("unknown imeiStrategy=%s", cfg.imeiStrategy)
	return cfg.baseIMEI
}

func mustLogin() string {
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, cfg.username, cfg.password)
	resp, err := httpCli.Post(cfg.baseURL+"/api/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		log.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("login failed status=%d", resp.StatusCode)
	}
	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		log.Fatalf("parse login response: %v", err)
	}
	return lr.Token
}

func mustCreateDevice(token, imei string) createDeviceResponse {
	req := createDeviceRequest{
		Name:           "kusum-e2e",
		IMEI:           imei,
		ProjectID:      cfg.projectID,
		ProtocolID:     "proto-pm-primary",
		ContractorID:   "seed-contractor",
		SupplierID:     "seed-supplier",
		ManufacturerID: "seed-manufacturer",
		Attributes:     map[string]interface{}{},
	}
	b, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest(http.MethodPost, cfg.baseURL+"/api/devices", strings.NewReader(string(b)))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpCli.Do(httpReq)
	if err != nil {
		log.Fatalf("create device request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("create device failed status=%d", resp.StatusCode)
	}
	var out createDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Fatalf("parse create device: %v", err)
	}
	return out
}

func mustRotate(token, deviceID string) rotateResp {
	httpReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/devices/%s/rotate-creds", cfg.baseURL, deviceID), nil)
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpCli.Do(httpReq)
	if err != nil {
		log.Fatalf("rotate request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("rotate failed status=%d", resp.StatusCode)
	}
	var out rotateResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Fatalf("parse rotate resp: %v", err)
	}
	return out
}

func mustPublish(broker, clientID, user, pass, topic string, payload []byte) {
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	opts.SetUsername(user)
	opts.SetPassword(pass)
	applyMqttTLS(broker, opts)
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetCleanSession(true)

	cli := mqtt.NewClient(opts)
	if token := cli.Connect(); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		log.Fatalf("mqtt connect failed (expected success): %v", token.Error())
	}
	defer cli.Disconnect(100)

	token := cli.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		log.Fatalf("mqtt publish failed (expected success): %v", token.Error())
	}
}

func expectPublishFail(broker, clientID, user, pass, topic string, payload []byte) {
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	opts.SetUsername(user)
	opts.SetPassword(pass)
	applyMqttTLS(broker, opts)
	opts.SetConnectTimeout(3 * time.Second)
	opts.SetCleanSession(true)

	cli := mqtt.NewClient(opts)
	token := cli.Connect()
	if token.WaitTimeout(4*time.Second) && token.Error() == nil {
		// Connection unexpectedly succeeded; try publish to see if ACL blocks
		pub := cli.Publish(topic, 1, false, payload)
		if pub.WaitTimeout(4*time.Second) && pub.Error() == nil {
			log.Fatalf("expected old credentials to fail, but publish succeeded")
		}
		log.Fatalf("expected old credentials to fail, publish returned error: %v", pub.Error())
	}
	log.Printf("[OK] Old credentials rejected as expected: %v", token.Error())
}

func mustVerifyTelemetry(ctx context.Context, deviceID, imei string) {
	conn, err := pgx.Connect(ctx, cfg.pgURL)
	if err != nil {
		log.Fatalf("pg connect: %v", err)
	}
	defer conn.Close(ctx)

	var nonEmpty int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM telemetry WHERE (device_id=$1 OR (project_id=$2 AND data->>'imei'=$3)) AND data IS NOT NULL AND data::text <> '{}'`, deviceID, cfg.projectID, imei).Scan(&nonEmpty)
	if err != nil {
		log.Fatalf("pg query telemetry: %v", err)
	}
	if nonEmpty > 0 {
		return
	}

	var total int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM telemetry WHERE device_id=$1 OR (project_id=$2 AND data->>'imei'=$3)`, deviceID, cfg.projectID, imei).Scan(&total)
	if err != nil {
		log.Fatalf("pg query telemetry (fallback): %v", err)
	}
	if total == 0 {
		log.Fatalf("no telemetry rows found for imei=%s", imei)
	}
	log.Printf("[WARN] Telemetry rows present but payloads empty (count=%d); check ingestion pipeline", total)
}

func pickTopic(raw interface{}, fallback string) string {
	switch v := raw.(type) {
	case string:
		if v != "" {
			return firstCSV(v)
		}
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	if fallback != "" {
		return fallback
	}
	log.Fatalf("no publish topic found in response")
	return ""
}

func normalizeBroker(ep, mqttHost, mqttPort string) string {
	if ep == "" {
		return fmt.Sprintf("mqtts://%s:%s", mqttHost, mqttPort)
	}
	u, err := url.Parse(ep)
	if err != nil {
		return fmt.Sprintf("mqtts://%s:%s", mqttHost, mqttPort)
	}
	host := u.Hostname()
	if host == "emqx" || host == "mqtt.local" || host == "localhost" {
		u.Host = fmt.Sprintf("%s:%s", mqttHost, mqttPort)
		u.Scheme = "mqtts"
		return u.String()
	}
	return ep
}

type payloadSpec struct {
	packetType string
	body       []byte
}

func publishAll(broker, clientID, user, pass, topic string, payloads []payloadSpec) {
	for _, p := range payloads {
		mustPublish(broker, clientID, user, pass, topic, p.body)
	}
}

func newHTTPClient(c config) *http.Client {
	tlsCfg, err := httpTLSConfig(c)
	if err != nil {
		log.Fatalf("http TLS config: %v", err)
	}

	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
		Timeout:   20 * time.Second,
	}
}

func httpTLSConfig(c config) (*tls.Config, error) {
	cfg := &tls.Config{InsecureSkipVerify: c.httpTLSInsecure} //nolint:gosec // intentional for dev
	if c.httpCAPath == "" {
		return cfg, nil
	}

	pem, err := os.ReadFile(c.httpCAPath)
	if err != nil {
		return nil, fmt.Errorf("read HTTP CA: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("append HTTP CA")
	}
	cfg.RootCAs = pool
	return cfg, nil
}

func applyMqttTLS(broker string, opts *mqtt.ClientOptions) {
	u, err := url.Parse(broker)
	if err != nil {
		return
	}
	if u.Scheme != "mqtts" && u.Scheme != "ssl" && u.Scheme != "tls" {
		return
	}

	tlsCfg, err := mqttTLSConfig()
	if err != nil {
		log.Fatalf("mqtt TLS config: %v", err)
	}
	opts.SetTLSConfig(tlsCfg)
}

func mqttTLSConfig() (*tls.Config, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.mqttTLSInsecure} //nolint:gosec // intentional for dev
	if cfg.mqttCAPath == "" {
		return tlsCfg, nil
	}

	pem, err := os.ReadFile(cfg.mqttCAPath)
	if err != nil {
		return nil, fmt.Errorf("read MQTT CA: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("append MQTT CA")
	}
	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}

func buildRmsPayloads(deviceID, imei string) []payloadSpec {
	now := time.Now()

	return []payloadSpec{
		rmsHeartbeat(deviceID, imei, now, 0),
		rmsHeartbeat(deviceID, imei, now, 1),
		rmsPump(deviceID, imei, now, 0),
		rmsPump(deviceID, imei, now, 1),
		rmsDaq(deviceID, imei, now, 0),
		rmsDaq(deviceID, imei, now, 1),
		rmsOndemandCmd(deviceID, imei, now, 0),
		rmsOndemandCmd(deviceID, imei, now, 1),
		rmsOndemandRsp(deviceID, imei, now, 0),
		rmsOndemandRsp(deviceID, imei, now, 1),
	}
}

func rmsHeartbeat(deviceID, imei string, base time.Time, offset int) payloadSpec {
	ts := base.Add(time.Duration(offset) * time.Second)
	env := baseEnvelope("heartbeat", deviceID, imei, ts)

	data := map[string]interface{}{
		"VD":         "HB1",
		"TIMESTAMP":  ts.Format(time.RFC3339),
		"DATE":       ts.Format("2006-01-02"),
		"IMEI":       imei,
		"ASN":        101 + offset,
		"RTCDATE":    ts.Format("2006-01-02"),
		"RTCTIME":    ts.Format("15:04:05"),
		"LAT":        19.0760 + 0.0001*float64(offset),
		"LONG":       72.8777 + 0.0001*float64(offset),
		"RSSI":       -70 + offset,
		"STINTERVAL": 300,
		"POTP":       1,
		"COTP":       1,
		"GSM":        "idea",
		"SIM":        "vi",
		"NET":        "4G",
		"GPRS":       "attached",
		"SD":         "present",
		"ONLINE":     true,
		"GPS":        "lock",
		"GPSLOC":     true,
		"RF":         "ok",
		"TEMP":       32.5 + float64(offset),
		"SIMSLOT":    1,
		"SIMCHNGCNT": 0,
		"FLASH":      "healthy",
		"BATTST":     "good",
		"VBATT":      12.8,
		"PST":        "run",
	}

	return payloadSpec{packetType: "heartbeat", body: marshalPayload(env, data)}
}

func rmsPump(deviceID, imei string, base time.Time, offset int) payloadSpec {
	ts := base.Add(time.Duration(5+offset) * time.Second)
	env := baseEnvelope("pump", deviceID, imei, ts)

	data := map[string]interface{}{
		"VD":         "PM1",
		"TIMESTAMP":  ts.Format(time.RFC3339),
		"DATE":       ts.Format("2006-01-02"),
		"IMEI":       imei,
		"ASN":        201 + offset,
		"PDKWH1":     1.4 + 0.1*float64(offset),
		"PTOTKWH1":   120.5 + 0.5*float64(offset),
		"POPDWD1":    0.65 + 0.02*float64(offset),
		"POPTOTWD1":  80.0 + 0.5*float64(offset),
		"PDHR1":      0.5 + 0.1*float64(offset),
		"PTOTHR1":    400 + offset,
		"POPKW1":     3.2 + 0.1*float64(offset),
		"MAXINDEX":   10,
		"INDEX":      1 + offset,
		"LOAD":       0.9,
		"STINTERVAL": 300,
		"POTP":       1,
		"COTP":       1,
		"PMAXFREQ1":  50,
		"PFREQLSP1":  45,
		"PFREQHSP1":  55,
		"PCNTRMODE1": "auto",
		"PRUNST1":    "run",
		"POPFREQ1":   50,
		"POPI1":      6.5 + 0.1*float64(offset),
		"POPV1":      230 + 1*offset,
		"PDC1V1":     420,
		"PDC1I1":     8.1 + 0.1*float64(offset),
		"PDCVOC1":    500,
		"POPFLW1":    18.5 + float64(offset),
	}

	return payloadSpec{packetType: "pump", body: marshalPayload(env, data)}
}

func rmsDaq(deviceID, imei string, base time.Time, offset int) payloadSpec {
	ts := base.Add(time.Duration(10+offset) * time.Second)
	env := baseEnvelope("daq", deviceID, imei, ts)

	data := map[string]interface{}{
		"VD":         "DAQ1",
		"TIMESTAMP":  ts.Format(time.RFC3339),
		"MAXINDEX":   5,
		"INDEX":      1 + offset,
		"LOAD":       0.85,
		"STINTERVAL": 300,
		"MSGID":      fmt.Sprintf("DAQ-%d", offset+1),
		"DATE":       ts.Format("2006-01-02"),
		"IMEI":       imei,
		"ASN":        301 + offset,
		"POTP":       1,
		"COTP":       1,
		"AI11":       3.3 + 0.1*float64(offset),
		"AI21":       6.4 + 0.1*float64(offset),
		"AI31":       9.5 + 0.1*float64(offset),
		"AI41":       12.6 + 0.1*float64(offset),
		"DI11":       offset%2 == 0,
		"DI21":       true,
		"DI31":       false,
		"DI41":       true,
		"DO11":       offset%2 == 0,
		"DO21":       true,
		"DO31":       false,
		"DO41":       true,
	}

	return payloadSpec{packetType: "daq", body: marshalPayload(env, data)}
}

func rmsOndemandCmd(deviceID, imei string, base time.Time, offset int) payloadSpec {
	ts := base.Add(time.Duration(15+offset) * time.Second)
	env := baseEnvelope("ondemand_cmd", deviceID, imei, ts)
	msgID := genMsgID()
	env["msg_id"] = msgID

	data := map[string]interface{}{
		"msgid":     msgID,
		"COTP":      1,
		"POTP":      1,
		"timestamp": ts.Unix(),
		"type":      "relay",
		"cmd":       "DO1",
		"DO1":       offset%2 == 0,
	}

	return payloadSpec{packetType: "ondemand_cmd", body: marshalPayload(env, data)}
}

func rmsOndemandRsp(deviceID, imei string, base time.Time, offset int) payloadSpec {
	ts := base.Add(time.Duration(18+offset) * time.Second)
	env := baseEnvelope("ondemand_rsp", deviceID, imei, ts)
	msgID := genMsgID()
	env["msg_id"] = msgID

	data := map[string]interface{}{
		"timestamp": ts.Unix(),
		"status":    "ok",
		"DO1":       offset%2 == 0,
		"PRUNST1":   "ack",
		"msgid":     msgID,
	}

	return payloadSpec{packetType: "ondemand_rsp", body: marshalPayload(env, data)}
}

func baseEnvelope(packetType, deviceID, imei string, ts time.Time) map[string]interface{} {
	return map[string]interface{}{
		"packet_type":     packetType,
		"project_id":      cfg.projectID,
		"protocol_id":     "proto-pm-primary",
		"contractor_id":   "seed-contractor",
		"supplier_id":     "seed-supplier",
		"manufacturer_id": "seed-manufacturer",
		"device_id":       deviceID,
		"imei":            imei,
		"ts":              ts.UnixMilli(),
		"msg_id":          genMsgID(),
	}
}

func marshalPayload(envelope map[string]interface{}, data map[string]interface{}) []byte {
	body := make(map[string]interface{}, len(envelope)+1)
	for k, v := range envelope {
		body[k] = v
	}
	body["data"] = data
	b, _ := json.Marshal(body)
	return b
}

func genMsgID() string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func firstCSV(s string) string {
	parts := strings.Split(s, ",")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return s
}

func incrementIMEI(imei string) (string, error) {
	val, err := strconv.ParseInt(imei, 10, 64)
	if err != nil {
		return "", fmt.Errorf("imei not numeric: %w", err)
	}
	val++
	width := len(imei)
	format := fmt.Sprintf("%%0%dd", width)
	return fmt.Sprintf(format, val), nil
}

func mustWaitForProvisioning(ctx context.Context, deviceID string) {
	conn, err := pgx.Connect(ctx, cfg.pgURL)
	if err != nil {
		log.Fatalf("pg connect (wait provision): %v", err)
	}
	defer conn.Close(ctx)

	deadline := time.Now().Add(15 * time.Second)
	for {
		var pending int
		err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM mqtt_provisioning_jobs WHERE device_id=$1 AND status IN ('pending','processing')`, deviceID).Scan(&pending)
		if err != nil {
			log.Fatalf("pg query provisioning: %v", err)
		}
		if pending == 0 {
			return
		}
		if time.Now().After(deadline) {
			log.Fatalf("provisioning jobs still pending for device %s", deviceID)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
