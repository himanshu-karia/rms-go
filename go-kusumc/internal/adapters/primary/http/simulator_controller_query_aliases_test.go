package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSimulatorLimitQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"limit": simulatorLimitQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?pageSize=30", nil)
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
	if payload["limit"] != "30" {
		t.Fatalf("expected pageSize alias value 30, got %v", payload["limit"])
	}

	req2 := httptest.NewRequest("GET", "/q?limit=50&pageSize=30", nil)
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
	if payload2["limit"] != "50" {
		t.Fatalf("expected limit precedence value 50, got %v", payload2["limit"])
	}
}

func TestSimulatorCursorQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"cursor": simulatorCursorQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?after_id=cur-snake", nil)
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
	if payload["cursor"] != "cur-snake" {
		t.Fatalf("expected after_id alias value cur-snake, got %v", payload["cursor"])
	}

	req1b := httptest.NewRequest("GET", "/q?afterId=cur-camel&after_id=cur-snake", nil)
	resp1b, err := app.Test(req1b)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp1b.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp1b.StatusCode)
	}
	var payload1b map[string]any
	if err := json.NewDecoder(resp1b.Body).Decode(&payload1b); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload1b["cursor"] != "cur-snake" {
		t.Fatalf("expected after_id precedence value cur-snake, got %v", payload1b["cursor"])
	}

	req2 := httptest.NewRequest("GET", "/q?cursor=cur-primary&afterId=cur-camel&after_id=cur-snake", nil)
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
	if payload2["cursor"] != "cur-primary" {
		t.Fatalf("expected cursor precedence value cur-primary, got %v", payload2["cursor"])
	}
}

func TestSimulatorStatusQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": simulatorStatusQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?status_filter=active", nil)
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
	if payload["status"] != "active" {
		t.Fatalf("expected status_filter alias value active, got %v", payload["status"])
	}

	req2 := httptest.NewRequest("GET", "/q?status=revoked&status_filter=active", nil)
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
	if payload2["status"] != "revoked" {
		t.Fatalf("expected status precedence value revoked, got %v", payload2["status"])
	}
}
