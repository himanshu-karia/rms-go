package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestUserGroupsFilterQuery_SnakeCaseAliases(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"stateId":     userGroupsFilterQuery(c, "state_id", "stateId"),
			"authorityId": userGroupsFilterQuery(c, "authority_id", "authorityId"),
			"projectId":   userGroupsFilterQuery(c, "project_id", "projectId"),
		})
	})

	req := httptest.NewRequest("GET", "/q?state_id=s1&authority_id=a1&project_id=p1", nil)
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
	if payload["stateId"] != "s1" || payload["authorityId"] != "a1" || payload["projectId"] != "p1" {
		t.Fatalf("unexpected alias mapping: %+v", payload)
	}
}

func TestUserGroupsFilterQuery_Precedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"stateId": userGroupsFilterQuery(c, "state_id", "stateId"),
		})
	})

	req := httptest.NewRequest("GET", "/q?stateId=camel&state_id=snake", nil)
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
	if payload["stateId"] != "snake" {
		t.Fatalf("expected snake_case precedence, got %v", payload["stateId"])
	}
}

func TestUserGroupsLimitQuery_AliasAndPrecedence(t *testing.T) {
	app := fiber.New()
	app.Get("/q", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"limit": userGroupsLimitQuery(c)})
	})

	req := httptest.NewRequest("GET", "/q?pageSize=15", nil)
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
	if payload["limit"] != "15" {
		t.Fatalf("expected pageSize alias value 15, got %v", payload["limit"])
	}

	req2 := httptest.NewRequest("GET", "/q?limit=20&pageSize=15", nil)
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
	if payload2["limit"] != "20" {
		t.Fatalf("expected limit precedence value 20, got %v", payload2["limit"])
	}
}
