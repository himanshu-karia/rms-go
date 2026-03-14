package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAdminFilterQuery_SnakeCaseAlias(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"state_id":      adminFilterQuery(c, "state_id", "stateId"),
			"authority_id":  adminFilterQuery(c, "authority_id", "authorityId"),
			"project_id":    adminFilterQuery(c, "project_id", "projectId"),
			"server_vendor": adminFilterQuery(c, "server_vendor_id", "serverVendorId"),
			"role_key":      adminFilterQuery(c, "role_key", "roleKey"),
		})
	})

	req := httptest.NewRequest("GET", "/q?state_id=s1&authority_id=a1&project_id=p1&server_vendor_id=v1&role_key=r1", nil)
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
	if payload["state_id"] != "s1" || payload["authority_id"] != "a1" || payload["project_id"] != "p1" || payload["server_vendor"] != "v1" || payload["role_key"] != "r1" {
		t.Fatalf("unexpected alias mapping: %+v", payload)
	}
}

func TestAdminFilterQuery_Precedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"project_id": adminFilterQuery(c, "project_id", "projectId"),
			"role_key":   adminFilterQuery(c, "role_key", "roleKey"),
		})
	})

	req := httptest.NewRequest("GET", "/q?projectId=proj-camel&project_id=proj-snake&roleKey=operator&role_key=viewer", nil)
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
	if payload["role_key"] != "viewer" {
		t.Fatalf("expected snake_case precedence for role_key, got %v", payload["role_key"])
	}
}

func TestAdminCursorQuery_AliasesAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"cursor": adminCursorQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?after_id=cursor-snake", nil)
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
	if payload["cursor"] != "cursor-snake" {
		t.Fatalf("expected after_id alias value cursor-snake, got %v", payload["cursor"])
	}

	req1b := httptest.NewRequest("GET", "/q?afterId=after-camel&after_id=after-snake", nil)
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
	if payload1b["cursor"] != "after-snake" {
		t.Fatalf("expected after_id precedence, got %v", payload1b["cursor"])
	}

	req2 := httptest.NewRequest("GET", "/q?cursor=cursor-primary&afterId=after-camel&after_id=after-snake", nil)
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
	if payload2["cursor"] != "cursor-primary" {
		t.Fatalf("expected cursor precedence, got %v", payload2["cursor"])
	}
}

func TestAdminLimitQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"limit": adminLimitQuery(c)})
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

func TestAdminStatusQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": adminStatusQuery(c)})
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

	req2 := httptest.NewRequest("GET", "/q?status=disabled&status_filter=active", nil)
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
	if payload2["status"] != "disabled" {
		t.Fatalf("expected status precedence value disabled, got %v", payload2["status"])
	}
}
