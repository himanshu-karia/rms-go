package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestDeviceListQueryAliases_ProjectID(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"project_id": deviceListProjectID(c)})
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
		t.Fatalf("expected project alias value proj-snake, got %v", payload["project_id"])
	}
}

func TestDeviceListQueryAliases_IncludeInactive(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"include_inactive": deviceListIncludeInactiveParam(c)})
	})

	req := httptest.NewRequest("GET", "/q?include_inactive=true", nil)
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
	if payload["include_inactive"] != "true" {
		t.Fatalf("expected include_inactive alias value true, got %v", payload["include_inactive"])
	}
}

func TestDeviceListQueryAliases_Precedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"project_id":       deviceListProjectID(c),
			"include_inactive": deviceListIncludeInactiveParam(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?projectId=proj-camel&project_id=proj-snake&includeInactive=false&include_inactive=true", nil)
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
		t.Fatalf("expected snake_case precedence for project_id, got %v", payload["project_id"])
	}
	if payload["include_inactive"] != "true" {
		t.Fatalf("expected snake_case precedence for include_inactive, got %v", payload["include_inactive"])
	}
}

func TestDeviceListQueryAliases_SearchAlias(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"search": deviceListSearchQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?q=device-001", nil)
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
	if payload["search"] != "device-001" {
		t.Fatalf("expected q alias value device-001, got %v", payload["search"])
	}
}

func TestDeviceListQueryAliases_SearchPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"search": deviceListSearchQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?search=camel&q=snake", nil)
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
	if payload["search"] != "camel" {
		t.Fatalf("expected search precedence value camel, got %v", payload["search"])
	}
}

func TestDeviceListQueryAliases_PageAndLimitAliases(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"page":  deviceListPageParam(c),
			"limit": deviceListLimitParam(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?pageNumber=3&pageSize=25", nil)
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
	if payload["page"] != "3" {
		t.Fatalf("expected page alias value 3, got %v", payload["page"])
	}
	if payload["limit"] != "25" {
		t.Fatalf("expected limit alias value 25, got %v", payload["limit"])
	}
}

func TestDeviceListQueryAliases_PageAndLimitPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"page":  deviceListPageParam(c),
			"limit": deviceListLimitParam(c),
		})
	})

	req := httptest.NewRequest("GET", "/q?page=2&pageNumber=3&limit=50&pageSize=25", nil)
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
	if payload["page"] != "2" {
		t.Fatalf("expected page precedence value 2, got %v", payload["page"])
	}
	if payload["limit"] != "50" {
		t.Fatalf("expected limit precedence value 50, got %v", payload["limit"])
	}
}

func TestDeviceConfigImportProjectQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"project": deviceConfigImportProjectQuery(c)})
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
		t.Fatalf("expected project_id alias value proj-snake, got %v", payload["project"])
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
		t.Fatalf("expected project_id precedence value proj-snake, got %v", payload2["project"])
	}
}
