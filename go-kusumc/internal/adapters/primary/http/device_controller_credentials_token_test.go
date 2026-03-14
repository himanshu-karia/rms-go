package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestIssueCredentialDownloadToken_EmitsExpiresAtSnakeCase(t *testing.T) {
	app := fiber.New()
	controller := &DeviceController{}
	app.Post("/api/devices/:id/credentials/download-token", controller.IssueCredentialDownloadToken)

	req := httptest.NewRequest("POST", "/api/devices/dev-1/credentials/download-token", strings.NewReader(`{"expires_in_seconds": 60}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if token, _ := payload["token"].(string); strings.TrimSpace(token) == "" {
		t.Fatalf("expected token in response, got %v", payload["token"])
	}

	expiresAt, _ := payload["expires_at"].(string)
	if strings.TrimSpace(expiresAt) == "" {
		t.Fatalf("expected expires_at in response, got %v", payload["expires_at"])
	}
	if _, err := time.Parse(time.RFC3339, expiresAt); err != nil {
		t.Fatalf("expected expires_at RFC3339, got %q: %v", expiresAt, err)
	}
}
