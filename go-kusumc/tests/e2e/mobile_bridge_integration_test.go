//go:build integration
// +build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMobileIngest_IdempotencyReplay(t *testing.T) {
	baseURL := getenv("BASE_URL", "http://localhost:8081")
	dsn := getenv("TIMESCALE_URI", "postgres://postgres:password@timescaledb:5432/telemetry?sslmode=disable")
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")

	if strings.EqualFold(os.Getenv("HTTP_TLS_INSECURE"), "") {
		_ = os.Setenv("HTTP_TLS_INSECURE", "true")
	}

	httpCli := httpClient(t)
	adminToken := mustLogin(t, httpCli, baseURL, "Him", "0554")

	pool := mustPool(t, dsn)
	defer pool.Close()
	mustEnsureMobileDedupeTable(t, pool)

	deviceID := "mobile-e2e-device"
	runID := time.Now().UnixNano()
	idemKey := "idem-mobile-" + strconv.FormatInt(runID, 10)
	batchKey := "batch-mobile-" + strconv.FormatInt(runID, 10)
	body := map[string]any{
		"project_id": projectID,
		"device_id":  deviceID,
		"packets": []map[string]any{
			{
				"idempotency_key": idemKey,
				"captured_at":     time.Now().UTC().Format(time.RFC3339),
				"topic_suffix":    "heartbeat",
				"raw_payload": map[string]any{
					"imei":        "869630050762180",
					"packet_type": "heartbeat",
					"msgid":       "mobile-e2e-msg-" + strconv.FormatInt(runID, 10),
					"timestamp":   time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	}

	url := strings.TrimRight(baseURL, "/") + "/api/mobile/ingest"
	first := mustAuthJSONMap(t, httpCli, adminToken, http.MethodPost, url, body, http.StatusOK, map[string]string{"Idempotency-Key": batchKey + "-1"})
	if skipped, _ := first["__skipped__"].(bool); skipped {
		t.Skip("mobile ingest route unavailable in current running server")
	}
	if intVal(first, "accepted") != 1 {
		t.Fatalf("expected first accepted=1, got body=%v", first)
	}
	if intVal(first, "duplicates") != 0 {
		t.Fatalf("expected first duplicates=0, got body=%v", first)
	}

	second := mustAuthJSONMap(t, httpCli, adminToken, http.MethodPost, url, body, http.StatusOK, map[string]string{"Idempotency-Key": batchKey + "-2"})
	if intVal(second, "duplicates") != 1 {
		t.Fatalf("expected second duplicates=1, got body=%v", second)
	}

	results, _ := second["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("expected duplicate results entry, got body=%v", second)
	}
	firstResult, _ := results[0].(map[string]any)
	if strings.TrimSpace(strVal(firstResult, "status")) != "duplicate" {
		t.Fatalf("expected duplicate status, got body=%v", second)
	}
	if _, ok := firstResult["prior_result"]; !ok {
		t.Fatalf("expected prior_result on duplicate replay, got body=%v", second)
	}
}

func TestMobileCommandStatus_Mapping(t *testing.T) {
	baseURL := getenv("BASE_URL", "http://localhost:8081")
	dsn := getenv("TIMESCALE_URI", "postgres://postgres:password@timescaledb:5432/telemetry?sslmode=disable")

	if strings.EqualFold(os.Getenv("HTTP_TLS_INSECURE"), "") {
		_ = os.Setenv("HTTP_TLS_INSECURE", "true")
	}

	httpCli := httpClient(t)
	adminToken := mustLogin(t, httpCli, baseURL, "Him", "0554")

	pool := mustPool(t, dsn)
	defer pool.Close()

	var corrID string
	err := pool.QueryRow(context.Background(), `SELECT correlation_id::text FROM command_requests ORDER BY created_at DESC LIMIT 1`).Scan(&corrID)
	if err != nil || strings.TrimSpace(corrID) == "" {
		t.Skip("no existing command_requests row available for status mapping check")
	}
	now := time.Now().UTC()
	if _, err := pool.Exec(context.Background(), `
		UPDATE command_requests
		SET status = 'published', published_at = $2
		WHERE correlation_id = $1::uuid
	`, corrID, now); err != nil {
		t.Fatalf("update command request status: %v", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/api/mobile/commands/" + corrID + "/status"
	res := mustAuthJSONMap(t, httpCli, adminToken, http.MethodGet, url, nil, http.StatusOK, nil)
	if skipped, _ := res["__skipped__"].(bool); skipped {
		t.Skip("mobile command status route unavailable in current running server")
	}
	if strings.TrimSpace(strVal(res, "status")) != "sent" {
		t.Fatalf("expected mapped status 'sent' for published, got body=%v", res)
	}
}

func mustEnsureMobileDedupeTable(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS mobile_ingest_dedupe (
			id BIGSERIAL PRIMARY KEY,
			project_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			request_hash TEXT,
			status_code INTEGER,
			response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (project_id, device_id, idempotency_key)
		)
	`)
	if err != nil {
		t.Fatalf("ensure mobile_ingest_dedupe table: %v", err)
	}
}

func mustAuthJSONMap(t testing.TB, cli *http.Client, token, method, url string, body any, expectedStatus int, extraHeaders map[string]string) map[string]any {
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return map[string]any{"__skipped__": true}
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status=%d got=%d body=%s", expectedStatus, resp.StatusCode, string(b))
	}

	out := map[string]any{}
	if len(bytes.TrimSpace(b)) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func intVal(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch vv := v.(type) {
	case float64:
		return int(vv)
	case int:
		return vv
	default:
		return 0
	}
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
