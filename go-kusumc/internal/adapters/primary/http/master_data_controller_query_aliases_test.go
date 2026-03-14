package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestMasterDataProjectQuery_SnakeCaseAlias(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"project_id": masterDataProjectQuery(c)})
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
	if payload["project_id"] != "proj-snake" {
		t.Fatalf("expected project alias proj-snake, got %v", payload["project_id"])
	}
}

func TestMasterDataProjectQuery_Precedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"project_id": masterDataProjectQuery(c)})
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
	if payload["project_id"] != "proj-snake" {
		t.Fatalf("expected snake_case precedence proj-snake, got %v", payload["project_id"])
	}
}
