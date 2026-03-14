package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestTelemetryExportQueryAliases_ProjectAndDevice(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"projectId": telemetryExportProjectQuery(c),
			"device":    telemetryExportDeviceQuery(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?project_id=proj-snake&device_id=dev-snake", nil)
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
	if payload["projectId"] != "proj-snake" {
		t.Fatalf("expected project alias proj-snake, got %v", payload["projectId"])
	}
	if payload["device"] != "dev-snake" {
		t.Fatalf("expected device alias dev-snake, got %v", payload["device"])
	}
}

func TestTelemetryExportQueryAliases_Precedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"projectId": telemetryExportProjectQuery(c),
			"device":    telemetryExportDeviceQuery(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?projectId=proj-camel&project_id=proj-snake&imei=imei-primary&deviceId=device-camel&device_id=device-snake", nil)
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
	if payload["projectId"] != "proj-snake" {
		t.Fatalf("expected project_id snake_case precedence, got %v", payload["projectId"])
	}
	if payload["device"] != "imei-primary" {
		t.Fatalf("expected imei precedence, got %v", payload["device"])
	}
}

func TestTelemetryExportQueryAliases_FilterAliasesAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"start":           telemetryExportStartQuery(c),
			"end":             telemetryExportEndQuery(c),
			"packetType":      telemetryExportPacketTypeQuery(c),
			"quality":         telemetryExportQualityQuery(c),
			"exclude_quality": telemetryExportExcludeQualityQuery(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?from=2026-01-01T00:00&to=2026-01-02T00:00&packet_type=live&data_quality=good&excludeQuality=bad", nil)
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
	if payload["start"] != "2026-01-01T00:00" || payload["end"] != "2026-01-02T00:00" {
		t.Fatalf("expected from/to aliases, got start=%v end=%v", payload["start"], payload["end"])
	}
	if payload["packetType"] != "live" {
		t.Fatalf("expected packet_type alias value live, got %v", payload["packetType"])
	}
	if payload["quality"] != "good" {
		t.Fatalf("expected data_quality alias value good, got %v", payload["quality"])
	}
	if payload["exclude_quality"] != "bad" {
		t.Fatalf("expected excludeQuality alias value bad, got %v", payload["exclude_quality"])
	}

	req2 := httptest.NewRequest("GET", "/q?start=2026-02-01T00:00&from=2026-01-01T00:00&end=2026-02-02T00:00&to=2026-01-02T00:00&packetType=snapshot&packet_type=live&quality=excellent&data_quality=good&exclude_quality=worse&excludeQuality=bad", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var payload2 map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&payload2); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload2["start"] != "2026-02-01T00:00" || payload2["end"] != "2026-02-02T00:00" {
		t.Fatalf("expected start/end precedence, got start=%v end=%v", payload2["start"], payload2["end"])
	}
	if payload2["packetType"] != "live" {
		t.Fatalf("expected packet_type snake_case precedence value live, got %v", payload2["packetType"])
	}
	if payload2["quality"] != "excellent" {
		t.Fatalf("expected quality precedence value excellent, got %v", payload2["quality"])
	}
	if payload2["exclude_quality"] != "worse" {
		t.Fatalf("expected exclude_quality precedence value worse, got %v", payload2["exclude_quality"])
	}
}
