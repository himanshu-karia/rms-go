package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type APIErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
}

func ensureRequestID(c *fiber.Ctx) string {
	if raw := strings.TrimSpace(c.Get("x-request-id")); raw != "" {
		c.Set("x-request-id", raw)
		return raw
	}
	if v, ok := c.Locals("request_id").(string); ok {
		if s := strings.TrimSpace(v); s != "" {
			c.Set("x-request-id", s)
			return s
		}
	}
	id := uuid.NewString()
	c.Locals("request_id", id)
	c.Set("x-request-id", id)
	return id
}

func WriteAPIError(c *fiber.Ctx, status int, code, message string, details any) error {
	if strings.TrimSpace(code) == "" {
		code = "api_error"
	}
	if strings.TrimSpace(message) == "" {
		message = "request failed"
	}

	resp := APIErrorResponse{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: ensureRequestID(c),
		Error:     message,
	}
	return c.Status(status).JSON(resp)
}
