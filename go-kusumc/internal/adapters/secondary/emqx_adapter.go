package secondary

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type EmqxAdapter struct {
	BaseURL   string
	AppID     string
	AppSecret string
	Token     string
	Client    *http.Client
	AuthzSrc  string
}

func NewEmqxAdapter() *EmqxAdapter {
	url := os.Getenv("EMQX_API_URL")
	if url == "" {
		url = "http://emqx:18083/api/v5" // Default generic
	}
	appID := os.Getenv("EMQX_APP_ID")
	if appID == "" {
		appID = "admin"
	}
	secret := os.Getenv("EMQX_APP_SECRET")
	if secret == "" {
		secret = "public"
	}

	token := os.Getenv("EMQX_API_TOKEN")
	dashboardUser := os.Getenv("EMQX_DASHBOARD_USER")
	dashboardPass := os.Getenv("EMQX_DASHBOARD_PASSWORD")

	authzSrc := os.Getenv("EMQX_AUTHZ_SOURCE_ID")
	if authzSrc == "" {
		authzSrc = "built_in_database"
	}

	emqx := &EmqxAdapter{
		BaseURL:   url,
		AppID:     appID,
		AppSecret: secret,
		Token:     token,
		AuthzSrc:  authzSrc,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}

	emqx.initDashboardToken(dashboardUser, dashboardPass)

	return emqx
}

// initDashboardToken fetches a dashboard token if none is already configured.
func (e *EmqxAdapter) initDashboardToken(user, pass string) {
	if e.Token != "" || user == "" || pass == "" {
		return
	}

	if token, err := loginForToken(e.BaseURL, user, pass); err == nil && token != "" {
		e.Token = token
		log.Printf("[EmqxAdapter] dashboard login succeeded; using token auth")
	} else if err != nil {
		log.Printf("[EmqxAdapter] dashboard login failed: %v", err)
	}
}

// loginForToken performs a dashboard login to fetch an API token.
func loginForToken(baseURL, user, pass string) (string, error) {
	trimmed := strings.TrimSuffix(baseURL, "/")
	loginURL := trimmed + "/login" // baseURL already includes /api/v5

	payload, _ := json.Marshal(map[string]string{
		"username": user,
		"password": pass,
	})

	resp, err := http.Post(loginURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dashboard login failed status=%d", resp.StatusCode)
	}

	var loginResp struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}

	if loginResp.Token == "" {
		return "", fmt.Errorf("dashboard login returned empty token")
	}

	return loginResp.Token, nil
}

func (e *EmqxAdapter) AuthHeader() string {
	return e.getAuthHeader()
}

