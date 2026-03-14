package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestStrictSnakeWireMiddleware_QueryParams(t *testing.T) {
	prev := os.Getenv(strictSnakeWireEnv)
	t.Cleanup(func() { _ = os.Setenv(strictSnakeWireEnv, prev) })
	_ = os.Setenv(strictSnakeWireEnv, "true")

	app := fiber.New()
	app.Use(StrictSnakeWireMiddleware())
	app.Get("/api/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/test?projectId=abc", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/api/test?project_id=abc", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStrictSnakeWireMiddleware_JSONBody(t *testing.T) {
	prev := os.Getenv(strictSnakeWireEnv)
	t.Cleanup(func() { _ = os.Setenv(strictSnakeWireEnv, prev) })
	_ = os.Setenv(strictSnakeWireEnv, "true")

	app := fiber.New()
	app.Use(StrictSnakeWireMiddleware())
	app.Post("/api/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	bad := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"projectId":"abc"}`))
	bad.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(bad)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	good := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"project_id":"abc"}`))
	good.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(good)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStrictSnakeWireMiddleware_ReactFlowOpaqueNodesEdgesAllowed(t *testing.T) {
	prev := os.Getenv(strictSnakeWireEnv)
	t.Cleanup(func() { _ = os.Setenv(strictSnakeWireEnv, prev) })
	_ = os.Setenv(strictSnakeWireEnv, "true")

	app := fiber.New()
	app.Use(StrictSnakeWireMiddleware())
	app.Post("/api/config/automation", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	// Nested camelCase under nodes/edges is allowed for this route.
	body := `{"project_id":"abc","nodes":[{"id":"1","data":{"fooBar":123}}],"edges":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/config/automation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStrictSnakeWireMiddleware_NorthboundSkipped(t *testing.T) {
	prev := os.Getenv(strictSnakeWireEnv)
	t.Cleanup(func() { _ = os.Setenv(strictSnakeWireEnv, prev) })
	_ = os.Setenv(strictSnakeWireEnv, "true")

	app := fiber.New()
	app.Use(StrictSnakeWireMiddleware())
	app.Post("/api/northbound/chirpstack", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	// devEui is part of ChirpStack webhook contract (camelCase); strict mode must not reject it.
	body := `{"deviceInfo":{"devEui":"abc"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/northbound/chirpstack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
