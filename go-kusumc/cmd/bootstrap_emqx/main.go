package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
)

// Simple CLI to provision backend service account in EMQX using existing adapter logic.
func main() {
	username := flag.String("username", getenv("SERVICE_MQTT_USERNAME", "backend-service"), "Service MQTT username")
	password := flag.String("password", getenv("SERVICE_MQTT_PASSWORD", "change-me"), "Service MQTT password")
	pub := flag.String("publish", getenv("SERVICE_MQTT_PUB_TOPICS", "+/ondemand,channels/+/alerts"), "Comma-separated publish topics")
	sub := flag.String("subscribe", getenv("SERVICE_MQTT_SUB_TOPICS", "+/heartbeat,+/data,+/daq,+/ondemand,+/errors,channels/+/messages/+,devices/+/telemetry,channels/+/commands/+/resp,channels/+/commands/+/ack,channels/+/alerts"), "Comma-separated subscribe topics")
	dashboardUser := getenv("EMQX_DASHBOARD_USER", "admin")
	dashboardPass := getenv("EMQX_DASHBOARD_PASSWORD", "public")

	flag.Parse()

	emqx := secondary.NewEmqxAdapter()

	// Try dashboard login first; Bearer tokens are accepted by the v5 APIs.
	if token, err := loginForToken(emqx.BaseURL, dashboardUser, dashboardPass); err == nil && token != "" {
		emqx.Token = token
		log.Printf("dashboard login succeeded; using token auth")
	} else if err != nil {
		log.Printf("dashboard login failed: %v", err)
	}

	// Ensure EMQX API is reachable; if Basic creds fail, attempt fallback app bootstrap. If using dashboard token, fail fast.
	if err := waitForEmqxAPI(emqx.BaseURL, emqx.AuthHeader(), 30, 2*time.Second); err != nil {
		if emqx.Token != "" {
			log.Fatalf("EMQX API not reachable with dashboard token: %v", err)
		}

		log.Fatalf("target EMQX creds rejected (%v)", err)
	}

	pubTopics := splitList(*pub)
	subTopics := splitList(*sub)

	if err := emqx.ProvisionDevice(*username, *password); err != nil {
		log.Fatalf("failed to provision service user: %v", err)
	}
	if err := emqx.UpdateACL(*username, pubTopics, subTopics); err != nil {
		log.Fatalf("failed to set service ACLs: %v", err)
	}

	log.Printf("service account provisioned: user=%s pub=%v sub=%v", *username, pubTopics, subTopics)
}

// ensureApplication creates or updates an EMQX management app using fallback creds, so target app_id/app_secret become valid.
func ensureApplication(baseURL, fallbackID, fallbackSecret, targetID, targetSecret string) error {
	baseURL = strings.TrimSuffix(baseURL, "/")
	client := &http.Client{Timeout: 5 * time.Second}
	auth := basic(fallbackID, fallbackSecret)

	appsBase := fmt.Sprintf("%s/apps", baseURL) // EMQX v5 management apps endpoint

	getURL := fmt.Sprintf("%s/%s", appsBase, targetID)
	req, _ := http.NewRequest("GET", getURL, nil)
	req.Header.Set("Authorization", auth)
	resp, err := client.Do(req)
	if err == nil && resp != nil && resp.StatusCode == 200 {
		resp.Body.Close()
		// Update secret to desired value
		return upsertApplication(client, appsBase, auth, targetID, targetSecret, true)
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Create if missing
	if err := upsertApplication(client, appsBase, auth, targetID, targetSecret, false); err != nil {
		return err
	}
	return nil
}

func upsertApplication(client *http.Client, appsBase, auth, targetID, targetSecret string, update bool) error {
	method := "POST"
	url := appsBase
	if update {
		method = "PUT"
		url = fmt.Sprintf("%s/%s", appsBase, targetID)
	}

	payload := map[string]interface{}{
		"app_id":     targetID,
		"name":       targetID,
		"app_secret": targetSecret,
		"enable":     true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(method, url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 { // ok / created
		return nil
	}
	if resp.StatusCode == 409 && !update {
		// Already exists; attempt update once
		return upsertApplication(client, appsBase, auth, targetID, targetSecret, true)
	}
	return fmt.Errorf("ensure application failed status=%d", resp.StatusCode)
}

// waitForEmqxAPI polls the EMQX authentication endpoint until it responds (or times out).
func waitForEmqxAPI(baseURL, authHeader string, attempts int, delay time.Duration) error {
	if baseURL == "" {
		baseURL = "http://emqx:18083/api/v5"
	}

	endpoint := strings.TrimSuffix(baseURL, "/") + "/authentication"
	client := &http.Client{Timeout: 3 * time.Second}
	auth := authHeader

	var lastStatus int
	for i := 1; i <= attempts; i++ {
		req, _ := http.NewRequest("GET", endpoint, nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}

		resp, err := client.Do(req)
		if err == nil && resp != nil {
			lastStatus = resp.StatusCode
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}

		time.Sleep(delay)
	}

	return fmt.Errorf("EMQX API %s not reachable with supplied creds after %d attempts (last status=%d)", endpoint, attempts, lastStatus)
}

// loginForToken exchanges dashboard credentials for a Bearer token.
func loginForToken(baseURL, username, password string) (string, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	log.Printf("attempting dashboard login to %s as %s", baseURL, username)
	loginURL := baseURL + "/login"

	payload := map[string]string{
		"username": username,
		"password": password,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", loginURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var payloadResp struct {
		Token string `json:"token"`
		Data  struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payloadResp); err != nil {
		return "", err
	}

	token := payloadResp.Token
	if token == "" {
		token = payloadResp.Data.Token
	}
	if token == "" {
		return "", fmt.Errorf("login succeeded but token missing")
	}
	return token, nil
}

// basic returns a Basic auth header for EMQX.
func basic(appID, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(appID+":"+secret))
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
