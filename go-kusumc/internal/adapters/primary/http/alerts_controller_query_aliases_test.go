package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAlertsProjectIDQuery_SnakeCaseAlias(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"projectId": alertsProjectIDQuery(c)})
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
	if payload["projectId"] != "proj-snake" {
		t.Fatalf("expected project alias value proj-snake, got %v", payload["projectId"])
	}
}

func TestAlertsProjectIDQuery_Precedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"projectId": alertsProjectIDQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?projectId=proj-camel&project_id=proj-snake", nil)
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
		t.Fatalf("expected snake_case precedence proj-snake, got %v", payload["projectId"])
	}
}

func TestAlertsStatusQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": alertsStatusQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?status_filter=open", nil)
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
	if payload["status"] != "open" {
		t.Fatalf("expected status_filter alias value open, got %v", payload["status"])
	}

	req2 := httptest.NewRequest("GET", "/q?status=closed&status_filter=open", nil)
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
	if payload2["status"] != "closed" {
		t.Fatalf("expected status precedence closed, got %v", payload2["status"])
	}
}
