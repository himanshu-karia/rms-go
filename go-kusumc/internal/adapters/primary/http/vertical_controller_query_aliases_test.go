package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestVerticalQuery_SnakeCaseAliases(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"project":       verticalQuery(c, "project_id", "projectId"),
			"accountStatus": verticalQuery(c, "account_status", "accountStatus"),
			"installation":  verticalQuery(c, "installationUuid", "installation_id"),
			"device":        verticalQuery(c, "device_id", "deviceUuid", "deviceId"),
		})
	})

	req := httptest.NewRequest("GET", "/q?project_id=p1&account_status=active&installation_id=i1&device_id=d1", nil)
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
	if payload["project"] != "p1" || payload["accountStatus"] != "active" || payload["installation"] != "i1" || payload["device"] != "d1" {
		t.Fatalf("unexpected alias mapping: %+v", payload)
	}
}

func TestVerticalQuery_PrecedenceAndBoolAlias(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"project":        verticalQuery(c, "project_id", "projectId"),
			"includeDeleted": verticalQueryBool(c, false, "include_soft_deleted", "includeSoftDeleted"),
		})
	})

	req := httptest.NewRequest("GET", "/q?projectId=camel&project_id=snake&includeSoftDeleted=true&include_soft_deleted=0", nil)
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
	if payload["project"] != "snake" {
		t.Fatalf("expected snake_case precedence for project, got %v", payload["project"])
	}
	if payload["includeDeleted"] != false {
		t.Fatalf("expected include_soft_deleted precedence false, got %v", payload["includeDeleted"])
	}
}

func TestVerticalQuery_SearchAliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"search": verticalQuery(c, "search", "q"),
		})
	})

	req := httptest.NewRequest("GET", "/q?q=solar", nil)
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
	if payload["search"] != "solar" {
		t.Fatalf("expected q alias value solar, got %v", payload["search"])
	}

	req2 := httptest.NewRequest("GET", "/q?search=device&q=solar", nil)
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
	if payload2["search"] != "device" {
		t.Fatalf("expected search precedence value device, got %v", payload2["search"])
	}
}

func TestVerticalQueryInt_LimitAliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"limit": verticalQueryInt(c, 25, "limit", "pageSize")})
	})

	req := httptest.NewRequest("GET", "/q?pageSize=40", nil)
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
	if payload["limit"] != float64(40) {
		t.Fatalf("expected pageSize alias value 40, got %v", payload["limit"])
	}

	req2 := httptest.NewRequest("GET", "/q?limit=60&pageSize=40", nil)
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
	if payload2["limit"] != float64(60) {
		t.Fatalf("expected limit precedence value 60, got %v", payload2["limit"])
	}
}

func TestVerticalQuery_EdgeRouteAliases(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":       verticalQuery(c, "status", "status_filter"),
			"manufacturer": verticalQuery(c, "manufacturer_id", "manufacturerId"),
			"protocol":     verticalQuery(c, "protocol_version_id", "protocolVersionId"),
		})
	})

	req := httptest.NewRequest("GET", "/q?status_filter=active&manufacturer_id=m1&protocol_version_id=pv1", nil)
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
	if payload["status"] != "active" || payload["manufacturer"] != "m1" || payload["protocol"] != "pv1" {
		t.Fatalf("unexpected edge alias mapping: %+v", payload)
	}

	req2 := httptest.NewRequest("GET", "/q?status=pending&status_filter=active&manufacturerId=m2&manufacturer_id=m1&protocolVersionId=pv2&protocol_version_id=pv1", nil)
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
	if payload2["status"] != "pending" || payload2["manufacturer"] != "m1" || payload2["protocol"] != "pv1" {
		t.Fatalf("expected snake_case precedence for edge aliases, got %+v", payload2)
	}
}
