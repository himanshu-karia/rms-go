package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := strings.TrimSpace(c.Get("x-request-id"))
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Locals("request_id", reqID)
		c.Set("x-request-id", reqID)
		return c.Next()
	}
}
