package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAnalyticsQueryAliases_Device(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"device": analyticsDeviceQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?device_uuid=dev-uuid-1", nil)
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
	if payload["device"] != "dev-uuid-1" {
		t.Fatalf("expected device alias value dev-uuid-1, got %v", payload["device"])
	}
}

func TestAnalyticsQueryAliases_PacketAndTime(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"packet_type": analyticsPacketTypeQuery(c),
			"start":       analyticsStartQuery(c),
			"end":         analyticsEndQuery(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?packetType=live&start=2026-01-01T00:00&end=2026-01-01T01:00", nil)
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
	if payload["packet_type"] != "live" {
		t.Fatalf("expected packet type alias live, got %v", payload["packet_type"])
	}
	if payload["start"] != "2026-01-01T00:00" || payload["end"] != "2026-01-01T01:00" {
		t.Fatalf("expected start/end aliases to resolve, got start=%v end=%v", payload["start"], payload["end"])
	}
}

func TestAnalyticsQueryAliases_QualityAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"device":          analyticsDeviceQuery(c),
			"packet_type":     analyticsPacketTypeQuery(c),
			"exclude_quality": analyticsExcludeQualityQuery(c),
			"quality":         analyticsQualityQuery(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?device=dev-camel&device_id=dev-snake&packet_type=pt-snake&packetType=pt-camel&exclude_quality=bad&excludeQuality=worse&quality=good&data_quality=fallback", nil)
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
	if payload["device"] != "dev-camel" {
		t.Fatalf("expected precedence to prefer device, got %v", payload["device"])
	}
	if payload["packet_type"] != "pt-snake" {
		t.Fatalf("expected precedence to prefer packet_type, got %v", payload["packet_type"])
	}
	if payload["exclude_quality"] != "bad" {
		t.Fatalf("expected precedence to prefer exclude_quality, got %v", payload["exclude_quality"])
	}
	if payload["quality"] != "good" {
		t.Fatalf("expected precedence to prefer quality, got %v", payload["quality"])
	}
}

func TestAnalyticsQueryAliases_PaginationAliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"page":  analyticsPageQuery(c),
			"limit": analyticsLimitQuery(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?pageNumber=4&pageSize=20", nil)
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
	if payload["page"] != "4" {
		t.Fatalf("expected page alias value 4, got %v", payload["page"])
	}
	if payload["limit"] != "20" {
		t.Fatalf("expected limit alias value 20, got %v", payload["limit"])
	}

	req = httptest.NewRequest("GET", "/q?page=2&pageNumber=4&limit=50&pageSize=20", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["page"] != "2" {
		t.Fatalf("expected page precedence value 2, got %v", payload["page"])
	}
	if payload["limit"] != "50" {
		t.Fatalf("expected limit precedence value 50, got %v", payload["limit"])
	}
}

func TestAnalyticsQueryAliases_ProjectAliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"project": analyticsProjectQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?project_id=proj-snake", nil)
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
	if payload["project"] != "proj-snake" {
		t.Fatalf("expected project snake alias value proj-snake, got %v", payload["project"])
	}

	req2 := httptest.NewRequest("GET", "/q?projectId=proj-camel&project_id=proj-snake", nil)
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
	if payload2["project"] != "proj-snake" {
		t.Fatalf("expected project snake precedence value proj-snake, got %v", payload2["project"])
	}
}
