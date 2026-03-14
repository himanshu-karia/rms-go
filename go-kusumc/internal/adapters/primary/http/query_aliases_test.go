package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestQueryAlias_PrecedenceAndFallback(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"value": queryAlias(c, "primary", "alias", "fallback"),
		})
	})

	req := httptest.NewRequest("GET", "/q?alias=b&fallback=c", nil)
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
	if payload["value"] != "b" {
		t.Fatalf("expected alias fallback value b, got %v", payload["value"])
	}

	req2 := httptest.NewRequest("GET", "/q?primary=a&alias=b", nil)
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
	if payload2["value"] != "a" {
		t.Fatalf("expected primary precedence value a, got %v", payload2["value"])
	}
}

func TestQueryAliasDefault_DefaultAndOverride(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"value": queryAliasDefault(c, "42", "limit", "pageSize"),
		})
	})

	req := httptest.NewRequest("GET", "/q", nil)
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
	if payload["value"] != "42" {
		t.Fatalf("expected default value 42, got %v", payload["value"])
	}

	req2 := httptest.NewRequest("GET", "/q?pageSize=25", nil)
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
	if payload2["value"] != "25" {
		t.Fatalf("expected alias override value 25, got %v", payload2["value"])
	}
}

func TestQueryAliasInt_DefaultAndInvalidFallback(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"value": queryAliasInt(c, 50, "limit", "pageSize"),
		})
	})

	req := httptest.NewRequest("GET", "/q?pageSize=20", nil)
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
	if payload["value"] != float64(20) {
		t.Fatalf("expected parsed int value 20, got %v", payload["value"])
	}

	req2 := httptest.NewRequest("GET", "/q?pageSize=invalid", nil)
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
	if payload2["value"] != float64(50) {
		t.Fatalf("expected default fallback value 50, got %v", payload2["value"])
	}
}

func TestQueryAliasBool_DefaultAliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"value": queryAliasBool(c, false, "includeSoftDeleted", "include_soft_deleted"),
		})
	})

	req := httptest.NewRequest("GET", "/q?include_soft_deleted=1", nil)
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
	if payload["value"] != true {
		t.Fatalf("expected alias truthy value true, got %v", payload["value"])
	}

	req2 := httptest.NewRequest("GET", "/q?includeSoftDeleted=false&include_soft_deleted=1", nil)
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
	if payload2["value"] != false {
		t.Fatalf("expected primary precedence value false, got %v", payload2["value"])
	}
}
