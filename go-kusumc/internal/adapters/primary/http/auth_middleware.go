package http

import (
	"ingestion-go/internal/core/services"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func isPublicAuthPath(path string) bool {
	publicPaths := map[string]struct{}{
		"/api/auth":                       {},
		"/api/auth/login":                 {},
		"/api/auth/google":                {},
		"/api/auth/register":              {},
		"/api/auth/refresh":               {},
		"/api/auth/logout":                {},
		"/api/auth/password-reset":        {},
		"/api/auth/session":               {},
		"/api/auth/me":                    {},
		"/api/v1/auth/login":              {},
		"/api/v1/auth/google":             {},
		"/api/v1/auth/register":           {},
		"/api/v1/auth/refresh":            {},
		"/api/v1/auth/logout":             {},
		"/api/v1/auth/password-reset":     {},
		"/api/v1/auth/session":            {},
		"/api/mobile/auth/request-otp":    {},
		"/api/mobile/auth/verify":         {},
		"/api/mobile/auth/refresh":        {},
		"/api/mobile/auth/dev-otp/latest": {},
	}

	_, ok := publicPaths[path]
	return ok
}

// AuthMiddleware protects routes requiring JWT (Northbound/UI)
func AuthMiddleware(authService *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Allow public auth routes even if mounted under protected group.
		path := c.Path()
		if isPublicAuthPath(path) {
			return c.Next()
		}

		// 0. Bypass if API Key Middleware already validated it
		if c.Locals("auth_method") == "api_key" {
			return c.Next()
		}

		// 1. Get Token from Header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return WriteAPIError(c, fiber.StatusUnauthorized, "auth_missing_authorization", "Missing Authorization Header", nil)
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return WriteAPIError(c, fiber.StatusUnauthorized, "auth_invalid_token_format", "Invalid Token Format", nil)
		}

		tokenString := parts[1]

		// 2. Verify Token
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			return WriteAPIError(c, fiber.StatusUnauthorized, "auth_invalid_or_expired_token", "Invalid or Expired Token", nil)
		}

		// 3. Inject User Context
		c.Locals("user", claims)          // claims is map[string]interface{}
		c.Locals("user_id", claims["id"]) // Normalize for Audit
		c.Locals("role", claims["role"])
		c.Locals("session_id", claims["session_id"])
		c.Locals("capabilities", claims["capabilities"])

		return c.Next()
	}
}
