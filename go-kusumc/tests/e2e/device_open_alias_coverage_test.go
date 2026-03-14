//go:build integration

package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestDeviceOpenAliasCoverage(t *testing.T) {
	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")

	httpCli := httpClient(t)
	token := mustLogin(t, httpCli, baseURL, "Him", "0554")
	imei := randomIMEI()
	deviceID := createDevice(t, httpCli, baseURL, token, projectID, imei)
	apiKey := ensureBootstrapAPIKey(t)

	bootstrapAliases := []string{
		"/api/device-open/bootstrap",
		"/api/devices/open/bootstrap",
		"/api/v1/device-open/bootstrap",
		"/api/v1/devices/open/bootstrap",
	}
	for _, p := range bootstrapAliases {
		uri := fmt.Sprintf("%s%s?imei=%s", strings.TrimRight(baseURL, "/"), p, url.QueryEscape(imei))
		status := doOpenGetStatus(t, httpCli, uri, apiKey)
		if !statusIn(status, http.StatusOK, http.StatusTooManyRequests) {
			t.Fatalf("bootstrap alias %s status=%d", p, status)
		}
	}

	openAliases := []string{
		"/api/device-open",
		"/api/devices/open",
		"/api/v1/device-open",
		"/api/v1/devices/open",
	}
	for _, prefix := range openAliases {
		localStatus := doOpenGetStatus(t, httpCli, fmt.Sprintf("%s%s/credentials/local?imei=%s", strings.TrimRight(baseURL, "/"), prefix, url.QueryEscape(imei)), "")
		if !statusIn(localStatus, http.StatusOK, http.StatusTooManyRequests) {
			t.Fatalf("local credentials alias %s status=%d", prefix, localStatus)
		}
	}

	canonicalPrefix := "/api/device-open"
	historyIMEIStatus := doOpenGetStatus(t, httpCli, fmt.Sprintf("%s%s/commands/history?imei=%s&limit=20", strings.TrimRight(baseURL, "/"), canonicalPrefix, url.QueryEscape(imei)), "")
	if !statusIn(historyIMEIStatus, http.StatusOK, http.StatusTooManyRequests) {
		t.Fatalf("history canonical (imei) status=%d", historyIMEIStatus)
	}

	historyDeviceUuidStatus := doOpenGetStatus(t, httpCli, fmt.Sprintf("%s%s/commands/history?deviceUuid=%s&limit=20", strings.TrimRight(baseURL, "/"), canonicalPrefix, url.QueryEscape(deviceID)), "")
	if !statusIn(historyDeviceUuidStatus, http.StatusOK, http.StatusTooManyRequests) {
		t.Fatalf("history canonical (deviceUuid) status=%d", historyDeviceUuidStatus)
	}

	historyDeviceUUIDSnakeStatus := doOpenGetStatus(t, httpCli, fmt.Sprintf("%s%s/commands/history?device_uuid=%s&limit=20", strings.TrimRight(baseURL, "/"), canonicalPrefix, url.QueryEscape(deviceID)), "")
	if !statusIn(historyDeviceUUIDSnakeStatus, http.StatusOK, http.StatusBadRequest, http.StatusTooManyRequests) {
		t.Fatalf("history canonical (device_uuid) status=%d", historyDeviceUUIDSnakeStatus)
	}

	vfdStatus := doOpenGetStatus(t, httpCli, fmt.Sprintf("%s%s/vfd?imei=%s", strings.TrimRight(baseURL, "/"), canonicalPrefix, url.QueryEscape(imei)), "")
	if !statusIn(vfdStatus, http.StatusOK, http.StatusTooManyRequests) {
		t.Fatalf("vfd canonical status=%d", vfdStatus)
	}

	installationStatus := doOpenGetStatus(t, httpCli, fmt.Sprintf("%s%s/installations/%s", strings.TrimRight(baseURL, "/"), canonicalPrefix, url.QueryEscape(deviceID)), "")
	if !statusIn(installationStatus, http.StatusOK, http.StatusNotFound, http.StatusTooManyRequests) {
		t.Fatalf("installation canonical status=%d", installationStatus)
	}
}

func statusIn(status int, allowed ...int) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}

func doOpenGetStatus(t testing.TB, cli *http.Client, uri, apiKey string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", uri, err)
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("request %s failed: %v", uri, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
