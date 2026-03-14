package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequireCapability_AllRequired(t *testing.T) {
	app := fiber.New()
	app.Get("/secure", func(c *fiber.Ctx) error {
		c.Locals("capabilities", []string{"devices:read", "admin:all"})
		return c.Next()
	}, RequireCapability([]string{"devices:read"}, true), func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/secure", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}
}

func TestRequireCapability_Missing(t *testing.T) {
	app := fiber.New()
	app.Get("/secure", func(c *fiber.Ctx) error {
		c.Locals("capabilities", []string{"devices:read"})
		return c.Next()
	}, RequireCapability([]string{"devices:write"}, true), func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/secure", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 got %d", resp.StatusCode)
	}
}
