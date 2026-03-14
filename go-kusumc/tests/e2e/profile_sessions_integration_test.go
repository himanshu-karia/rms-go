//go:build integration
// +build integration

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestProfileAndSessions_Flow(t *testing.T) {
	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	if strings.EqualFold(os.Getenv("HTTP_TLS_INSECURE"), "") {
		_ = os.Setenv("HTTP_TLS_INSECURE", "true")
	}

	httpCli := httpClient(t)
	token := loginOrSkip(t, httpCli, baseURL, "Him", "0554")

	profile := mustAuthJSONMapStrict(t, httpCli, token, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/profile", nil, http.StatusOK)
	userID := strings.TrimSpace(strVal(profile, "id"))
	if userID == "" {
		t.Fatalf("profile id missing: body=%v", profile)
	}
	displayName := strings.TrimSpace(strVal(profile, "display_name"))
	if displayName == "" {
		displayName = "Admin"
	}

	selfSessions := mustAuthJSONMapStrict(t, httpCli, token, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/auth/sessions", nil, http.StatusOK)
	if intVal(selfSessions, "count") <= 0 {
		t.Fatalf("expected self sessions count > 0, got body=%v", selfSessions)
	}
	assertSessionShape(t, selfSessions)

	adminSessions := mustAuthJSONMapStrict(t, httpCli, token, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/admin/users/"+userID+"/sessions", nil, http.StatusOK)
	if strings.TrimSpace(strVal(adminSessions, "user_id")) != userID {
		t.Fatalf("unexpected admin sessions user_id: body=%v", adminSessions)
	}
	if intVal(adminSessions, "count") <= 0 {
		t.Fatalf("expected admin sessions count > 0, got body=%v", adminSessions)
	}
	assertSessionShape(t, adminSessions)

	otpReqBody := map[string]any{
		"channel": "sms",
		"target":  "9999999999",
		"purpose": "profile_update",
	}
	otpResp := mustAuthJSONMapStrict(t, httpCli, token, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/profile/otp/request", otpReqBody, http.StatusOK)
	otpRef := strings.TrimSpace(strVal(otpResp, "otp_ref"))
	otpCode := strings.TrimSpace(strVal(otpResp, "dev_otp_code"))
	if otpRef == "" || otpCode == "" {
		t.Fatalf("otp request missing otp_ref or dev_otp_code: body=%v", otpResp)
	}

	verifyBody := map[string]any{
		"otp_ref":      otpRef,
		"otp":          otpCode,
		"display_name": displayName,
	}
	verifyResp := mustAuthJSONMapStrict(t, httpCli, token, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/profile/otp/verify", verifyBody, http.StatusOK)
	if ok, _ := verifyResp["ok"].(bool); !ok {
		t.Fatalf("expected ok=true on otp verify, got body=%v", verifyResp)
	}
	if verified, _ := verifyResp["verified"].(bool); !verified {
		t.Fatalf("expected verified=true on otp verify, got body=%v", verifyResp)
	}

	profileAfter := mustAuthJSONMapStrict(t, httpCli, token, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/profile", nil, http.StatusOK)
	if strings.TrimSpace(strVal(profileAfter, "id")) != userID {
		t.Fatalf("profile id changed unexpectedly after otp flow: body=%v", profileAfter)
	}
}

func assertSessionShape(t testing.TB, payload map[string]any) {
	t.Helper()
	rawSessions, ok := payload["sessions"].([]any)
	if !ok || len(rawSessions) == 0 {
		t.Fatalf("sessions array missing/empty: body=%v", payload)
	}
	first, ok := rawSessions[0].(map[string]any)
	if !ok {
		t.Fatalf("first session has wrong shape: %T", rawSessions[0])
	}
	required := []string{"session_id", "user_id", "started_at", "last_used_at", "expires_at", "revoked", "ip_address", "geo_estimate", "duration_seconds", "user_agent"}
	for _, key := range required {
		if _, exists := first[key]; !exists {
			t.Fatalf("session payload missing key %q: %v", key, first)
		}
	}
}

func mustAuthJSONMapStrict(t testing.TB, cli *http.Client, token, method, url string, body any, expectedStatus int) map[string]any {
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

	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected status=%d got=%d url=%s body=%s", expectedStatus, resp.StatusCode, url, string(b))
	}

	out := map[string]any{}
	if len(bytes.TrimSpace(b)) > 0 {
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("decode json failed: %v body=%s", err, string(b))
		}
	}
	return out
}

func loginOrSkip(t testing.TB, cli *http.Client, baseURL, user, pass string) string {
	t.Helper()
	trimmed := strings.TrimRight(baseURL, "/")
	endpoints := []string{
		trimmed + "/api/auth/login",
		trimmed + "/api/v1/auth/login",
		trimmed + "/auth/login",
		trimmed + "/v1/auth/login",
	}
	bodyVariants := []map[string]string{
		{"username": user, "password": pass},
		{"email": user, "password": pass},
	}

	var lastStatus int
	var lastBody string
	var lastErr error

	for _, endpoint := range endpoints {
		for _, body := range bodyVariants {
			buf, _ := json.Marshal(body)
			resp, err := cli.Post(endpoint, "application/json", bytes.NewReader(buf))
			if err != nil {
				lastErr = err
				continue
			}
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastStatus = resp.StatusCode
			lastBody = string(payload)

			if resp.StatusCode != http.StatusOK {
				continue
			}
			var out map[string]any
			if err := json.Unmarshal(payload, &out); err != nil {
				lastErr = err
				continue
			}
			token := strings.TrimSpace(strVal(out, "token"))
			if token == "" {
				token = strings.TrimSpace(strVal(out, "access_token"))
			}
			if token != "" {
				return token
			}
		}
	}

	if lastErr != nil && lastStatus == 0 {
		t.Skipf("integration server unavailable for login: %v", lastErr)
		return ""
	}
	t.Skipf("integration login unavailable across known auth routes: status=%d body=%s", lastStatus, lastBody)
	return ""
}