func (e *EmqxAdapter) getAuthHeader() string {
	if e.Token != "" {
		return "Bearer " + e.Token
	}
	auth := e.AppID + ":" + e.AppSecret
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// ProvisionDevice creates or updates a user in EMQX Built-in DB
func (e *EmqxAdapter) ProvisionDevice(username, password string) error {
	log.Printf("[EmqxAdapter] Provisioning User: %s", username)

	// 1. Create/Upsert User
	endpoint := fmt.Sprintf("%s/authentication/password_based:built_in_database/users", e.BaseURL)

	payload := map[string]string{
		"user_id":  username,
		"password": password,
	}
	body, _ := json.Marshal(payload)

	resp, err := e.doJSON("POST", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		if err := e.ensureAuthenticator(); err != nil {
			return fmt.Errorf("authenticator missing and failed to create: %w", err)
		}

		resp, err = e.doJSON("POST", endpoint, body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode == 409 {
		// User exists, Update Password (PUT)
		log.Printf("[EmqxAdapter] User %s exists. Updating password.", username)
		updateEndpoint := fmt.Sprintf("%s/%s", endpoint, username)

		updatePayload := map[string]string{"password": password}
		updateBody, _ := json.Marshal(updatePayload)

		respUp, err := e.doJSON("PUT", updateEndpoint, updateBody)
		if err != nil {
			return err
		}
		defer respUp.Body.Close()

		if respUp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(respUp.Body)
			return fmt.Errorf("failed to update user: %s", string(bodyBytes))
		}
		return nil
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create user (status=%d url=%s): %s", resp.StatusCode, endpoint, string(bodyBytes))
	}

	return nil
}

// ListClientsByUsername fetches active client IDs for a username.
func (e *EmqxAdapter) ListClientsByUsername(username string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/clients?username=%s", e.BaseURL, username)
	resp, err := e.doJSON("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list clients failed: %s", string(bodyBytes))
	}

	var payload struct {
		Data []struct {
			ClientID string `json:"clientid"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	clients := make([]string, 0, len(payload.Data))
	for _, c := range payload.Data {
		if c.ClientID != "" {
			clients = append(clients, c.ClientID)
		}
	}
	return clients, nil
}

// KillSession terminates a client connection by client ID.
func (e *EmqxAdapter) KillSession(clientID string) error {
	endpoint := fmt.Sprintf("%s/clients/%s", e.BaseURL, clientID)
	resp, err := e.doJSON("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // already gone
	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kill session failed status=%d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// ensureAuthenticator creates the built-in database authenticator if missing.
func (e *EmqxAdapter) ensureAuthenticator() error {
	endpoint := fmt.Sprintf("%s/authentication", e.BaseURL)

	payload := map[string]interface{}{
		"backend":      "built_in_database",
		"user_id_type": "username",
		"mechanism":    "password_based",
		"password_hash_algorithm": map[string]interface{}{
			"name":          "plain",
			"salt_position": "prefix",
		},
	}
	body, _ := json.Marshal(payload)

	resp, err := e.doJSON("POST", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 409 {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("create authenticator failed status=%d: %s", resp.StatusCode, string(bodyBytes))
}

// UpdateACL sets publish/subscribe rules for a user
func (e *EmqxAdapter) UpdateACL(username string, pubTopics, subTopics []string) error {
	log.Printf("[EmqxAdapter] Updating ACLs for: %s", username)

	if err := e.ensureAuthorizationSource(); err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/authorization/sources/%s/rules/users/%s", e.BaseURL, e.AuthzSrc, username)

	type Rule struct {
		Permission string `json:"permission"`
		Action     string `json:"action"`
		Topic      string `json:"topic"`
	}

	var rules []Rule

	for _, t := range pubTopics {
		rules = append(rules, Rule{Permission: "allow", Action: "publish", Topic: t})
	}
	for _, t := range subTopics {
		rules = append(rules, Rule{Permission: "allow", Action: "subscribe", Topic: t})
	}
	// Deny-all fallback to match legacy ACL safety pattern.
	rules = append(rules, Rule{Permission: "deny", Action: "all", Topic: "#"})

	payload := map[string]interface{}{
		"username": username,
		"rules":    rules,
	}
	body, _ := json.Marshal(payload)

	resp, err := e.doJSON("PUT", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set ACLs (status=%d url=%s): %s", resp.StatusCode, endpoint, string(bodyBytes))
	}

	return nil
}

// DeleteACL removes ACL rules for a user so stale accounts cannot publish/subscribe.
func (e *EmqxAdapter) DeleteACL(username string) error {
	if e.AuthzSrc == "" {
		return nil // nothing to delete if authz source is not configured
	}

	endpoint := fmt.Sprintf("%s/authorization/sources/%s/rules/users/%s", e.BaseURL, e.AuthzSrc, username)
	resp, err := e.doJSON("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete ACLs (status=%d url=%s): %s", resp.StatusCode, endpoint, string(bodyBytes))
	}
	return nil
}

// DeleteUser removes a built-in DB user in EMQX.
func (e *EmqxAdapter) DeleteUser(username string) error {
	endpoint := fmt.Sprintf("%s/authentication/password_based:built_in_database/users/%s", e.BaseURL, username)
	resp, err := e.doJSON("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete user (status=%d url=%s): %s", resp.StatusCode, endpoint, string(bodyBytes))
	}
	return nil
}

// SyncBroker ensures authenticator and authorization sources are initialized.
func (e *EmqxAdapter) SyncBroker() error {
	if err := e.ensureAuthenticator(); err != nil {
		return err
	}
	if err := e.ensureAuthorizationSource(); err != nil {
		return err
	}
	return e.checkAuthorizationSource()
}

// ensureAuthorizationSource creates or verifies the configured authorization source exists.
func (e *EmqxAdapter) ensureAuthorizationSource() error {
	maxAttempts := 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := e.checkAuthorizationSource(); err == nil {
			return nil
		}

		if err := e.createAuthorizationSource(); err == nil {
			return nil
		}

		delay := time.Duration(1<<attempt) * time.Second
		time.Sleep(delay)
	}
	return fmt.Errorf("authorization source %s unavailable after retries", e.AuthzSrc)
}

func (e *EmqxAdapter) checkAuthorizationSource() error {
	endpoint := fmt.Sprintf("%s/authorization/sources", e.BaseURL)

	resp, err := e.doJSON("GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("check authz source status=%d", resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}

	if sources, ok := payload["sources"].([]interface{}); ok {
		for _, s := range sources {
			if smap, ok := s.(map[string]interface{}); ok {
				if name, _ := smap["name"].(string); name == e.AuthzSrc {
					return nil
				}
				if t, _ := smap["type"].(string); t == "built_in_database" && e.AuthzSrc == "built_in_database" {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("authz source %s not found", e.AuthzSrc)
}

func (e *EmqxAdapter) createAuthorizationSource() error {
	endpoint := fmt.Sprintf("%s/authorization/sources", e.BaseURL)
	body := map[string]interface{}{
		"type":   "built_in_database",
		"enable": true,
	}
	buf, _ := json.Marshal(body)
	resp, err := e.doJSON("POST", endpoint, buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 409 {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("create authz source failed: %s", string(bodyBytes))
}

// Publish sends an MQTT message via HTTP API (Bonus)
func (e *EmqxAdapter) Publish(topic string, payload interface{}) error {
	endpoint := fmt.Sprintf("%s/publish", e.BaseURL)

	payloadStr := ""
	if s, ok := payload.(string); ok {
		payloadStr = s
	} else {
		b, _ := json.Marshal(payload)
		payloadStr = string(b)
	}

	msg := map[string]interface{}{
		"topic":   topic,
		"payload": payloadStr,
		"qos":     1,
	}
	body, _ := json.Marshal(msg)

	resp, err := e.doJSON("POST", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("publish failed: %d", resp.StatusCode)
	}
	return nil
}

// doJSON sends an HTTP request with JSON payload, applying a small retry for transient failures.
func (e *EmqxAdapter) doJSON(method, url string, body []byte) (*http.Response, error) {
	headerAuth := e.getAuthHeader()
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var req *http.Request
		if body != nil {
			req, _ = http.NewRequest(method, url, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, url, nil)
		}
		req.Header.Set("Authorization", headerAuth)

		resp, err := e.Client.Do(req)
		if err == nil {
			return resp, nil
		}

		if attempt == maxAttempts {
			return nil, fmt.Errorf("emqx request failed after %d attempts (url=%s): %w", attempt, url, err)
		}
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	return nil, fmt.Errorf("emqx request failed (url=%s)", url)
}
