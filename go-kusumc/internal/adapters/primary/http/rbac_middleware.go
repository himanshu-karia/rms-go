package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Role Hierarchy: viewer < manager < admin
var rolePower = map[string]int{
	"viewer":  1,
	"manager": 2,
	"admin":   3,
}

// RequireRole enforces RBAC based on token claims
func RequireRole(minRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Get User from Context (set by AuthMiddleware)
		// Fix: AuthMiddleware sets 'user' as jwt.MapClaims directly, NOT *jwt.Token
		var claims jwt.MapClaims = make(jwt.MapClaims)

		val := c.Locals("user")
		if cClaims, ok := val.(jwt.MapClaims); ok {
			claims = cClaims
		} else if mapClaims, ok := val.(map[string]interface{}); ok {
			for k, v := range mapClaims {
				claims[k] = v
			}
		} else if token, ok := val.(*jwt.Token); ok {
			claims = token.Claims.(jwt.MapClaims)
		} else {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized context"})
		}

		userRole := "viewer" // Default
		if r, ok := claims["role"].(string); ok {
			userRole = r
		}

		// 2. Normalize
		userRole = strings.ToLower(userRole)
		minRole = strings.ToLower(minRole)

		// 3. Check Power
		userPower := rolePower[userRole]
		requiredPower := rolePower[minRole]

		if userPower < requiredPower {
			return c.Status(403).JSON(fiber.Map{
				"error":    "insufficient permissions",
				"required": minRole,
				"current":  userRole,
			})
		}

		return c.Next()
	}
}

// RequireCapability enforces capability-based access (legacy parity)
func RequireCapability(required []string, matchAll bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		val := c.Locals("capabilities")
		capSet := map[string]bool{}

		switch typed := val.(type) {
		case []string:
			for _, cap := range typed {
				capSet[strings.ToLower(cap)] = true
			}
		case []interface{}:
			for _, item := range typed {
				if cap, ok := item.(string); ok {
					capSet[strings.ToLower(cap)] = true
				}
			}
		}

		// admin override
		if capSet["admin:all"] {
			return c.Next()
		}

		if len(required) == 0 {
			return c.Next()
		}

		if matchAll {
			for _, cap := range required {
				if !capSet[strings.ToLower(cap)] {
					return c.Status(403).JSON(fiber.Map{
						"error":    "missing_capability",
						"required": required,
					})
				}
			}
			return c.Next()
		}

		for _, cap := range required {
			if capSet[strings.ToLower(cap)] {
				return c.Next()
			}
		}
		return c.Status(403).JSON(fiber.Map{
			"error":    "missing_capability",
			"required": required,
		})
	}
}
