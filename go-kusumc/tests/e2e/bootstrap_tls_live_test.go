//go:build integration

package e2e

import (
	"strings"
	"testing"
)

func TestLiveBootstrapTLS(t *testing.T) {
	baseURL := getenv("BASE_URL", "https://rms-iot.local:7443")
	base := getenv("BOOTSTRAP_URL", strings.TrimRight(baseURL, "/")+"/api/bootstrap")
	imei := getenv("BOOTSTRAP_IMEI", randomIMEI())
	projectID := getenv("PROJECT_ID", "pm-kusum-solar-pump-msedcl")

	httpCli := httpClient(t)
	token := loginOrSkip(t, httpCli, baseURL, "Him", "0554")
	_ = createDevice(t, httpCli, baseURL, token, projectID, imei)

	boot, err := fetchBootstrap(t, base, imei)
	if err != nil {
		t.Fatalf("bootstrap request failed: %v", err)
	}

	endpoints := boot.PrimaryBroker.Endpoints
	if len(endpoints) == 0 {
		t.Fatalf("no endpoints returned in primary_broker")
	}

	foundTLS := false
	for _, ep := range endpoints {
		if strings.HasPrefix(ep, "mqtts://") {
			foundTLS = true
			break
		}
	}
	if !foundTLS {
		t.Fatalf("expected at least one mqtts:// endpoint, got %v", endpoints)
	}
}
