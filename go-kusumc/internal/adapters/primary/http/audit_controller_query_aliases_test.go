package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAuditQueryAliases_SnakeCase(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"afterId":     auditQuery(c, "afterId"),
			"actorId":     auditQuery(c, "actorId"),
			"stateId":     auditQuery(c, "stateId"),
			"authorityId": auditQuery(c, "authorityId"),
			"projectId":   auditQuery(c, "projectId"),
		})
	})

	req := httptest.NewRequest("GET", "/q?after_id=a1&actor_id=u1&state_id=s1&authority_id=auth1&project_id=p1", nil)
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
	if payload["afterId"] != "a1" || payload["actorId"] != "u1" || payload["stateId"] != "s1" || payload["authorityId"] != "auth1" || payload["projectId"] != "p1" {
		t.Fatalf("unexpected alias mapping: %+v", payload)
	}
}

func TestAuditQueryAliases_Precedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"afterId":   auditQuery(c, "afterId"),
			"projectId": auditQuery(c, "projectId"),
		})
	})

	req := httptest.NewRequest("GET", "/q?afterId=camel-after&after_id=snake-after&projectId=camel-project&project_id=snake-project", nil)
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
	if payload["afterId"] != "snake-after" {
		t.Fatalf("expected snake_case precedence for afterId, got %v", payload["afterId"])
	}
	if payload["projectId"] != "snake-project" {
		t.Fatalf("expected snake_case precedence for projectId, got %v", payload["projectId"])
	}
}

func TestAuditCursorQuery_AliasesAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"afterId": auditCursorQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?cursor=cursor-id", nil)
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
	if payload["afterId"] != "cursor-id" {
		t.Fatalf("expected cursor alias value cursor-id, got %v", payload["afterId"])
	}

	req2 := httptest.NewRequest("GET", "/q?afterId=camel-after&after_id=snake-after&cursor=cursor-id", nil)
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
	if payload2["afterId"] != "cursor-id" {
		t.Fatalf("expected cursor precedence, got %v", payload2["afterId"])
	}
}

func TestAuditQuery_LimitAliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"limit": auditQuery(c, "limit")})
	})

	req := httptest.NewRequest("GET", "/q?pageSize=75", nil)
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
	if payload["limit"] != "75" {
		t.Fatalf("expected pageSize alias value 75, got %v", payload["limit"])
	}

	req2 := httptest.NewRequest("GET", "/q?limit=100&pageSize=75", nil)
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
	if payload2["limit"] != "100" {
		t.Fatalf("expected limit precedence value 100, got %v", payload2["limit"])
	}
}
